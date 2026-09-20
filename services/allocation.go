package services

import (
	"errors"

	"github.com/pufferpanel/pufferpanel/v3/models"
	"gorm.io/gorm"
)

var ErrNoPortAvailable = errors.New("no free port is available in this node range")

type Allocation struct { DB *gorm.DB }

// AllocateNext finds the first unused port in the node's configured range.
// The database unique index is the final guard against concurrent requests.
func (s *Allocation) AllocateNext(node *models.Node, serverID string) (*models.Allocation, error) {
	for candidate := int(node.PortRangeStart); candidate <= int(node.PortRangeEnd); candidate++ {
		port := uint16(candidate)
		var legacy models.Server
		if s.DB.Where("node_id = ? AND port = ?", node.ID, port).First(&legacy).Error == nil { continue }
		allocation := &models.Allocation{NodeID: node.ID, ServerIdentifier: serverID, Port: port, Protocols: "tcp,udp"}
		err := s.DB.Create(allocation).Error
		if err == nil { return allocation, nil }
		// A duplicate means a competing request has claimed this port; continue.
		if errors.Is(err, gorm.ErrDuplicatedKey) { continue }
		var existing models.Allocation
		if s.DB.Where("node_id = ? AND port = ?", node.ID, port).First(&existing).Error == nil { continue }
		return nil, err
	}
	return nil, ErrNoPortAvailable
}

func (s *Allocation) List(nodeID uint) ([]models.Allocation, error) {
	var allocations []models.Allocation
	err := s.DB.Where("node_id = ?", nodeID).Order("port").Find(&allocations).Error
	return allocations, err
}
