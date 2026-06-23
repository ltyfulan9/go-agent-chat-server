package service

import (
	"context"
	"errors"
	"strings"

	"go-agent-chat-server/internal/cache"
	"go-agent-chat-server/internal/model"
	"go-agent-chat-server/internal/pkg/idgen"
	"go-agent-chat-server/internal/store"
)

type SessionService struct {
	sessionRepo *store.SessionRepo
}

func NewSessionService(sessionRepo *store.SessionRepo) *SessionService {
	return &SessionService{
		sessionRepo: sessionRepo,
	}
}

func (s *SessionService) CreateSession(userID string, title string) (model.Session, error) {
	if userID == "" {
		return model.Session{}, errors.New("user_id is required")
	}

	title = strings.TrimSpace(title)
	if title == "" {
		title = "New Chat"
	}

	session := model.Session{
		ID:     idgen.NewID(),
		UserID: userID,
		Title:  title,
	}

	if err := s.sessionRepo.Create(&session); err != nil {
		return model.Session{}, err
	}

	return session, nil
}

func (s *SessionService) GetSession(userID string, id string) (model.Session, error) {
	if userID == "" {
		return model.Session{}, errors.New("user_id is required")
	}
	if id == "" {
		return model.Session{}, errors.New("session_id is required")
	}
	return s.sessionRepo.GetByIDAndUserID(id, userID)
}

func (s *SessionService) ListSessions(userID string, page int, pageSize int) ([]model.Session, int64, error) {
	if userID == "" {
		return []model.Session{}, 0, errors.New("user_id is required")
	}

	total, err := s.sessionRepo.CountByUserID(userID)
	if err != nil {
		return []model.Session{}, 0, err
	}

	sessions, err := s.sessionRepo.ListByUserIDPage(userID, page, pageSize)
	if err != nil {
		return []model.Session{}, 0, err
	}

	return sessions, total, nil
}

func (s *SessionService) UpdateSessionTitle(userID string, id string, title string) (model.Session, error) {
	if userID == "" {
		return model.Session{}, errors.New("user_id is required")
	}
	if id == "" {
		return model.Session{}, errors.New("session_id is required")
	}

	title = strings.TrimSpace(title)
	if title == "" {
		return model.Session{}, errors.New("title is required")
	}

	return s.sessionRepo.UpdateTitleByIDAndUserID(id, userID, title)
}

func (s *SessionService) DeleteSession(ctx context.Context, userID string, id string) error {
	if userID == "" {
		return errors.New("user_id is required")
	}
	if id == "" {
		return errors.New("session_id is required")
	}

	if err := s.sessionRepo.DeleteByIDAndUserIDWithMessages(id, userID); err != nil {
		return err
	}

	_ = cache.DeleteMessages(ctx, id)
	return nil
}
