package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/models"
	"github.com/pufferpanel/pufferpanel/v3/response"
	"github.com/pufferpanel/pufferpanel/v3/scopes"
)

func registerAnnouncements(g *gin.RouterGroup) {
	g.GET("", middleware.RequiresPermission(scopes.ScopeLogin), getAnnouncements)
	g.OPTIONS("", response.CreateOptions("GET"))
	admin := g.Group("/admin", middleware.RequiresPermission(scopes.ScopeAdmin))
	admin.GET("", getAdminAnnouncements)
	admin.POST("", createAnnouncement)
	admin.PUT("/:id", updateAnnouncement)
	admin.DELETE("/:id", deleteAnnouncement)
	admin.OPTIONS("", response.CreateOptions("GET", "POST"))
	admin.OPTIONS("/:id", response.CreateOptions("PUT", "DELETE"))
}

func getAnnouncements(c *gin.Context) {
	var announcements []models.Announcement
	if err := middleware.GetDatabase(c).Where("active = ? AND (expires_at IS NULL OR expires_at > ?)", true, time.Now()).Order("created_at DESC").Limit(10).Find(&announcements).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, announcements)
}

func getAdminAnnouncements(c *gin.Context) {
	var announcements []models.Announcement
	if err := middleware.GetDatabase(c).Order("created_at DESC").Limit(100).Find(&announcements).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, announcements)
}

func normalizeAnnouncement(announcement *models.Announcement) bool {
	announcement.Title = strings.TrimSpace(announcement.Title)
	announcement.Message = strings.TrimSpace(announcement.Message)
	announcement.Level = strings.ToLower(strings.TrimSpace(announcement.Level))
	if announcement.Level == "" {
		announcement.Level = "info"
	}
	return announcement.Title != "" && len(announcement.Title) <= 140 && announcement.Message != "" && len(announcement.Message) <= 4000 && (announcement.Level == "info" || announcement.Level == "warning" || announcement.Level == "maintenance")
}

func createAnnouncement(c *gin.Context) {
	var announcement models.Announcement
	if err := c.ShouldBindJSON(&announcement); err != nil || !normalizeAnnouncement(&announcement) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid announcement"})
		return
	}
	if err := middleware.GetDatabase(c).Create(&announcement).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusCreated, announcement)
}

func updateAnnouncement(c *gin.Context) {
	var announcement models.Announcement
	if err := middleware.GetDatabase(c).First(&announcement, c.Param("id")).Error; err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	var input models.Announcement
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid announcement"})
		return
	}
	announcement.Title, announcement.Message, announcement.Level, announcement.Active, announcement.ExpiresAt = input.Title, input.Message, input.Level, input.Active, input.ExpiresAt
	if !normalizeAnnouncement(&announcement) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid announcement"})
		return
	}
	if err := middleware.GetDatabase(c).Save(&announcement).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, announcement)
}

func deleteAnnouncement(c *gin.Context) {
	result := middleware.GetDatabase(c).Delete(&models.Announcement{}, c.Param("id"))
	if result.Error != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if result.RowsAffected == 0 {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Status(http.StatusNoContent)
}
