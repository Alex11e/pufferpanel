package api

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/models"
	"github.com/pufferpanel/pufferpanel/v3/response"
	"github.com/pufferpanel/pufferpanel/v3/scopes"
	"github.com/pufferpanel/pufferpanel/v3/services"
)

var folderColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type folderView struct {
	ID        uint     `json:"id"`
	Name      string   `json:"name"`
	Color     string   `json:"color"`
	ServerIDs []string `json:"serverIds"`
}
type folderRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

func registerFolders(g *gin.RouterGroup) {
	g.Use(middleware.RequiresPermission(scopes.ScopeLogin))
	g.GET("", getFolders)
	g.POST("", createFolder)
	g.PUT("/:id", updateFolder)
	g.DELETE("/:id", deleteFolder)
	g.PUT("/:id/servers/:serverId", assignFolderServer)
	g.DELETE("/:id/servers/:serverId", unassignFolderServer)
	g.OPTIONS("", response.CreateOptions("GET", "POST"))
	g.OPTIONS("/:id", response.CreateOptions("PUT", "DELETE"))
	g.OPTIONS("/:id/servers/:serverId", response.CreateOptions("PUT", "DELETE"))
}

func folderOwner(c *gin.Context) *models.User { return c.MustGet("user").(*models.User) }
func loadFolder(c *gin.Context) *models.ServerFolder {
	folder := &models.ServerFolder{}
	if err := middleware.GetDatabase(c).Where("id = ? AND user_id = ?", c.Param("id"), folderOwner(c).ID).First(folder).Error; err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return nil
	}
	return folder
}
func validFolder(request *folderRequest) bool {
	request.Name = strings.TrimSpace(request.Name)
	request.Color = strings.TrimSpace(request.Color)
	if request.Color == "" {
		request.Color = "#4f7cff"
	}
	return request.Name != "" && len(request.Name) <= 60 && folderColorPattern.MatchString(request.Color)
}

func getFolders(c *gin.Context) {
	user := folderOwner(c)
	db := middleware.GetDatabase(c)
	var folders []models.ServerFolder
	if err := db.Where("user_id = ?", user.ID).Order("name").Find(&folders).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	var items []models.ServerFolderItem
	if err := db.Where("user_id = ?", user.ID).Find(&items).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	servers := make(map[uint][]string)
	for _, item := range items {
		servers[item.FolderID] = append(servers[item.FolderID], item.ServerIdentifier)
	}
	result := make([]folderView, 0, len(folders))
	for _, folder := range folders {
		result = append(result, folderView{ID: folder.ID, Name: folder.Name, Color: folder.Color, ServerIDs: servers[folder.ID]})
	}
	c.JSON(http.StatusOK, result)
}

func createFolder(c *gin.Context) {
	var request folderRequest
	if err := c.ShouldBindJSON(&request); err != nil || !validFolder(&request) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid folder"})
		return
	}
	folder := models.ServerFolder{UserID: folderOwner(c).ID, Name: request.Name, Color: request.Color}
	if err := middleware.GetDatabase(c).Create(&folder).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusCreated, folderView{ID: folder.ID, Name: folder.Name, Color: folder.Color, ServerIDs: []string{}})
}
func updateFolder(c *gin.Context) {
	folder := loadFolder(c)
	if folder == nil {
		return
	}
	var request folderRequest
	if err := c.ShouldBindJSON(&request); err != nil || !validFolder(&request) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid folder"})
		return
	}
	folder.Name, folder.Color = request.Name, request.Color
	if err := middleware.GetDatabase(c).Save(folder).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, folderView{ID: folder.ID, Name: folder.Name, Color: folder.Color})
}
func deleteFolder(c *gin.Context) {
	folder := loadFolder(c)
	if folder == nil {
		return
	}
	db := middleware.GetDatabase(c)
	if err := db.Where("folder_id = ?", folder.ID).Delete(&models.ServerFolderItem{}).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if err := db.Delete(folder).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNoContent)
}

func canOrganizeServer(c *gin.Context, id string) bool {
	allowed, err := (&services.Permission{DB: middleware.GetDatabase(c)}).HasPermission(folderOwner(c).ID, id, scopes.ScopeServerView)
	return err == nil && allowed
}
func assignFolderServer(c *gin.Context) {
	folder := loadFolder(c)
	if folder == nil {
		return
	}
	serverID := c.Param("serverId")
	if !canOrganizeServer(c, serverID) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	item := models.ServerFolderItem{FolderID: folder.ID, UserID: folderOwner(c).ID, ServerIdentifier: serverID}
	db := middleware.GetDatabase(c)
	if err := db.Where("user_id = ? AND server_identifier = ?", item.UserID, item.ServerIdentifier).Delete(&models.ServerFolderItem{}).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if err := db.Create(&item).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNoContent)
}
func unassignFolderServer(c *gin.Context) {
	folder := loadFolder(c)
	if folder == nil {
		return
	}
	result := middleware.GetDatabase(c).Where("folder_id = ? AND user_id = ? AND server_identifier = ?", folder.ID, folderOwner(c).ID, c.Param("serverId")).Delete(&models.ServerFolderItem{})
	if result.Error != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNoContent)
}
