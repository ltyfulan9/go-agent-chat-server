package service

import (
	"errors"
	"time"

	"go-agent-chat-server/internal/model"
	"go-agent-chat-server/internal/pkg/idgen"
	"go-agent-chat-server/internal/pkg/jwtutil"
	"go-agent-chat-server/internal/store"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	userRepo       *store.UserRepo
	jwtSecret      string
	jwtExpireHours int
}

type AuthResult struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Token    string `json:"token"`
}

func NewAuthService(userRepo *store.UserRepo, jwtSecret string, jwtExpireHours int) *AuthService {
	if jwtExpireHours <= 0 {
		jwtExpireHours = 24
	}

	return &AuthService{
		userRepo:       userRepo,
		jwtSecret:      jwtSecret,
		jwtExpireHours: jwtExpireHours,
	}
}

func (s *AuthService) Register(username string, password string) (AuthResult, error) {
	if username == "" {
		return AuthResult{}, errors.New("username is required")
	}
	if password == "" {
		return AuthResult{}, errors.New("password is required")
	}
	if len(password) < 6 {
		return AuthResult{}, errors.New("password must be at least 6 characters")
	}

	if _, err := s.userRepo.GetByUsername(username); err == nil {
		return AuthResult{}, errors.New("username already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return AuthResult{}, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AuthResult{}, err
	}

	user := model.User{
		ID:           idgen.NewID(),
		Username:     username,
		PasswordHash: string(passwordHash),
	}

	if err := s.userRepo.Create(&user); err != nil {
		return AuthResult{}, err
	}

	return s.buildAuthResult(user)
}

func (s *AuthService) Login(username string, password string) (AuthResult, error) {
	if username == "" || password == "" {
		return AuthResult{}, errors.New("username and password are required")
	}

	user, err := s.userRepo.GetByUsername(username)
	if err != nil {
		return AuthResult{}, errors.New("invalid username or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return AuthResult{}, errors.New("invalid username or password")
	}

	return s.buildAuthResult(user)
}

func (s *AuthService) buildAuthResult(user model.User) (AuthResult, error) {
	token, err := jwtutil.GenerateToken(
		user.ID,
		user.Username,
		s.jwtSecret,
		time.Duration(s.jwtExpireHours)*time.Hour,
	)
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		UserID:   user.ID,
		Username: user.Username,
		Token:    token,
	}, nil
}
