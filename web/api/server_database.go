package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/models"
	"github.com/pufferpanel/pufferpanel/v3/response"
	"github.com/pufferpanel/pufferpanel/v3/services"
	"github.com/pufferpanel/pufferpanel/v3/utils"
	"gopkg.in/go-playground/validator.v9"
	"gorm.io/gorm"
)

var databaseEnvironmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var databaseIdentifier = regexp.MustCompile(`^[A-Za-z0-9_]{1,32}$`)

type serverDatabaseRequest struct {
	DatabaseServerID string            `json:"databaseServerId"`
	Engine           string            `json:"engine"`
	AccessMode       string            `json:"accessMode"`
	Host             string            `json:"host"`
	Port             uint16            `json:"port"`
	DatabaseName     string            `json:"databaseName"`
	Username         string            `json:"username"`
	Password         string            `json:"password"`
	VariableMapping  map[string]string `json:"variableMapping"`
}

var databaseVariableRoles = []string{"host", "port", "database", "username", "password"}

func getServerDatabase(c *gin.Context) {
	parent := getServerFromGin(c)
	result := struct {
		Database *models.ServerDatabase `json:"database"`
		Node     *models.NodeView       `json:"node"`
	}{Node: models.FromNode(&parent.Node)}
	var linked models.ServerDatabase
	err := middleware.GetDatabase(c).Where("server_identifier = ?", parent.Identifier).First(&linked).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, result)
		return
	}
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	result.Database = &linked
	c.JSON(http.StatusOK, result)
}

