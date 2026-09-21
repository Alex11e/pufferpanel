package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/models"
	"github.com/pufferpanel/pufferpanel/v3/response"
	"github.com/pufferpanel/pufferpanel/v3/scopes"
	"github.com/pufferpanel/pufferpanel/v3/services"
)

// AdminOverview deliberately contains only operational totals. It is useful on
// the dashboard without exposing secrets, passwords, server definitions, or
// node deployment tokens.
type AdminOverview struct {
	Servers              int64 `json:"servers"`
	Users                int64 `json:"users"`
	Nodes                int   `json:"nodes"`
	AllocatedPorts       int64 `json:"allocatedPorts"`
	Backups              int64 `json:"backups"`
	AutomaticBackupUsers int64 `json:"automaticBackupServers"`
	AllServerAccess      bool  `json:"allServerAccess"`
}

type AdminPortUsage struct {
	Node     *models.NodeView `json:"node"`
	Used     int64            `json:"used"`
	Capacity uint32           `json:"capacity"`
	Free     uint32           `json:"free"`
}

type AdminBackupView struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	FileName   string `json:"fileName"`
	ServerID   string `json:"serverId"`
	ServerName string `json:"serverName"`
	CreatedAt  string `json:"createdAt"`
}

func registerAdmin(g *gin.RouterGroup) {
	g.GET("/overview", middleware.RequiresPermission(scopes.ScopeAdmin), getAdminOverview)
	g.GET("/ports", middleware.RequiresPermission(scopes.ScopeAdmin), getAdminPortUsage)
	g.GET("/backups", middleware.RequiresPermission(scopes.ScopeAdmin), getAdminBackups)
	g.OPTIONS("/overview", response.CreateOptions("GET"))
	g.OPTIONS("/ports", response.CreateOptions("GET"))
	g.OPTIONS("/backups", response.CreateOptions("GET"))
}

func getAdminOverview(c *gin.Context) {
	db := middleware.GetDatabase(c)
	overview := AdminOverview{AllServerAccess: true}
	if err := db.Model(&models.Server{}).Count(&overview.Servers).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if err := db.Model(&models.User{}).Count(&overview.Users).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if err := db.Model(&models.Allocation{}).Count(&overview.AllocatedPorts).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if err := db.Model(&models.Backup{}).Count(&overview.Backups).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if err := db.Model(&models.Server{}).Where("auto_backup_enabled = ?", true).Count(&overview.AutomaticBackupUsers).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	nodes, err := (&services.Node{DB: db}).GetAll()
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	overview.Nodes = len(nodes)
	c.JSON(http.StatusOK, overview)
}

func getAdminPortUsage(c *gin.Context) {
	db := middleware.GetDatabase(c)
	nodes, err := (&services.Node{DB: db}).GetAll()
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	result := make([]AdminPortUsage, 0, len(nodes))
	for _, node := range nodes {
		var used int64
		if err = db.Model(&models.Allocation{}).Where("node_id = ?", node.ID).Count(&used).Error; err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		capacity := uint32(node.PortRangeEnd) - uint32(node.PortRangeStart) + 1
		free := capacity
		if used >= int64(capacity) {
			free = 0
		} else {
			free -= uint32(used)
		}
		result = append(result, AdminPortUsage{Node: models.FromNode(node), Used: used, Capacity: capacity, Free: free})
	}
	c.JSON(http.StatusOK, result)
}

func getAdminBackups(c *gin.Context) {
	var backups []models.Backup
	if err := middleware.GetDatabase(c).Preload("Server").Order("created_at DESC").Limit(25).Find(&backups).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	result := make([]AdminBackupView, 0, len(backups))
	for _, backup := range backups {
		result = append(result, AdminBackupView{
			ID: backup.ID, Name: backup.Name, FileName: backup.FileName, ServerID: backup.ServerID,
			ServerName: backup.Server.Name, CreatedAt: backup.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, result)
}
