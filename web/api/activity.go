package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/models"
)

func recordActivity(c *gin.Context) {
	c.Next()
	if c.Writer.Status() >= http.StatusBadRequest || !strings.HasPrefix(c.Request.URL.Path, "/api/servers/") {
		return
	}
	action, serverID, details := activityFromRequest(c)
	if action == "" || serverID == "" {
		return
	}
	user, ok := c.Get("user")
	if !ok {
		return
	}
	username := user.(*models.User).Username
	activity := &models.Activity{ServerIdentifier: serverID, Username: username, Action: action, Details: details, IPAddress: c.ClientIP()}
	_ = middleware.GetDatabase(c).Create(activity).Error
}

func activityFromRequest(c *gin.Context) (action, serverID, details string) {
	parts := strings.Split(strings.Trim(c.Request.URL.Path, "/"), "/")
	if len(parts) < 3 { return }
	serverID = parts[2]
	if len(parts) >= 4 && parts[3] == "file" {
		details = strings.Join(parts[4:], "/")
		switch c.Request.Method { case http.MethodGet: action = "server.file.read"; case http.MethodPut, http.MethodPost: action = "server.file.write"; case http.MethodDelete: action = "server.file.delete" }
		return
	}
	if len(parts) >= 5 && parts[3] == "plugins" && parts[4] == "download" && c.Request.Method == http.MethodPost {
		action = "server.plugin.install"
		details = "plugins/"
		return
	}
	if len(parts) >= 5 && parts[3] == "backup" && parts[4] == "create" && c.Request.Method == http.MethodPost { action = "server.backup.create" }
	return
}

func getServerActivity(c *gin.Context) {
	server := getServerFromGin(c)
	var records []models.Activity
	if err := middleware.GetDatabase(c).Where("server_identifier = ?", server.Identifier).Order("created_at DESC").Limit(100).Find(&records).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, records)
}

func getRecentActivity(c *gin.Context) {
	var records []models.Activity
	if err := middleware.GetDatabase(c).Order("created_at DESC").Limit(12).Find(&records).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, records)
}