func saveServerDatabase(c *gin.Context) {
	parent := getServerFromGin(c)
	var request serverDatabaseRequest
	if err := c.ShouldBindJSON(&request); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	if request.Engine != "mariadb" && request.Engine != "postgres" {
		response.HandleError(c, errors.New("unsupported database engine"), http.StatusBadRequest)
		return
	}
	if request.AccessMode != "private" && request.AccessMode != "public" && request.AccessMode != "external" {
		response.HandleError(c, errors.New("unsupported database access mode"), http.StatusBadRequest)
		return
	}
	if err := validateDatabaseVariableMapping(request.VariableMapping); err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}

	databaseName := strings.TrimSpace(request.DatabaseName)
	username := strings.TrimSpace(request.Username)
	if !databaseIdentifier.MatchString(databaseName) || !databaseIdentifier.MatchString(username) {
		response.HandleError(c, errors.New("database name and username must contain only letters, numbers, or underscores"), http.StatusBadRequest)
		return
	}

	linked := &models.ServerDatabase{
		ServerIdentifier: parent.Identifier,
		Engine:           request.Engine,
		AccessMode:       request.AccessMode,
		DatabaseName:     databaseName,
		Username:         username,
		VariableMapping:  request.VariableMapping,
	}
	db := middleware.GetDatabase(c)
	ns := &services.Node{DB: db}
	ctx, cancel := context.WithTimeout(c.Request.Context(), services.NodeCallTimeout)
	defer cancel()
	connection := map[string]string{
		"database": databaseName,
		"username": username,
		"password": request.Password,
	}
	if request.AccessMode == "external" {
		linked.Host = strings.TrimSpace(request.Host)
		linked.Port = request.Port
		if validateDatabaseHost(linked.Host) != nil || linked.Port == 0 || request.DatabaseServerID != "" || request.Password == "" {
			response.HandleError(c, errors.New("external database host, port, and credentials are required"), http.StatusBadRequest)
			return
		}
	} else {
		if request.DatabaseServerID == "" || request.Password == "" {
			response.HandleError(c, errors.New("a database server and password are required"), http.StatusBadRequest)
			return
		}
		databaseServer, err := (&services.Server{DB: middleware.GetDatabase(c)}).Get(request.DatabaseServerID)
		if response.HandleError(c, err, http.StatusBadRequest) {
			return
		}
		if databaseServer.Identifier == parent.Identifier || databaseServer.Type != dbHostingType || !databaseServersShareNode(parent, databaseServer) || databaseServer.Port == 0 {
			response.HandleError(c, errors.New("database server must be a database service on the same node with an allocated port"), http.StatusBadRequest)
			return
		}
		databaseDefinition, err := loadDatabaseParentDefinition(ctx, ns, databaseServer)
		if response.HandleError(c, err, http.StatusBadGateway) {
			return
		}
		if engine := fmt.Sprint(databaseDefinition.Variables["engine"].Value); engine != request.Engine {
			response.HandleError(c, errors.New("selected database engine does not match the database server"), http.StatusBadRequest)
			return
		}
		if name := fmt.Sprint(databaseDefinition.Variables["db_name"].Value); name != databaseName {
			response.HandleError(c, errors.New("database name does not match the database server"), http.StatusBadRequest)
			return
		}
		if user := fmt.Sprint(databaseDefinition.Variables["db_user"].Value); user != username {
			response.HandleError(c, errors.New("database username does not match the database server"), http.StatusBadRequest)
			return
		}
		if password := fmt.Sprint(databaseDefinition.Variables["db_password"].Value); password != request.Password {
			response.HandleError(c, errors.New("database password does not match the database server"), http.StatusBadRequest)
			return
		}
		linked.DatabaseServerIdentifier = &databaseServer.Identifier
		linked.Port = databaseServer.Port
		linked.Host = parent.Node.PrivateHost
		if validateDatabaseHost(linked.Host) != nil || linked.Host == "127.0.0.1" || linked.Host == "::1" || linked.Host == "0.0.0.0" || linked.Host == "::" {
			response.HandleError(c, errors.New("configure a node private address reachable by its server containers"), http.StatusBadRequest)
			return
		}
	}
	if linked.Port == 0 || len(linked.Host) > 253 {
		response.HandleError(c, errors.New("database host or port is invalid"), http.StatusBadRequest)
		return
	}
	connection["host"] = linked.Host
	connection["port"] = fmt.Sprint(linked.Port)

	var previous models.ServerDatabase
	previousErr := db.Where("server_identifier = ?", parent.Identifier).First(&previous).Error
	if previousErr != nil && !errors.Is(previousErr, gorm.ErrRecordNotFound) {
		response.HandleError(c, previousErr, http.StatusInternalServerError)
		return
	}

	definition, err := loadDatabaseParentDefinition(ctx, ns, parent)
	if response.HandleError(c, err, http.StatusBadGateway) {
		return
	}
	if definition.Execution.EnvironmentVariables == nil {
		definition.Execution.EnvironmentVariables = make(map[string]string)
	}
	previousDefinition := cloneDatabaseDefinition(definition)
	previousNames := make(map[string]bool)
	for _, name := range previous.VariableMapping {
		previousNames[name] = true
		delete(definition.Execution.EnvironmentVariables, name)
	}
	for _, role := range databaseVariableRoles {
		name := request.VariableMapping[role]
		if _, exists := definition.Execution.EnvironmentVariables[name]; exists && !previousNames[name] {
			response.HandleError(c, fmt.Errorf("environment variable %s is already in use", name), http.StatusConflict)
			return
		}
		definition.Execution.EnvironmentVariables[name] = connection[role]
	}

	if err = saveDatabaseParentDefinition(ctx, ns, parent, definition); response.HandleError(c, err, http.StatusBadGateway) {
		return
	}
	linkedErr := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("server_identifier = ?", parent.Identifier).Delete(&models.ServerDatabase{}).Error; err != nil {
			return err
		}
		return tx.Create(linked).Error
	})
	if response.HandleError(c, linkedErr, http.StatusInternalServerError) {
		if rollbackErr := restoreDatabaseDefinition(ns, parent, previousDefinition); rollbackErr != nil {
			c.Error(rollbackErr)
		}
		return
	}
	c.JSON(http.StatusOK, linked)
}

