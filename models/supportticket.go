package models

import "time"

// SupportTicket keeps customer support conversations in the panel instead of
// requiring users to expose server details in a third-party chat service.
type SupportTicket struct {
	ID               uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID           uint      `json:"userId" gorm:"not null;index"`
	ServerIdentifier string    `json:"serverId,omitempty" gorm:"size:20;index"`
	Subject          string    `json:"subject" gorm:"not null;size:140"`
	Status           string    `json:"status" gorm:"not null;size:20;default:open;index"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type SupportTicketMessage struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	TicketID  uint      `json:"ticketId" gorm:"not null;index"`
	UserID    uint      `json:"userId" gorm:"not null;index"`
	Username  string    `json:"username" gorm:"not null;size:100"`
	Body      string    `json:"body" gorm:"not null;size:4000"`
	CreatedAt time.Time `json:"createdAt"`
}
