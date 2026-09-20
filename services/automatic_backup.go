package services

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/pufferpanel/pufferpanel/v3"
	"github.com/pufferpanel/pufferpanel/v3/database"
	"github.com/pufferpanel/pufferpanel/v3/logging"
	"github.com/pufferpanel/pufferpanel/v3/models"
	"github.com/pufferpanel/pufferpanel/v3/utils"
	"gorm.io/gorm"
)

// StartAutomaticBackups runs enabled server backups once each hour. Each panel
// instance owns its own scheduler, so deployments should run a single panel.
func StartAutomaticBackups() {
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			RunAutomaticBackups()
		}
	}()
}

func RunAutomaticBackups() {
	db, err := database.GetConnection()
	if err != nil { logging.Error.Printf("automatic backups: %s", err); return }
	var servers []*models.Server
	if err = db.Preload("Node").Where("auto_backup_enabled = ?", true).Find(&servers).Error; err != nil {
		logging.Error.Printf("automatic backups: %s", err)
		return
	}
	for _, server := range servers {
		runAutomaticBackup(db, server)
	}
}

func runAutomaticBackup(db *gorm.DB, server *models.Server) {
	ns := &Node{DB: db}
	response, err := ns.CallNode(&server.Node, http.MethodPost, "/daemon/server/"+server.Identifier+"/backup/create", nil, nil)
	defer utils.CloseResponse(response)
	if err != nil || response == nil {
		logging.Error.Printf("automatic backup for %s failed: %v", server.Identifier, err)
		return
	}
	if response.StatusCode != http.StatusOK {
		logging.Error.Printf("automatic backup for %s failed with status %d", server.Identifier, response.StatusCode)
		return
	}
	var result pufferpanel.ServerBackupResponse
	if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
		logging.Error.Printf("automatic backup for %s could not be decoded: %s", server.Identifier, err)
		return
	}
	backup := &models.Backup{ServerID: server.Identifier, FileName: result.BackupFileName, Name: "Automatic " + time.Now().Format("2006-01-02 15:04")}
	if err = (&Backup{DB: db}).Create(backup); err != nil {
		logging.Error.Printf("automatic backup for %s could not be saved: %s", server.Identifier, err)
		return
	}
	pruneAutomaticBackups(db, server)
}

func pruneAutomaticBackups(db *gorm.DB, server *models.Server) {
	retention := server.AutoBackupRetention
	if retention == 0 {
		retention = 24
	}
	var backups []models.Backup
	if err := db.Where("server_id = ? AND name LIKE ?", server.Identifier, "Automatic %").Order("created_at DESC").Find(&backups).Error; err != nil {
		logging.Error.Printf("automatic backup pruning for %s failed: %s", server.Identifier, err)
		return
	}
	ns := &Node{DB: db}
	for index := retention; index < uint(len(backups)); index++ {
		backup := backups[index]
		response, err := ns.CallNode(&server.Node, http.MethodDelete, "/daemon/server/"+server.Identifier+"/backup?fileName="+backup.FileName, nil, nil)
		utils.CloseResponse(response)
		if err != nil || response == nil || response.StatusCode >= http.StatusBadRequest {
			logging.Error.Printf("automatic backup pruning for %s failed: %v", server.Identifier, err)
			continue
		}
		if err = db.Delete(&backup).Error; err != nil {
			logging.Error.Printf("automatic backup record pruning for %s failed: %s", server.Identifier, err)
		}
	}
}
