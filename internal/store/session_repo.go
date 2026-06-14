package store

import (
	"errors"

	"go-agent-chat-server/internal/model"

	"gorm.io/gorm"
)

type SessionRepo struct {
	db *gorm.DB
}

func NewSessionRepo(db *gorm.DB) *SessionRepo {
	return &SessionRepo{
		db: db,
	}
}

func (r *SessionRepo) Create(session *model.Session) error {
	return r.db.Create(session).Error
}

func (r *SessionRepo) GetByID(id string) (model.Session, error) {
	var session model.Session

	err := r.db.Where("id = ?", id).First(&session).Error
	if err != nil {
		return model.Session{}, err
	}

	return session, nil
}

func (r *SessionRepo) GetByIDAndUserID(id string, userID string) (model.Session, error) {
	var session model.Session

	err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&session).Error
	if err != nil {
		return model.Session{}, err
	}

	return session, nil
}

func (r *SessionRepo) ListByUserID(userID string) ([]model.Session, error) {
	return r.ListByUserIDPage(userID, 1, 1000)
}

func (r *SessionRepo) ListByUserIDPage(userID string, page int, pageSize int) ([]model.Session, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	sessions := make([]model.Session, 0)
	err := r.db.Where("user_id = ?", userID).
		Order("created_at desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&sessions).Error
	if err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *SessionRepo) CountByUserID(userID string) (int64, error) {
	var total int64
	err := r.db.Model(&model.Session{}).Where("user_id = ?", userID).Count(&total).Error
	return total, err
}

func (r *SessionRepo) UpdateTitleByIDAndUserID(id string, userID string, title string) (model.Session, error) {
	result := r.db.Model(&model.Session{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("title", title)
	if result.Error != nil {
		return model.Session{}, result.Error
	}
	if result.RowsAffected == 0 {
		return model.Session{}, gorm.ErrRecordNotFound
	}

	return r.GetByIDAndUserID(id, userID)
}

func (r *SessionRepo) DeleteByIDAndUserIDWithMessages(id string, userID string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var session model.Session
		if err := tx.Where("id = ? AND user_id = ?", id, userID).First(&session).Error; err != nil {
			return err
		}

		if err := tx.Where("session_id = ?", id).Delete(&model.Message{}).Error; err != nil {
			return err
		}

		result := tx.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Session{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("session delete failed")
		}

		return nil
	})
}
