package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/providers/contabo"
	"github.com/pufferpanel/pufferpanel/v3/response"
	"github.com/pufferpanel/pufferpanel/v3/scopes"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func registerContabo(g *gin.RouterGroup) {
	g.GET("/instances", middleware.RequiresPermission(scopes.ScopeAdmin), getContaboInstances)
	g.OPTIONS("/instances", response.CreateOptions("GET"))
	g.GET("/instances/:id", middleware.RequiresPermission(scopes.ScopeAdmin), getContaboInstance)
	g.OPTIONS("/instances/:id", response.CreateOptions("GET"))
	g.GET("/instances/:id/ssh", middleware.RequiresPermission(scopes.ScopeAdmin), openContaboSSH)
	g.POST("/instances/:id/actions/:action", middleware.RequiresPermission(scopes.ScopeAdmin), performContaboInstanceAction)
	g.OPTIONS("/instances/:id/actions/:action", response.CreateOptions("POST"))
	g.PUT("/instances/:id/reinstall", middleware.RequiresPermission(scopes.ScopeAdmin), reinstallContaboInstance)
	g.OPTIONS("/instances/:id/reinstall", response.CreateOptions("PUT"))
	g.POST("/instances/:id/firewall-addon", middleware.RequiresPermission(scopes.ScopeAdmin), enableContaboFirewallAddon)
	g.OPTIONS("/instances/:id/firewall-addon", response.CreateOptions("POST"))

	g.GET("/images", middleware.RequiresPermission(scopes.ScopeAdmin), getContaboImages)
	g.POST("/images", middleware.RequiresPermission(scopes.ScopeAdmin), createContaboImage)
	g.OPTIONS("/images", response.CreateOptions("GET", "POST"))

	g.GET("/firewalls", middleware.RequiresPermission(scopes.ScopeAdmin), getContaboFirewalls)
	g.POST("/firewalls", middleware.RequiresPermission(scopes.ScopeAdmin), createContaboFirewall)
	g.OPTIONS("/firewalls", response.CreateOptions("GET", "POST"))
	g.PUT("/firewalls/:id/rules", middleware.RequiresPermission(scopes.ScopeAdmin), updateContaboFirewallRules)
	g.OPTIONS("/firewalls/:id/rules", response.CreateOptions("PUT"))
	g.POST("/firewalls/:id/instances/:instanceId", middleware.RequiresPermission(scopes.ScopeAdmin), assignContaboFirewall)
	g.OPTIONS("/firewalls/:id/instances/:instanceId", response.CreateOptions("POST"))
	g.DELETE("/firewalls/:id/instances/:instanceId", middleware.RequiresPermission(scopes.ScopeAdmin), unassignContaboFirewall)
	g.OPTIONS("/firewalls/:id/instances/:instanceId", response.CreateOptions("DELETE"))
}

var contaboSSHUpgrader = websocket.Upgrader{
	HandshakeTimeout: 10 * time.Second,
	CheckOrigin: func(request *http.Request) bool {
		origin, err := url.Parse(request.Header.Get("Origin"))
		return err == nil && origin.Host != "" && strings.EqualFold(origin.Host, request.Host)
	},
}

type contaboInstanceAddress struct {
	Data []struct {
		DefaultUser string `json:"defaultUser"`
		IPAddress   string `json:"ipAddress"`
		IPConfig    struct {
			V4 struct {
				IP string `json:"ip"`
			} `json:"v4"`
		} `json:"ipConfig"`
	} `json:"data"`
}

