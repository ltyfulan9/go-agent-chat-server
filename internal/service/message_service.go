package service

import (
	"context"
	"errors"

	"go-agent-chat-server/internal/cache"
	"go-agent-chat-server/internal/model"
	"go-agent-chat-server/internal/pkg/idgen"
	"go-agent-chat-server/internal/store"
)

type MessageService struct {
	messageRepo *store.MessageRepo
	sessionRepo *store.SessionRepo
}

func NewMessageService(messageRepo *store.MessageRepo, sessionRepo *store.SessionRepo) *MessageService {
	return &MessageService{
		messageRepo: messageRepo,
		sessionRepo: sessionRepo,
	}
}

func (s *MessageService) CreateMessage(ctx context.Context, userID, sessionID, role, content string) (model.Message, error) {
	if userID == "" {
		return model.Message{}, errors.New("user_id is required")
	}

	if sessionID == "" {
		return model.Message{}, errors.New("session_id is required")
	}

	if content == "" {
		return model.Message{}, errors.New("content is required")
	}

	if role == "" {
		role = "user"
	}

	if _, err := s.sessionRepo.GetByIDAndUserID(sessionID, userID); err != nil {
		return model.Message{}, errors.New("session not found")
	}

	message := model.Message{
		ID:        idgen.NewID(),
		SessionID: sessionID,
		Role:      role,
		Content:   content,
	}

	if err := s.messageRepo.Create(&message); err != nil {
		return model.Message{}, err
	}

	_ = cache.DeleteMessages(ctx, sessionID)

	return message, nil
}

func (s *MessageService) ListMessages(ctx context.Context, userID string, sessionID string) ([]model.Message, error) {
	if userID == "" {
		return []model.Message{}, errors.New("user_id is required")
	}

	if sessionID == "" {
		return []model.Message{}, errors.New("session_id is required")
	}

	if _, err := s.sessionRepo.GetByIDAndUserID(sessionID, userID); err != nil {
		return []model.Message{}, errors.New("session not found")
	}

	cachedMessages, hit, err := cache.GetMessages(ctx, sessionID)
	if err == nil && hit {
		return cachedMessages, nil
	}

	messages, err := s.messageRepo.ListBySessionID(sessionID)
	if err != nil {
		return []model.Message{}, err
	}

	_ = cache.SetMessages(ctx, sessionID, messages)

	return messages, nil
}

func (s *MessageService) ListMessagesPage(ctx context.Context, userID string, sessionID string, page int, pageSize int) ([]model.Message, int64, error) {
	if userID == "" {
		return []model.Message{}, 0, errors.New("user_id is required")
	}

	if sessionID == "" {
		return []model.Message{}, 0, errors.New("session_id is required")
	}

	if _, err := s.sessionRepo.GetByIDAndUserID(sessionID, userID); err != nil {
		return []model.Message{}, 0, errors.New("session not found")
	}

	total, err := s.messageRepo.CountBySessionID(sessionID)
	if err != nil {
		return []model.Message{}, 0, err
	}

	messages, err := s.messageRepo.ListBySessionIDPage(sessionID, page, pageSize)
	if err != nil {
		return []model.Message{}, 0, err
	}

	return messages, total, nil
}

func (s *MessageService) SearchMessages(ctx context.Context, userID string, sessionID string, keyword string, limit int) ([]model.Message, error) {
	if userID == "" {
		return []model.Message{}, errors.New("user_id is required")
	}

	if sessionID == "" {
		return []model.Message{}, errors.New("session_id is required")
	}

	if _, err := s.sessionRepo.GetByIDAndUserID(sessionID, userID); err != nil {
		return []model.Message{}, errors.New("session not found")
	}

	return s.messageRepo.SearchBySessionID(sessionID, keyword, limit)
}
