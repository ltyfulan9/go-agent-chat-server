package model

import "time"

type User struct {
	ID           string    `json:"id" gorm:"primaryKey;size:64"`
	Username     string    `json:"username" gorm:"size:64;uniqueIndex;not null"`
	PasswordHash string    `json:"-" gorm:"size:255;not null"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
