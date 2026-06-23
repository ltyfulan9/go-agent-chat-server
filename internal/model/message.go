package model

import "time"

type Message struct {
	ID        string    `json:"id" gorm:"primaryKey;size:64"`
	SessionID string    `json:"session_id" gorm:"size:64;not null;index:idx_session_created,priority:1"`
	Role      string    `json:"role" gorm:"size:32;not null"`
	Content   string    `json:"content" gorm:"type:text;not null"`
	CreatedAt time.Time `json:"created_at" gorm:"index:idx_session_created,priority:2"`
	UpdatedAt time.Time `json:"updated_at"`
}
