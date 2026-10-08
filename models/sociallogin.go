package models

import "time"

type SocialProvider struct {
	ID                    uint   `gorm:"primaryKey;autoIncrement"`
	Key                   string `gorm:"not null;size:40;uniqueIndex"`
	Name                  string `gorm:"not null;size:100"`
	Kind                  string `gorm:"not null;size:20"`
	ClientID              string `gorm:"not null;size:255"`
	ClientSecretEncrypted string `gorm:"not null;size:2048" json:"-"`
	IssuerURL             string `gorm:"size:500"`
	Enabled               bool   `gorm:"not null;default:false"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type SocialConnection struct {
	ID          uint   `gorm:"primaryKey;autoIncrement"`
	UserID      uint   `gorm:"not null;uniqueIndex:idx_social_connection_user_provider,priority:1;index"`
	ProviderID  uint   `gorm:"not null;uniqueIndex:idx_social_connection_user_provider,priority:2;uniqueIndex:idx_social_connection_provider_subject,priority:1"`
	Subject     string `gorm:"not null;size:255;uniqueIndex:idx_social_connection_provider_subject,priority:2"`
	Email       string `gorm:"size:255"`
	DisplayName string `gorm:"size:255"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	User        User           `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
	Provider    SocialProvider `gorm:"foreignKey:ProviderID;references:ID;constraint:OnDelete:CASCADE"`
}
