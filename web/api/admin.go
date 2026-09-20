package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/models"
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

func registerAdmin(g *gin.RouterGroup) {
	g.GET("/overview", middleware.RequiresPermission(scopes.ScopeAdmin), getAdminOverview)
	g.GET("/ports", middleware.RequiresPermission(scopes.ScopeAdmin), getAdminPortUsage)
	g.OPTIONS("/overview", responseOptions("GET"))
	g.OPTIONS("/ports", responseOptions("GET"))
}

// responseOptions keeps this small admin-only group independent from the
// route registration helpers used by the older API groups.
func responseOptions(methods ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Allow", "OPTIONS, "+methods[0])
		c.Status(http.StatusNoContent)
	}
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
