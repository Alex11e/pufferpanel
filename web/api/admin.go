package api

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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
	OpenTickets          int64 `json:"openTickets"`
	NewUsers7Days        int64 `json:"newUsers7Days"`
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
	g.POST("/servers/action", middleware.RequiresPermission(scopes.ScopeAdmin), runBulkServerAction)
	g.GET("/activity/export", middleware.RequiresPermission(scopes.ScopeAdmin), exportAdminActivity)
	g.GET("/expiring", middleware.RequiresPermission(scopes.ScopeAdmin), getAdminExpiring)
	g.PUT("/servers/:id/expiry", middleware.RequiresPermission(scopes.ScopeAdmin), setServerExpiry)
	g.OPTIONS("/expiring", response.CreateOptions("GET"))
	g.OPTIONS("/servers/:id/expiry", response.CreateOptions("PUT"))
	g.PUT("/servers/:id/backup-limit", middleware.RequiresPermission(scopes.ScopeAdmin), setServerBackupLimit)
	g.GET("/servers/:id/limits", middleware.RequiresPermission(scopes.ScopeAdmin), getServerLimits)
	g.OPTIONS("/servers/:id/limits", response.CreateOptions("GET"))
	g.OPTIONS("/servers/:id/backup-limit", response.CreateOptions("PUT"))
	g.OPTIONS("/overview", response.CreateOptions("GET"))
	g.OPTIONS("/ports", response.CreateOptions("GET"))
	g.OPTIONS("/backups", response.CreateOptions("GET"))
	g.OPTIONS("/servers/action", response.CreateOptions("POST"))
	g.OPTIONS("/activity/export", response.CreateOptions("GET"))
}

// exportAdminActivity gives admins a portable audit report without granting
// database access. It is capped to keep a browser request bounded.
func exportAdminActivity(c *gin.Context) {
	var records []models.Activity
	if err := middleware.GetDatabase(c).Order("created_at DESC").Limit(10000).Find(&records).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=pufferpanel-activity.csv")
	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{"id", "time_utc", "username", "action", "server_id", "details", "ip_address"})
	for _, record := range records {
		_ = writer.Write([]string{fmt.Sprint(record.ID), record.CreatedAt.UTC().Format(time.RFC3339), record.Username, record.Action, record.ServerIdentifier, record.Details, record.IPAddress})
	}
	writer.Flush()
}

type bulkServerActionRequest struct {
	ServerIDs []string `json:"serverIds"`
	Action    string   `json:"action"`
}

type bulkServerActionResult struct {
	ServerID string `json:"serverId"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}

// runBulkServerAction intentionally accepts only non-destructive lifecycle
// actions. The UI asks the administrator for confirmation before calling it.
func runBulkServerAction(c *gin.Context) {
	var request bulkServerActionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	request.Action = strings.ToLower(strings.TrimSpace(request.Action))
	if request.Action != "start" && request.Action != "restart" && request.Action != "stop" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "action must be start, restart, or stop"})
		return
	}
	if len(request.ServerIDs) == 0 || len(request.ServerIDs) > 25 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "select between 1 and 25 servers"})
		return
	}

	serverService := &services.Server{DB: middleware.GetDatabase(c)}
	nodeService := &services.Node{DB: middleware.GetDatabase(c)}
	results := make([]bulkServerActionResult, 0, len(request.ServerIDs))
	seen := make(map[string]struct{}, len(request.ServerIDs))
	for _, id := range request.ServerIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		server, err := serverService.Get(id)
		if err != nil {
			results = append(results, bulkServerActionResult{ServerID: id, Error: "server not found"})
			continue
		}
		res, err := nodeService.CallNode(&server.Node, http.MethodPost, "/daemon/server/"+url.PathEscape(server.Identifier)+"/"+request.Action, nil, nil)
		if err != nil {
			results = append(results, bulkServerActionResult{ServerID: id, Error: err.Error()})
			continue
		}
		if res.Body != nil {
			_ = res.Body.Close()
		}
		if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
			results = append(results, bulkServerActionResult{ServerID: id, Error: fmt.Sprintf("node returned HTTP %d", res.StatusCode)})
			continue
		}
		results = append(results, bulkServerActionResult{ServerID: id, Success: true})
	}
	c.JSON(http.StatusOK, results)
}

// blockExpiredServer stops non-admins from starting a server past its expiry date.
func blockExpiredServer(c *gin.Context) {
	server, ok := c.MustGet("server").(*models.Server)
	if !ok || server.ExpiresAt == nil || server.ExpiresAt.After(time.Now()) {
		return
	}
	user, ok := c.MustGet("user").(*models.User)
	if ok {
		p, err := (&services.Permission{DB: middleware.GetDatabase(c)}).GetForUserAndServer(user.ID, "")
		if err == nil && p != nil && scopes.ContainsScope(p.Scopes, scopes.ScopeAdmin) {
			return
		}
	}
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "server expired"})
}

func getServerLimits(c *gin.Context) {
	var server models.Server
	if err := middleware.GetDatabase(c).Where("identifier = ?", c.Param("id")).First(&server).Error; err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, gin.H{"expiresAt": server.ExpiresAt, "backupLimit": server.BackupLimit})
}

// setServerBackupLimit caps total backups per server; 0 removes the limit.
func setServerBackupLimit(c *gin.Context) {
	var body struct {
		Limit uint `json:"limit"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Limit > 1000 {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	res := middleware.GetDatabase(c).Model(&models.Server{}).Where("identifier = ?", c.Param("id")).UpdateColumn("backup_limit", body.Limit)
	if res.Error != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if res.RowsAffected == 0 {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Status(http.StatusNoContent)
}

type adminExpiryView struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

func getAdminExpiring(c *gin.Context) {
	var servers []models.Server
	if err := middleware.GetDatabase(c).Where("expires_at IS NOT NULL").Order("expires_at ASC").Limit(100).Find(&servers).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	result := make([]adminExpiryView, 0, len(servers))
	for _, s := range servers {
		result = append(result, adminExpiryView{ID: s.Identifier, Name: s.Name, ExpiresAt: s.ExpiresAt})
	}
	c.JSON(http.StatusOK, result)
}

// setServerExpiry sets or clears (null) the expiry date. Expiry is informational; it does not stop the server.
func setServerExpiry(c *gin.Context) {
	var body struct {
		ExpiresAt *time.Time `json:"expiresAt"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	// UpdateColumn skips model hooks, which would validate an empty struct.
	res := middleware.GetDatabase(c).Model(&models.Server{}).Where("identifier = ?", c.Param("id")).UpdateColumn("expires_at", body.ExpiresAt)
	if res.Error != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if res.RowsAffected == 0 {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Status(http.StatusNoContent)
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
	if err := db.Model(&models.SupportTicket{}).Where("status = ?", "open").Count(&overview.OpenTickets).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if err := db.Model(&models.User{}).Where("created_at >= ?", time.Now().AddDate(0, 0, -7)).Count(&overview.NewUsers7Days).Error; err != nil {
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
