package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/keploy/taskmanager/internal/auth"
	"github.com/keploy/taskmanager/internal/models"
	"github.com/keploy/taskmanager/internal/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserExists         = errors.New("user already exists")
	ErrInactiveUser       = errors.New("user inactive")
)

type AuthService struct {
	users *repository.UserRepo
	jwt   *auth.Manager
}

func NewAuthService(users *repository.UserRepo, jwt *auth.Manager) *AuthService {
	return &AuthService{users: users, jwt: jwt}
}

type RegisterInput struct {
	Username   string
	Email      string
	Password   string
	Role       string // optional, defaults to "user"
	ExternalID *int
}

func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*models.User, error) {
	in.Username = strings.TrimSpace(in.Username)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if in.Username == "" || in.Email == "" || len(in.Password) < 6 {
		return nil, errors.New("username, email and password (>=6 chars) are required")
	}
	if existing, err := s.users.GetByUsername(ctx, in.Username); err == nil && existing != nil {
		return nil, ErrUserExists
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	roleName := in.Role
	if roleName == "" {
		roleName = models.RoleUser
	}
	role, err := s.users.GetRoleByName(ctx, roleName)
	if err != nil {
		return nil, err
	}

	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return nil, err
	}
	u := &models.User{
		ID:           uuid.NewString(),
		Username:     in.Username,
		Email:        in.Email,
		PasswordHash: hash,
		RoleID:       role.ID,
		RoleName:     role.Name,
		ExternalID:   in.ExternalID,
		IsActive:     true,
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	return s.users.GetByID(ctx, u.ID)
}

type LoginResult struct {
	Token     string
	ExpiresAt time.Time
	User      *models.User
}

func (s *AuthService) Login(ctx context.Context, username, password string) (*LoginResult, error) {
	u, err := s.users.GetByUsername(ctx, username)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if !u.IsActive {
		return nil, ErrInactiveUser
	}
	if err := auth.CheckPassword(u.PasswordHash, password); err != nil {
		return nil, ErrInvalidCredentials
	}
	tok, exp, err := s.jwt.Issue(u.ID, u.Username, u.RoleName)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Token: tok, ExpiresAt: exp, User: u}, nil
}
