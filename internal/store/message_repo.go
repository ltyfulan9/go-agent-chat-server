package store

import (
	"go-agent-chat-server/internal/model"

	"gorm.io/gorm"
)

type MessageRepo struct {
	db *gorm.DB
}

func NewMessageRepo(db *gorm.DB) *MessageRepo {
	return &MessageRepo{
		db: db,
	}
}

func (r *MessageRepo) Create(message *model.Message) error {
	return r.db.Create(message).Error
}

func (r *MessageRepo) ListBySessionID(sessionID string) ([]model.Message, error) {
	messages := make([]model.Message, 0)

	err := r.db.
		Where("session_id = ?", sessionID).
		Order("created_at asc").
		Find(&messages).Error

	if err != nil {
		return nil, err
	}

	return messages, nil
}

func (r *MessageRepo) ListBySessionIDPage(sessionID string, page int, pageSize int) ([]model.Message, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 30
	}

	messages := make([]model.Message, 0)
	err := r.db.
		Where("session_id = ?", sessionID).
		Order("created_at asc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&messages).Error
	if err != nil {
		return nil, err
	}

	return messages, nil
}

func (r *MessageRepo) CountBySessionID(sessionID string) (int64, error) {
	var total int64
	err := r.db.Model(&model.Message{}).Where("session_id = ?", sessionID).Count(&total).Error
	return total, err
}

func (r *MessageRepo) SearchBySessionID(sessionID string, keyword string, limit int) ([]model.Message, error) {
	if limit <= 0 {
		limit = 5
	}

	messages := make([]model.Message, 0)
	query := r.db.Where("session_id = ?", sessionID)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("content LIKE ?", like)
	}

	err := query.Order("created_at desc").Limit(limit).Find(&messages).Error
	if err != nil {
		return nil, err
	}

	return messages, nil
}
