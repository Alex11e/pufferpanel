package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3/logging"
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
	username := ""
	if user, ok := c.Get("user"); ok {
		username = user.(*models.User).Username
	} else if client, ok := c.Get("client"); ok {
		apiClient := client.(*models.Client)
		username = apiClient.Name
		if username == "" {
			username = apiClient.ClientId
		}
	} else {
		return
	}
	activity := &models.Activity{ServerIdentifier: serverID, Username: username, Action: action, Details: details, IPAddress: c.ClientIP()}
	if err := middleware.GetDatabase(c).Create(activity).Error; err != nil {
		logging.Error.Printf("could not record activity for server %s: %s", serverID, err)
	}
}

func activityFromRequest(c *gin.Context) (action, serverID, details string) {
	return classifyActivity(c.Request.Method, c.Request.URL.Path)
}

func classifyActivity(method, requestPath string) (action, serverID, details string) {
	parts := strings.Split(strings.Trim(requestPath, "/"), "/")
	if len(parts) < 3 || parts[0] != "api" || parts[1] != "servers" {
		return
	}
	serverID = parts[2]
	if serverID == "" {
		return "", "", ""
	}
	if len(parts) == 3 {
		switch method {
		case http.MethodPut:
			action = "server.create"
		case http.MethodDelete:
			action = "server.delete"
		}
		return
	}

	switch parts[3] {
	case "start", "restart", "stop", "kill", "install", "reload":
		if len(parts) == 4 && method == http.MethodPost {
			action = "server." + parts[3]
		}
	case "name":
		if len(parts) == 5 && method == http.MethodPut {
			action, details = "server.rename", parts[4]
		}
	case "metadata":
		if len(parts) == 4 && method == http.MethodPut {
			action = "server.metadata.update"
		}
	case "database":
		if len(parts) == 4 && method == http.MethodPut {
			action = "server.database.attach"
		} else if len(parts) == 4 && method == http.MethodDelete {
			action = "server.database.detach"
		}
	case "definition":
		if len(parts) == 4 && method == http.MethodPut {
			action = "server.definition.update"
		}
	case "data":
		if len(parts) == 4 && (method == http.MethodPost || method == http.MethodPut) {
			action = "server.data.update"
		}
	case "console":
		if len(parts) == 4 && method == http.MethodPost {
			action = "server.console.command"
		}
	case "file":
		if len(parts) < 5 {
			return
		}
		details = strings.Join(parts[4:], "/")
		switch method {
		case http.MethodGet:
			action = "server.file.read"
		case http.MethodPut, http.MethodPost:
			action = "server.file.write"
		case http.MethodDelete:
			action = "server.file.delete"
		}
	case "plugins":
		if len(parts) == 5 && parts[4] == "download" && method == http.MethodPost {
			action, details = "server.plugin.install", "plugins/"
		}
	case "backup":
		if len(parts) == 5 && parts[4] == "create" && method == http.MethodPost {
			action = "server.backup.create"
		} else if len(parts) == 5 && parts[4] == "automatic" && method == http.MethodPut {
			action = "server.backup.automatic.update"
		} else if len(parts) == 5 && method == http.MethodDelete {
			action, details = "server.backup.delete", parts[4]
		} else if len(parts) == 6 && parts[4] == "restore" && method == http.MethodPost {
			action, details = "server.backup.restore", parts[5]
		} else if len(parts) == 6 && parts[4] == "download" && method == http.MethodGet {
			action, details = "server.backup.download", parts[5]
		}
	case "tasks":
		if len(parts) == 6 && parts[5] == "run" && method == http.MethodPost {
			action, details = "server.task.run", parts[4]
		}
	case "archive", "extract":
		if len(parts) >= 5 && method == http.MethodPost {
			action = "server.file." + parts[3]
			details = strings.Join(parts[4:], "/")
		}
	}
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
