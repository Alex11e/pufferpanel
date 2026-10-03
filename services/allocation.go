package services

import (
	"errors"
	"strings"

	"github.com/pufferpanel/pufferpanel/v3/models"
	"gorm.io/gorm"
)

var ErrNoPortAvailable = errors.New("no free port is available in this node range")
var ErrInvalidPortRange = errors.New("port range must be between 1 and 65535 and start must be <= end")

type Allocation struct{ DB *gorm.DB }

// AllocateNext finds the first unused port in the node's configured range.
// The database unique index is the final guard against concurrent requests.
func (s *Allocation) AllocateNext(node *models.Node, serverID string) (*models.Allocation, error) {
	return s.AllocateRange(node, serverID, node.PortRangeStart, node.PortRangeEnd)
}

// AllocateRange finds the first unused port in an explicit inclusive range.
func (s *Allocation) AllocateRange(node *models.Node, serverID string, start, end uint16) (*models.Allocation, error) {
	return s.AllocateRangeWithPurpose(node, serverID, start, end, "")
}

func (s *Allocation) AllocateRangeWithPurpose(node *models.Node, serverID string, start, end uint16, purpose string) (*models.Allocation, error) {
	return s.allocateRange(node, serverID, start, end, purpose, 0, "tcp,udp")
}

func (s *Allocation) AllocateForward(node *models.Node, serverID string, targetPort uint16, protocols string) (*models.Allocation, error) {
	if targetPort == 0 {
		return nil, errors.New("target port must be between 1 and 65535")
	}
	normalizedProtocols, err := normalizePortProtocols(protocols)
	if err != nil {
		return nil, err
	}
	return s.allocateRange(node, serverID, node.PortRangeStart, node.PortRangeEnd, "forward", targetPort, normalizedProtocols)
}

func (s *Allocation) allocateRange(node *models.Node, serverID string, start, end uint16, purpose string, targetPort uint16, protocols string) (*models.Allocation, error) {
	if start == 0 || end == 0 || start > end {
		return nil, ErrInvalidPortRange
	}
	for candidate := int(start); candidate <= int(end); candidate++ {
		port := uint16(candidate)
		var legacy models.Server
		// LocalNode is virtual: its servers have a NULL node_id rather than the
		// in-memory node ID (0). Include those older primary-port records so an
		// automatic allocation never reuses a LocalNode port already in use.
		legacyQuery := s.DB.Where("port = ?", port)
		if node.IsLocal() {
			legacyQuery = legacyQuery.Where("node_id IS NULL")
		} else {
			legacyQuery = legacyQuery.Where("node_id = ?", node.ID)
		}
		if legacyQuery.First(&legacy).Error == nil {
			continue
		}
		allocation := &models.Allocation{NodeID: node.ID, ServerIdentifier: serverID, Port: port, TargetPort: targetPort, Protocols: protocols, Purpose: purpose}
		err := s.DB.Create(allocation).Error
		if err == nil {
			return allocation, nil
		}
		// A duplicate means a competing request has claimed this port; continue.
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			continue
		}
		var existing models.Allocation
		if s.DB.Where("node_id = ? AND port = ?", node.ID, port).First(&existing).Error == nil {
			continue
		}
		return nil, err
	}
	return nil, ErrNoPortAvailable
}

func (s *Allocation) List(nodeID uint, purpose ...string) ([]models.Allocation, error) {
	var allocations []models.Allocation
	query := s.DB.Where("node_id = ?", nodeID)
	if len(purpose) > 0 {
		filter := strings.TrimSpace(purpose[0])
		if filter != "" {
			query = query.Where("purpose = ?", filter)
		}
	}
	err := query.Order("port").Find(&allocations).Error
	return allocations, err
}

func (s *Allocation) Delete(nodeID, allocationID uint) error {
	result := s.DB.Where("id = ? AND node_id = ?", allocationID, nodeID).Delete(&models.Allocation{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func normalizePortProtocols(protocols string) (string, error) {
	parts := strings.Split(protocols, ",")
	if len(parts) == 0 {
		return "", errors.New("protocols must be tcp, udp, or tcp,udp")
	}

	normalized := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		protocol := strings.ToLower(strings.TrimSpace(part))
		if protocol == "" {
			continue
		}
		if protocol != "tcp" && protocol != "udp" {
			return "", errors.New("protocols must be tcp, udp, or tcp,udp")
		}
		if _, exists := seen[protocol]; exists {
			continue
		}
		seen[protocol] = struct{}{}
		normalized = append(normalized, protocol)
	}
	if len(normalized) == 0 {
		return "", errors.New("protocols must be tcp, udp, or tcp,udp")
	}
	return strings.Join(normalized, ","), nil
}
