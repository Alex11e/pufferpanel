package models

import (
	"errors"
	"github.com/gofrs/uuid/v5"
	"github.com/pufferpanel/pufferpanel/v3"
	"gopkg.in/go-playground/validator.v9"
	"gorm.io/gorm"
	"strings"
	"time"
)

type Node struct {
	ID          uint   `json:"-"`
	Name        string `gorm:"column:name;not null;size:100;uniqueIndex;unique" json:"-" validate:"required,printascii"`
	PublicHost  string `gorm:"column:public_host;not null;size:100" json:"-" validate:"required,ip|fqdn|hostname"`
	PrivateHost string `gorm:"column:private_host;not null;size:100" json:"-" validate:"required,ip|fqdn|hostname"`
	PublicPort  uint16 `gorm:"column:public_port;not null;default:8080" json:"-" validate:"required,min=1,max=65535,nefield=SFTPPort"`
	PrivatePort uint16 `gorm:"column:private_port;not null;default:8080" json:"-" validate:"required,min=1,max=65535,nefield=SFTPPort"`
	SFTPPort    uint16 `gorm:"column:sftp_port;not null;default:5657" json:"-" validate:"required,min=1,max=65535,nefield=PublicPort,nefield=PrivatePort"`
	PortRangeStart uint16 `gorm:"column:port_range_start;not null;default:1000" json:"-" validate:"min=1,max=65535"`
	PortRangeEnd uint16 `gorm:"column:port_range_end;not null;default:8000" json:"-" validate:"min=1,max=65535,gtefield=PortRangeStart"`
	FirewallEnabled bool `gorm:"column:firewall_enabled;not null;default:false" json:"-"`
	SubdomainBase string `gorm:"column:subdomain_base;size:253" json:"-" validate:"omitempty,fqdn"`

	Secret string `gorm:"column:secret;not null;size=36" json:"-" validate:"required"`

	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`

	Local bool `gorm:"-" json:"-"`
}

func (n *Node) IsValid() (err error) {
	err = validator.New().Struct(n)
	if err != nil {
		err = pufferpanel.GenerateValidationMessage(err)
	}
	return
}

func (n *Node) BeforeSave(*gorm.DB) (err error) {
	// Existing installations and programmatic node creation may not provide the
	// optional allocation range. Keep the model valid while preserving the
	// documented default range used by the database schema.
	if n.PortRangeStart == 0 {
		n.PortRangeStart = 1000
	}
	if n.PortRangeEnd == 0 {
		n.PortRangeEnd = 8000
	}
	err = n.IsValid()
	if err != nil {
		return err
	}
	if n.IsLocal() {
		return errors.New("cannot save local node")
	}
	return
}

func (n *Node) IsLocal() bool {
	return n.Local
}

var LocalNode = &Node{
	ID:          0,
	Name:        "LocalNode",
	PublicHost:  "127.0.0.1",
	PrivateHost: "127.0.0.1",
	PublicPort:  8080,
	PrivatePort: 8080,
	SFTPPort:    5657,
	PortRangeStart: 1000,
	PortRangeEnd: 8000,
	Local:       true,
}

func init() {
	u, err := uuid.NewV4()
	if err != nil {
		panic(err)
	}
	LocalNode.Secret = strings.Replace(u.String(), "-", "", -1)
}
