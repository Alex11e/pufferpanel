package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/models"
	"github.com/pufferpanel/pufferpanel/v3/response"
	"github.com/pufferpanel/pufferpanel/v3/scopes"
	"github.com/pufferpanel/pufferpanel/v3/web/auth"
)

type socialConnectionView struct {
	Key           string `json:"key"`
	Provider      string `json:"provider"`
	DisplayName   string `json:"displayName"`
	Email         string `json:"email,omitempty"`
	CanDisconnect bool   `json:"canDisconnect"`
}

func registerSelfSocial(g *gin.RouterGroup) {
	permission := middleware.RequiresPermission(scopes.ScopeSelfEdit)
	g.GET("/social", permission, listSocialConnections)
	g.GET("/social/:provider/connect", permission, auth.BeginSocialConnect)
	g.DELETE("/social/:provider", permission, deleteSocialConnection)
	g.OPTIONS("/social", response.CreateOptions("GET"))
	g.OPTIONS("/social/:provider/connect", response.CreateOptions("GET"))
	g.OPTIONS("/social/:provider", response.CreateOptions("DELETE"))
}

func listSocialConnections(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	db := middleware.GetDatabase(c)
	var connections []models.SocialConnection
	if err := db.Preload("Provider").Where("user_id = ?", user.ID).Order("created_at ASC").Find(&connections).Error; response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	canUsePasskey := false
	if user.AllowPasswordlessLogin {
		var credentialCount int64
		if err := db.Model(&models.WebauthnCredential{}).Where("user_id = ?", user.ID).Count(&credentialCount).Error; response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
		canUsePasskey = credentialCount > 0
	}
	views := make([]socialConnectionView, 0, len(connections))
	canDisconnect := len(connections) > 1 || canUsePasskey
	for _, connection := range connections {
		views = append(views, socialConnectionView{
			Key: connection.Provider.Key, Provider: connection.Provider.Name,
			DisplayName: connection.DisplayName, Email: connection.Email, CanDisconnect: canDisconnect,
		})
	}
	c.JSON(http.StatusOK, views)
}

func deleteSocialConnection(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	key := c.Param("provider")
	db := middleware.GetDatabase(c)
	var provider models.SocialProvider
	if err := db.Where("key = ?", key).First(&provider).Error; response.HandleError(c, err, http.StatusNotFound) {
		return
	}
	var connectionCount int64
	if err := db.Model(&models.SocialConnection{}).Where("user_id = ?", user.ID).Count(&connectionCount).Error; response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	canUsePasskey := false
	if user.AllowPasswordlessLogin {
		var credentialCount int64
		if err := db.Model(&models.WebauthnCredential{}).Where("user_id = ?", user.ID).Count(&credentialCount).Error; response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
		canUsePasskey = credentialCount > 0
	}
	if connectionCount <= 1 && !canUsePasskey {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": gin.H{"msg": "Add another sign-in method before disconnecting this account."}})
		return
	}
	if err := db.Where("user_id = ? AND provider_id = ?", user.ID, provider.ID).Delete(&models.SocialConnection{}).Error; response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.Status(http.StatusNoContent)
}