func openContaboSSH(c *gin.Context) {
	id, ok := contaboInstanceID(c, "id")
	if !ok {
		return
	}
	privateKeyPath := os.Getenv("PUFFER_CONTABO_SSH_PRIVATE_KEY_FILE")
	knownHostsPath := os.Getenv("PUFFER_CONTABO_SSH_KNOWN_HOSTS_FILE")
	if privateKeyPath == "" || knownHostsPath == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"msg": "Contabo SSH requires the panel's private key and known_hosts file to be configured"}})
		return
	}

	apiClient, err := contabo.NewClientFromEnv()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"msg": err.Error()}})
		return
	}
	instanceResponse, err := apiClient.Do(c.Request.Context(), http.MethodGet, "/v1/compute/instances/"+id, nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"msg": err.Error()}})
		return
	}
	var instance contaboInstanceAddress
	if err = json.Unmarshal(instanceResponse, &instance); err != nil || len(instance.Data) == 0 {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"msg": "Contabo did not return instance connection details"}})
		return
	}
	address := instance.Data[0].IPConfig.V4.IP
	if address == "" {
		address = instance.Data[0].IPAddress
	}
	if net.ParseIP(address) == nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"msg": "Contabo did not return a public IPv4 address for SSH"}})
		return
	}

	privateKey, err := os.ReadFile(privateKeyPath)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"msg": "could not read the configured SSH private key"}})
		return
	}
	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"msg": "the configured SSH private key is invalid or encrypted"}})
		return
	}
	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"msg": "could not load the configured SSH known_hosts file"}})
		return
	}
	user := instance.Data[0].DefaultUser
	if user == "" {
		user = "root"
	}
	if user != "root" && user != "admin" && user != "administrator" {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"msg": "Contabo returned an unsupported SSH user"}})
		return
	}
	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: hostKeyCallback,
		Timeout:         15 * time.Second,
	}
	sshClient, err := ssh.Dial("tcp", net.JoinHostPort(address, "22"), sshConfig)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"msg": "SSH connection failed; check the host key, firewall, and SSH service"}})
		return
	}
	defer sshClient.Close()

	session, err := sshClient.NewSession()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"msg": "could not open an SSH session"}})
		return
	}
	defer session.Close()
	stdin, err := session.StdinPipe()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"msg": "could not open SSH input"}})
		return
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"msg": "could not open SSH output"}})
		return
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"msg": "could not open SSH error output"}})
		return
	}
	if err = session.RequestPty("xterm-256color", 24, 80, ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400}); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"msg": "could not allocate a remote terminal"}})
		return
	}
	if err = session.Shell(); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"msg": "could not start the remote shell"}})
		return
	}

	connection, err := contaboSSHUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	writer := &contaboSSHWriter{connection: connection}
	go func() { _, _ = io.Copy(writer, stdout) }()
	go func() { _, _ = io.Copy(writer, stderr) }()
	go func() {
		_ = session.Wait()
		_ = connection.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "SSH session ended"), time.Now().Add(time.Second))
		_ = connection.Close()
	}()

	for {
		messageType, message, readErr := connection.ReadMessage()
		if readErr != nil {
			return
		}
		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}
		if _, err = stdin.Write(message); err != nil {
			return
		}
	}
}

type contaboSSHWriter struct {
	connection *websocket.Conn
	mu         sync.Mutex
}

func (w *contaboSSHWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.connection.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return 0, err
	}
	return len(data), nil
}

func getContaboInstances(c *gin.Context) {
	forwardContaboRequest(c, http.MethodGet, "/v1/compute/instances?page=1&size=100", nil)
}

func getContaboInstance(c *gin.Context) {
	id, ok := contaboInstanceID(c, "id")
	if !ok {
		return
	}
	forwardContaboRequest(c, http.MethodGet, "/v1/compute/instances/"+id, nil)
}

func performContaboInstanceAction(c *gin.Context) {
	id, ok := contaboInstanceID(c, "id")
	if !ok {
		return
	}
	action := c.Param("action")
	if action != "start" && action != "restart" && action != "stop" && action != "shutdown" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "unsupported Contabo instance action"}})
		return
	}
	forwardContaboRequest(c, http.MethodPost, "/v1/compute/instances/"+id+"/actions/"+action, nil)
}

type contaboReinstallRequest struct {
	ImageID      string  `json:"imageId"`
	DefaultUser  string  `json:"defaultUser"`
	UserData     string  `json:"userData"`
	SSHKeys      []int64 `json:"sshKeys"`
	RootPassword int64   `json:"rootPassword"`
	ConfirmWipe  bool    `json:"confirmWipe"`
}

func reinstallContaboInstance(c *gin.Context) {
	id, ok := contaboInstanceID(c, "id")
	if !ok {
		return
	}
	var request contaboReinstallRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "invalid reinstall request"}})
		return
	}
	if !request.ConfirmWipe {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "confirmWipe must be true; reinstall erases the VPS disk"}})
		return
	}
	if _, err := uuid.Parse(request.ImageID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "imageId must be a valid Contabo image ID"}})
		return
	}
	if request.DefaultUser != "root" && request.DefaultUser != "admin" && request.DefaultUser != "administrator" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "defaultUser must be root, admin, or administrator"}})
		return
	}
	if len(request.UserData) > 65536 || request.RootPassword < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "reinstall settings are outside the supported limits"}})
		return
	}
	for _, key := range request.SSHKeys {
		if key <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "SSH key IDs must be positive Contabo secret IDs"}})
			return
		}
	}

	body := map[string]any{"imageId": request.ImageID, "defaultUser": request.DefaultUser}
	if request.UserData != "" {
		body["userData"] = request.UserData
	}
	if len(request.SSHKeys) > 0 {
		body["sshKeys"] = request.SSHKeys
	}
	if request.RootPassword > 0 {
		body["rootPassword"] = request.RootPassword
	}
	forwardContaboRequest(c, http.MethodPut, "/v1/compute/instances/"+id, body)
}

