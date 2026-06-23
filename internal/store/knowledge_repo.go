package store

import (
	"go-agent-chat-server/internal/model"

	"gorm.io/gorm"
)

type KnowledgeRepo struct {
	db *gorm.DB
}

func NewKnowledgeRepo(db *gorm.DB) *KnowledgeRepo {
	return &KnowledgeRepo{db: db}
}

func (r *KnowledgeRepo) Create(doc *model.KnowledgeDoc) error {
	return r.db.Create(doc).Error
}

func (r *KnowledgeRepo) ListByUserID(userID string, page int, pageSize int) ([]model.KnowledgeDoc, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	docs := make([]model.KnowledgeDoc, 0)
	err := r.db.Where("user_id = ?", userID).
		Order("created_at desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&docs).Error
	return docs, err
}

func (r *KnowledgeRepo) CountByUserID(userID string) (int64, error) {
	var total int64
	err := r.db.Model(&model.KnowledgeDoc{}).Where("user_id = ?", userID).Count(&total).Error
	return total, err
}

func (r *KnowledgeRepo) GetByIDAndUserID(id string, userID string) (model.KnowledgeDoc, error) {
	var doc model.KnowledgeDoc
	err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&doc).Error
	if err != nil {
		return model.KnowledgeDoc{}, err
	}
	return doc, nil
}

func (r *KnowledgeRepo) DeleteByIDAndUserID(id string, userID string) error {
	result := r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.KnowledgeDoc{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *KnowledgeRepo) Search(userID string, keyword string, limit int) ([]model.KnowledgeDoc, error) {
	if limit <= 0 {
		limit = 5
	}

	docs := make([]model.KnowledgeDoc, 0)
	query := r.db.Where("user_id = ?", userID)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("title LIKE ? OR content LIKE ?", like, like)
	}

	err := query.Order("created_at desc").Limit(limit).Find(&docs).Error
	return docs, err
}
