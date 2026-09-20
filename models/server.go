package models

import (
	"github.com/pufferpanel/pufferpanel/v3"
	"gopkg.in/go-playground/validator.v9"
	"gorm.io/gorm"
	"time"
)

type Server struct {
	Name       string `gorm:"column:name;not null;size:40" json:"-" validate:"required,printascii"`
	Identifier string `gorm:"column:identifier;primaryKey;size:20" json:"-" validate:"required,printascii"`

	RawNodeID *uint `gorm:"column:node_id;index;->;<-:create" json:"-" validate:"-"`
	NodeID    uint  `gorm:"-" json:"-" validate:"-"`
	Node      Node  `gorm:"foreignKey:RawNodeID;->;<-:create" json:"-" validate:"-"`

	IP   string `gorm:"" json:"-" validate:"omitempty,ip|fqdn"`
	Port uint16 `gorm:"" json:"-" validate:"omitempty"`
	Subdomain string `gorm:"column:subdomain;size:253;uniqueIndex" json:"-" validate:"omitempty,fqdn"`
	Allocations []Allocation `gorm:"foreignKey:ServerIdentifier;references:Identifier" json:"-"`
	AutoBackupEnabled bool `gorm:"column:auto_backup_enabled;not null;default:false" json:"-"`
	AutoBackupRetention uint `gorm:"column:auto_backup_retention;not null;default:24" json:"-" validate:"max=168"`
	Notes string `gorm:"column:notes;size:2000" json:"-"`
	Tags string `gorm:"column:tags;size:255" json:"-"`

	Type string `gorm:"NOT NULL;default='generic'" json:"-" validate:"required,printascii"`
	Icon string `gorm:"" json:"-"`

	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

func (s *Server) IsValid() (err error) {
	err = validator.New().Struct(s)
	if err != nil {
		err = pufferpanel.GenerateValidationMessage(err)
	}
	return
}

func (s *Server) BeforeSave(*gorm.DB) (err error) {
	err = s.IsValid()
	if s.NodeID == 0 || s.Node.IsLocal() {
		s.RawNodeID = nil
	} else {
		s.RawNodeID = &s.NodeID
	}
	return
}

func (s *Server) AfterFind(*gorm.DB) (err error) {
	if s.RawNodeID == nil || *s.RawNodeID == LocalNode.ID {
		s.Node = *LocalNode
	}
	return
}
