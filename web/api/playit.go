package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/models"
	"github.com/pufferpanel/pufferpanel/v3/response"
	"github.com/pufferpanel/pufferpanel/v3/scopes"
	"github.com/pufferpanel/pufferpanel/v3/services"
	"github.com/pufferpanel/pufferpanel/v3/utils"
)

func registerPlayitServerRoutes(g *gin.RouterGroup) {
	admin := middleware.RequiresPermission(scopes.ScopeAdmin)
	g.GET("/:serverId/playit", admin, middleware.ResolveServerPanel, getPlayitState)
	g.GET("/:serverId/playit/login-url", admin, middleware.ResolveServerPanel, getPlayitLoginURL)
	g.PUT("/:serverId/playit/secret", admin, middleware.ResolveServerPanel, setPlayitSecret)
	g.POST("/:serverId/playit/stop", admin, middleware.ResolveServerPanel, stopPlayitAgent)
	g.OPTIONS("/:serverId/playit", response.CreateOptions("GET"))
	g.OPTIONS("/:serverId/playit/login-url", response.CreateOptions("GET"))
	g.OPTIONS("/:serverId/playit/secret", response.CreateOptions("PUT"))
	g.OPTIONS("/:serverId/playit/stop", response.CreateOptions("POST"))
}

func getPlayitState(c *gin.Context) {
	proxyPlayitRequest(c, http.MethodGet, "/daemon/playit/state", nil)
}

func getPlayitLoginURL(c *gin.Context) {
	proxyPlayitRequest(c, http.MethodGet, "/daemon/playit/login-url", nil)
}

func setPlayitSecret(c *gin.Context) {
	var request struct {
		Secret string `json:"secret"`
	}
	if err := c.ShouldBindJSON(&request); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	if len(request.Secret) == 0 || len(request.Secret) > 4096 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "a valid Playit secret is required"}})
		return
	}
	body, err := json.Marshal(request)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	proxyPlayitRequest(c, http.MethodPut, "/daemon/playit/secret", body)
}

func stopPlayitAgent(c *gin.Context) {
	proxyPlayitRequest(c, http.MethodPost, "/daemon/playit/stop", []byte("{}"))
}

func proxyPlayitRequest(c *gin.Context, method, path string, body []byte) {
	server := c.MustGet("server").(*models.Server)
	var requestBody io.ReadCloser
	if body != nil {
		requestBody = io.NopCloser(bytes.NewReader(body))
	}
	headers := http.Header{"Content-Type": []string{"application/json"}}
	remoteResponse, err := (&services.Node{DB: middleware.GetDatabase(c)}).CallNode(&server.Node, method, path, requestBody, headers)
	if response.HandleError(c, err, http.StatusBadGateway) {
		utils.CloseResponse(remoteResponse)
		return
	}
	if remoteResponse == nil || remoteResponse.Body == nil {
		c.AbortWithStatus(http.StatusBadGateway)
		return
	}
	defer utils.CloseResponse(remoteResponse)
	data, err := io.ReadAll(io.LimitReader(remoteResponse.Body, 1<<20))
	if response.HandleError(c, err, http.StatusBadGateway) {
		return
	}
	contentType := remoteResponse.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(remoteResponse.StatusCode, contentType, data)
}
