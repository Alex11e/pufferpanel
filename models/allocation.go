package models

import "time"

// Allocation reserves a host port on a node for one server.  An allocation is
// deliberately protocol-agnostic: it reserves both TCP and UDP, preventing a
// second server from accidentally claiming the same numeric port.
type Allocation struct {
	ID               uint      `json:"id"`
	NodeID           uint      `gorm:"not null;uniqueIndex:node_port" json:"nodeId"`
	ServerIdentifier string    `gorm:"not null;index" json:"serverId"`
	Port             uint16    `gorm:"not null;uniqueIndex:node_port" json:"port"`
	Protocols        string    `gorm:"not null;default:tcp,udp" json:"protocols"`
	CreatedAt        time.Time `json:"createdAt"`
}
