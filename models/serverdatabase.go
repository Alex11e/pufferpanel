package models

import "time"

type ServerDatabase struct {
	ID                       uint              `gorm:"primaryKey" json:"id"`
	ServerIdentifier         string            `gorm:"not null;uniqueIndex;size:20" json:"serverId"`
	DatabaseServerIdentifier *string           `gorm:"uniqueIndex;size:20" json:"databaseServerId,omitempty"`
	Engine                   string            `gorm:"not null;size:16" json:"engine"`
	AccessMode               string            `gorm:"not null;size:16" json:"accessMode"`
	Host                     string            `gorm:"not null;size:253" json:"host"`
	Port                     uint16            `gorm:"not null" json:"port"`
	DatabaseName             string            `gorm:"not null;size:32" json:"databaseName"`
	Username                 string            `gorm:"not null;size:32" json:"username"`
	VariableMapping          map[string]string `gorm:"serializer:json;type:text" json:"variableMapping"`
	CreatedAt                time.Time         `json:"createdAt"`
	UpdatedAt                time.Time         `json:"updatedAt"`
}
