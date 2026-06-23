package store

import (
	"time"

	"go-agent-chat-server/internal/model"

	"gorm.io/gorm"
)

type ChatJobRepo struct {
	db *gorm.DB
}

func NewChatJobRepo(db *gorm.DB) *ChatJobRepo {
	return &ChatJobRepo{db: db}
}

func (r *ChatJobRepo) Create(job *model.ChatJob) error {
	return r.db.Create(job).Error
}

func (r *ChatJobRepo) GetByID(id string) (model.ChatJob, error) {
	var job model.ChatJob
	err := r.db.Where("id = ?", id).First(&job).Error
	if err != nil {
		return model.ChatJob{}, err
	}
	return job, nil
}

func (r *ChatJobRepo) GetByIDAndUserID(id, userID string) (model.ChatJob, error) {
	var job model.ChatJob
	err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&job).Error
	if err != nil {
		return model.ChatJob{}, err
	}
	return job, nil
}

func (r *ChatJobRepo) MarkRunning(id string) error {
	now := time.Now()
	return r.db.Model(&model.ChatJob{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        model.ChatJobStatusRunning,
			"started_at":    &now,
			"error_message": "",
		}).Error
}

func (r *ChatJobRepo) MarkSuccess(id, answer, assistantMessageID string) error {
	now := time.Now()
	return r.db.Model(&model.ChatJob{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":               model.ChatJobStatusSuccess,
			"answer_text":          answer,
			"assistant_message_id": assistantMessageID,
			"finished_at":          &now,
			"error_message":        "",
		}).Error
}

func (r *ChatJobRepo) MarkFailed(id, errMsg string, retryCount int) error {
	now := time.Now()
	return r.db.Model(&model.ChatJob{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        model.ChatJobStatusFailed,
			"error_message": errMsg,
			"retry_count":   retryCount,
			"finished_at":   &now,
		}).Error
}

func (r *ChatJobRepo) MarkPendingForRetry(id, errMsg string, retryCount int) error {
	return r.db.Model(&model.ChatJob{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        model.ChatJobStatusPending,
			"error_message": errMsg,
			"retry_count":   retryCount,
		}).Error
}