func enableContaboFirewallAddon(c *gin.Context) {
	id, ok := contaboInstanceID(c, "id")
	if !ok {
		return
	}
	var request struct {
		Confirmation string `json:"confirmation"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.Confirmation != "BUY FIREWALL ADD-ON" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "explicit confirmation is required for the paid firewall add-on"}})
		return
	}
	forwardContaboRequest(c, http.MethodPost, "/v1/compute/instances/"+id+"/upgrade", map[string]any{"firewall": map[string]any{}})
}

func getContaboImages(c *gin.Context) {
	forwardContaboRequest(c, http.MethodGet, "/v1/compute/images?page=1&size=100", nil)
}

type contaboCustomImageRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	OSType      string `json:"osType"`
	Version     string `json:"version"`
}

func createContaboImage(c *gin.Context) {
	var request contaboCustomImageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "invalid custom image request"}})
		return
	}
	imageURL, err := url.ParseRequestURI(request.URL)
	if err != nil || imageURL.Scheme != "https" || imageURL.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "custom images require a public HTTPS URL"}})
		return
	}
	if !strings.HasSuffix(strings.ToLower(imageURL.Path), ".iso") && !strings.HasSuffix(strings.ToLower(imageURL.Path), ".qcow2") {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "custom image URL must end in .iso or .qcow2"}})
		return
	}
	if strings.TrimSpace(request.Name) == "" || strings.TrimSpace(request.Version) == "" || (request.OSType != "Linux" && request.OSType != "Windows") {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "name, version, and a supported OS type are required"}})
		return
	}
	forwardContaboRequest(c, http.MethodPost, "/v1/compute/images", request)
}

func getContaboFirewalls(c *gin.Context) {
	forwardContaboRequest(c, http.MethodGet, "/v1/firewalls?page=1&size=100", nil)
}

type contaboFirewallRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Status      string          `json:"status"`
	Rules       json.RawMessage `json:"rules,omitempty"`
}

func createContaboFirewall(c *gin.Context) {
	var request contaboFirewallRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "invalid firewall request"}})
		return
	}
	if strings.TrimSpace(request.Name) == "" || (request.Status != "active" && request.Status != "inactive") {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "firewall name and valid status are required"}})
		return
	}
	if len(request.Rules) > 0 && !json.Valid(request.Rules) {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "rules must be valid JSON"}})
		return
	}
	forwardContaboRequest(c, http.MethodPost, "/v1/firewalls", request)
}

func updateContaboFirewallRules(c *gin.Context) {
	id, ok := contaboResourceID(c, "id")
	if !ok {
		return
	}
	var request struct {
		Rules json.RawMessage `json:"rules"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || len(request.Rules) == 0 || !json.Valid(request.Rules) {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "rules must be valid JSON"}})
		return
	}
	var rules map[string]json.RawMessage
	if err := json.Unmarshal(request.Rules, &rules); err != nil || len(rules["inbound"]) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "rules must include an inbound array"}})
		return
	}
	forwardContaboRequest(c, http.MethodPut, "/v1/firewalls/"+id, map[string]json.RawMessage{"rules": request.Rules})
}

func assignContaboFirewall(c *gin.Context) {
	firewallID, ok := contaboResourceID(c, "id")
	if !ok {
		return
	}
	instanceID, ok := contaboInstanceID(c, "instanceId")
	if !ok {
		return
	}
	forwardContaboRequest(c, http.MethodPost, "/v1/firewalls/"+firewallID+"/instances/"+instanceID, nil)
}

func unassignContaboFirewall(c *gin.Context) {
	firewallID, ok := contaboResourceID(c, "id")
	if !ok {
		return
	}
	instanceID, ok := contaboInstanceID(c, "instanceId")
	if !ok {
		return
	}
	forwardContaboRequest(c, http.MethodDelete, "/v1/firewalls/"+firewallID+"/instances/"+instanceID, nil)
}

func contaboInstanceID(c *gin.Context, parameter string) (string, bool) {
	value := c.Param(parameter)
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": fmt.Sprintf("%s must be a positive instance ID", parameter)}})
		return "", false
	}
	return strconv.FormatUint(id, 10), true
}

func contaboResourceID(c *gin.Context, parameter string) (string, bool) {
	value := c.Param(parameter)
	if _, err := uuid.Parse(value); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": fmt.Sprintf("%s must be a valid resource ID", parameter)}})
		return "", false
	}
	return value, true
}

func forwardContaboRequest(c *gin.Context, method, path string, body any) {
	client, err := contabo.NewClientFromEnv()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"msg": err.Error()}})
		return
	}
	result, err := client.Do(c.Request.Context(), method, path, body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"msg": err.Error()}})
		return
	}
	c.Data(http.StatusOK, "application/json", result)
}
