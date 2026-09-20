package models

import "time"

// Activity is an immutable audit record for user-facing server operations.
type Activity struct {
	ID               uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerIdentifier string    `json:"serverId" gorm:"index;size:20"`
	Username         string    `json:"username" gorm:"size:100"`
	Action           string    `json:"action" gorm:"size:100"`
	Details          string    `json:"details,omitempty" gorm:"size:255"`
	IPAddress        string    `json:"ipAddress,omitempty" gorm:"size:64"`
	CreatedAt        time.Time `json:"createdAt"`
}
