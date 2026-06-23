package model

import "time"

type Session struct {
	ID        string    `json:"id" gorm:"primaryKey;size:64"`
	UserID    string    `json:"user_id" gorm:"size:64;not null;index:idx_user_created,priority:1"`
	Title     string    `json:"title" gorm:"size:255;not null"`
	CreatedAt time.Time `json:"created_at" gorm:"index:idx_user_created,priority:2"`
	UpdatedAt time.Time `json:"updated_at"`
}
