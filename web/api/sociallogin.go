package api

import (
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3/config"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/models"
	"github.com/pufferpanel/pufferpanel/v3/response"
	"github.com/pufferpanel/pufferpanel/v3/scopes"
	"github.com/pufferpanel/pufferpanel/v3/services"
	"gorm.io/gorm"
)

var socialProviderKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,39}$`)

type socialLoginAdminResponse struct {
	Providers           []socialProviderAdminView `json:"providers"`
	RedirectURL         string                    `json:"redirectUrl"`
	AllowRegistration   bool                      `json:"allowRegistration"`
	AllowEmailLinking   bool                      `json:"allowEmailLinking"`
	AllowAccountLinking bool                      `json:"allowAccountLinking"`
}

type socialProviderAdminView struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	ClientID  string `json:"clientId"`
	IssuerURL string `json:"issuerUrl,omitempty"`
	Enabled   bool   `json:"enabled"`
	HasSecret bool   `json:"hasSecret"`
}

type socialProviderRequest struct {
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	IssuerURL    string `json:"issuerUrl"`
	Enabled      bool   `json:"enabled"`
}

type socialLoginSettingsRequest struct {
	AllowRegistration   *bool `json:"allowRegistration"`
	AllowEmailLinking   *bool `json:"allowEmailLinking"`
	AllowAccountLinking *bool `json:"allowAccountLinking"`
}

func registerSocialLoginAdmin(g *gin.RouterGroup) {
	admin := middleware.RequiresPermission(scopes.ScopeAdmin)
	g.GET("/social-login", admin, getSocialLoginAdmin)
	g.PUT("/social-login/settings", admin, updateSocialLoginSettings)
	g.PUT("/social-login/providers/:key", admin, saveSocialProvider)
	g.DELETE("/social-login/providers/:key", admin, deleteSocialProvider)
	g.OPTIONS("/social-login", response.CreateOptions("GET"))
	g.OPTIONS("/social-login/settings", response.CreateOptions("PUT"))
	g.OPTIONS("/social-login/providers/:key", response.CreateOptions("PUT", "DELETE"))
}

func getSocialLoginAdmin(c *gin.Context) {
	db := middleware.GetDatabase(c)
	var providers []models.SocialProvider
	if err := db.Order("name ASC").Find(&providers).Error; response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	views := make([]socialProviderAdminView, 0, len(providers))
	for _, provider := range providers {
		views = append(views, socialProviderAdminView{
			Key: provider.Key, Name: provider.Name, Kind: provider.Kind,
			ClientID: provider.ClientID, IssuerURL: provider.IssuerURL,
			Enabled: provider.Enabled, HasSecret: provider.ClientSecretEncrypted != "",
		})
	}
	c.JSON(http.StatusOK, socialLoginAdminResponse{
		Providers:           views,
		RedirectURL:         strings.TrimRight(config.MasterUrl.Value(), "/") + "/auth/social/callback",
		AllowRegistration:   config.SocialLoginAllowRegistration.Value(),
		AllowEmailLinking:   config.SocialLoginAllowEmailLinking.Value(),
		AllowAccountLinking: config.SocialLoginAllowAccountLinking.Value(),
	})
}

func updateSocialLoginSettings(c *gin.Context) {
	var request socialLoginSettingsRequest
	if err := c.ShouldBindJSON(&request); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	if request.AllowRegistration == nil || request.AllowEmailLinking == nil || request.AllowAccountLinking == nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "all social login policies are required"})
		return
	}
	if err := config.SocialLoginAllowRegistration.Set(*request.AllowRegistration, true); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	if err := config.SocialLoginAllowEmailLinking.Set(*request.AllowEmailLinking, true); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	if err := config.SocialLoginAllowAccountLinking.Set(*request.AllowAccountLinking, true); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.Status(http.StatusNoContent)
}

func saveSocialProvider(c *gin.Context) {
	key := c.Param("key")
	if !socialProviderKeyPattern.MatchString(key) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid provider key"})
		return
	}
	var request socialProviderRequest
	if err := c.ShouldBindJSON(&request); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	request.ClientID = strings.TrimSpace(request.ClientID)
	request.IssuerURL = strings.TrimSpace(request.IssuerURL)
	if request.Name == "" || len(request.Name) > 100 || request.ClientID == "" || len(request.ClientID) > 255 || len(request.ClientSecret) > 2048 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "provider name and client ID are required"})
		return
	}
	if request.Kind != "google" && request.Kind != "discord" && request.Kind != "github" && request.Kind != "oidc" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "unsupported provider kind"})
		return
	}
	if request.Kind == "oidc" {
		issuer, err := url.ParseRequestURI(request.IssuerURL)
		if err != nil || issuer.Scheme != "https" || issuer.Host == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "OIDC issuer must be an HTTPS URL"})
			return
		}
	}

	db := middleware.GetDatabase(c)
	var provider models.SocialProvider
	err := db.Where("key = ?", key).First(&provider).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		response.HandleError(c, err, http.StatusInternalServerError)
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) && request.ClientSecret == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "client secret is required for a new provider"})
		return
	}
	if request.ClientSecret != "" {
		encrypted, encryptErr := services.EncryptSocialSecret(request.ClientSecret)
		if response.HandleError(c, encryptErr, http.StatusInternalServerError) {
			return
		}
		provider.ClientSecretEncrypted = encrypted
	}
	if request.Enabled && provider.ClientSecretEncrypted == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "client secret is required to enable a provider"})
		return
	}
	provider.Key = key
	provider.Name = request.Name
	provider.Kind = request.Kind
	provider.ClientID = request.ClientID
	provider.IssuerURL = request.IssuerURL
	provider.Enabled = request.Enabled
	if err := db.Save(&provider).Error; response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.Status(http.StatusNoContent)
}

func deleteSocialProvider(c *gin.Context) {
	key := c.Param("key")
	if !socialProviderKeyPattern.MatchString(key) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid provider key"})
		return
	}
	db := middleware.GetDatabase(c)
	err := db.Transaction(func(tx *gorm.DB) error {
		var provider models.SocialProvider
		if err := tx.Where("key = ?", key).First(&provider).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if err := tx.Where("provider_id = ?", provider.ID).Delete(&models.SocialConnection{}).Error; err != nil {
			return err
		}
		return tx.Delete(&provider).Error
	})
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.Status(http.StatusNoContent)
}
