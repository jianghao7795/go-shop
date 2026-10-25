package model

import "time"

const (
	SupportModeBot     = "bot"
	SupportModeWaiting = "waiting"
	SupportModeHuman   = "human"
)

// SupportTicket is a private conversation between a customer and support staff.
type SupportTicket struct {
	ID      uint   `json:"id" gorm:"primaryKey"`
	UserID  string `json:"userId" gorm:"index;size:64"`
	Subject string `json:"subject" gorm:"size:100"`
	Status  string `json:"status" gorm:"size:20"`
	// Existing conversations were handled by staff. New conversations explicitly start in bot mode.
	Mode             string     `json:"mode" gorm:"size:16;not null;default:human;index"`
	HumanRequestedAt *time.Time `json:"humanRequestedAt"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

func (SupportTicket) TableName() string { return "shop_support_tickets" }

type SupportMessage struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	TicketID  uint      `json:"ticketId" gorm:"index"`
	Staff     bool      `json:"staff"`
	Bot       bool      `json:"bot" gorm:"not null;default:false"`
	Content   string    `json:"content" gorm:"size:2000"`
	CreatedAt time.Time `json:"createdAt"`
}

func (SupportMessage) TableName() string { return "shop_support_messages" }