func deleteServerDatabase(c *gin.Context) {
	parent := getServerFromGin(c)
	db := middleware.GetDatabase(c)
	var linked models.ServerDatabase
	if err := db.Where("server_identifier = ?", parent.Identifier).First(&linked).Error; response.HandleError(c, err, http.StatusNotFound) {
		return
	}

	ns := &services.Node{DB: db}
	ctx, cancel := context.WithTimeout(c.Request.Context(), services.NodeCallTimeout)
	defer cancel()
	definition, err := loadDatabaseParentDefinition(ctx, ns, parent)
	if response.HandleError(c, err, http.StatusBadGateway) {
		return
	}
	previousDefinition := cloneDatabaseDefinition(definition)
	for _, name := range linked.VariableMapping {
		delete(definition.Execution.EnvironmentVariables, name)
	}
	if err = saveDatabaseParentDefinition(ctx, ns, parent, definition); response.HandleError(c, err, http.StatusBadGateway) {
		return
	}
	if err = db.Delete(&linked).Error; response.HandleError(c, err, http.StatusInternalServerError) {
		if rollbackErr := restoreDatabaseDefinition(ns, parent, previousDefinition); rollbackErr != nil {
			c.Error(rollbackErr)
		}
		return
	}
	c.Status(http.StatusNoContent)
}

func validateDatabaseVariableMapping(mapping map[string]string) error {
	if len(mapping) != len(databaseVariableRoles) {
		return errors.New("map all five database values to environment variables")
	}
	seen := make(map[string]bool, len(mapping))
	for _, role := range databaseVariableRoles {
		name := mapping[role]
		if !databaseEnvironmentName.MatchString(name) {
			return fmt.Errorf("environment variable name for %s is invalid", role)
		}
		if seen[name] {
			return errors.New("database environment variable names must be unique")
		}
		seen[name] = true
	}
	return nil
}

func validateDatabaseHost(host string) error {
	return validator.New().Var(host, "required,ip|fqdn|hostname")
}

func databaseServersShareNode(parent, databaseServer *models.Server) bool {
	return parent.Node.ID == databaseServer.Node.ID
}

func loadDatabaseParentDefinition(ctx context.Context, ns *services.Node, parent *models.Server) (*pufferpanel.Server, error) {
	callResponse, err := ns.CallNodeWithContext(ctx, &parent.Node, http.MethodGet, "/daemon/server/"+parent.Identifier+"/definition", nil, nil)
	defer utils.CloseResponse(callResponse)
	if err != nil {
		return nil, err
	}
	if callResponse == nil || callResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("node returned status %d while reading server definition", responseStatusCode(callResponse))
	}
	definition := &pufferpanel.Server{}
	if err = json.NewDecoder(callResponse.Body).Decode(definition); err != nil {
		return nil, err
	}
	return definition, nil
}

func saveDatabaseParentDefinition(ctx context.Context, ns *services.Node, parent *models.Server, definition *pufferpanel.Server) error {
	body, err := json.Marshal(definition)
	if err != nil {
		return err
	}
	callResponse, err := ns.CallNodeWithContext(ctx, &parent.Node, http.MethodPut, "/daemon/server/"+parent.Identifier+"/definition", io.NopCloser(strings.NewReader(string(body))), nil)
	defer utils.CloseResponse(callResponse)
	if err != nil {
		return err
	}
	if callResponse == nil || callResponse.StatusCode != http.StatusNoContent {
		return fmt.Errorf("node returned status %d while updating server definition", responseStatusCode(callResponse))
	}
	return nil
}

func cloneDatabaseDefinition(definition *pufferpanel.Server) *pufferpanel.Server {
	clone := *definition
	if definition.Execution.EnvironmentVariables != nil {
		clone.Execution.EnvironmentVariables = make(map[string]string, len(definition.Execution.EnvironmentVariables))
		for name, value := range definition.Execution.EnvironmentVariables {
			clone.Execution.EnvironmentVariables[name] = value
		}
	}
	return &clone
}

func restoreDatabaseDefinition(ns *services.Node, parent *models.Server, definition *pufferpanel.Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), services.NodeCallTimeout)
	defer cancel()
	return saveDatabaseParentDefinition(ctx, ns, parent, definition)
}

func responseStatusCode(response *http.Response) int {
	if response == nil {
		return 0
	}
	return response.StatusCode
}
