package services

import (
	"net/http"
	"time"

	"github.com/pufferpanel/pufferpanel/v3/database"
	"github.com/pufferpanel/pufferpanel/v3/logging"
	"github.com/pufferpanel/pufferpanel/v3/models"
	"github.com/pufferpanel/pufferpanel/v3/utils"
)

// StartExpiryEnforcement stops servers past their expiry date every 10 minutes.
// Like automatic backups, each panel instance runs its own loop.
func StartExpiryEnforcement() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			StopExpiredServers()
		}
	}()
}

func StopExpiredServers() {
	db, err := database.GetConnection()
	if err != nil {
		logging.Error.Printf("expiry enforcement: %s", err)
		return
	}
	var servers []*models.Server
	if err = db.Preload("Node").Where("expires_at IS NOT NULL AND expires_at <= ?", time.Now()).Find(&servers).Error; err != nil {
		logging.Error.Printf("expiry enforcement: %s", err)
		return
	}
	ns := &Node{DB: db}
	for _, server := range servers {
		response, err := ns.CallNode(&server.Node, http.MethodPost, "/daemon/server/"+server.Identifier+"/stop", nil, nil)
		utils.CloseResponse(response)
		if err != nil {
			logging.Error.Printf("expiry enforcement: stopping %s failed: %s", server.Identifier, err)
		}
	}
}
