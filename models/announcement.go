package models

import "time"

// Announcement is a panel-wide message maintained by administrators. It never
// contains server credentials or node configuration, so it is safe to return
// to every signed-in panel user.
type Announcement struct {
	ID        uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	Title     string     `json:"title" gorm:"not null;size:140"`
	Message   string     `json:"message" gorm:"not null;size:4000"`
	Level     string     `json:"level" gorm:"not null;size:20;default:info"`
	Active    bool       `json:"active" gorm:"not null;default:true"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}
