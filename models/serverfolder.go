package models

import "time"

// ServerFolder is a private, per-user way to organize a large server list.
// It does not change server ownership, permissions, or node configuration.
type ServerFolder struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID    uint      `json:"userId" gorm:"not null;index"`
	Name      string    `json:"name" gorm:"not null;size:60"`
	Color     string    `json:"color" gorm:"not null;size:7;default:#4f7cff"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ServerFolderItem struct {
	ID               uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	FolderID         uint   `json:"folderId" gorm:"not null;index"`
	UserID           uint   `json:"userId" gorm:"not null;uniqueIndex:idx_user_folder_server"`
	ServerIdentifier string `json:"serverId" gorm:"not null;size:20;uniqueIndex:idx_user_folder_server"`
}
