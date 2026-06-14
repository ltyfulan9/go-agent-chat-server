package model

import "time"

type KnowledgeDoc struct {
	ID        string    `json:"id" gorm:"primaryKey;size:64"`
	UserID    string    `json:"user_id" gorm:"size:64;not null;index:idx_knowledge_user_created,priority:1"`
	Title     string    `json:"title" gorm:"size:255;not null"`
	Content   string    `json:"content" gorm:"type:longtext;not null"`
	CreatedAt time.Time `json:"created_at" gorm:"index:idx_knowledge_user_created,priority:2"`
	UpdatedAt time.Time `json:"updated_at"`
}
