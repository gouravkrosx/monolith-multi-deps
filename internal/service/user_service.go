package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/keploy/taskmanager/internal/cache"
	"github.com/keploy/taskmanager/internal/external"
	"github.com/keploy/taskmanager/internal/models"
	"github.com/keploy/taskmanager/internal/repository"
)

type UserService struct {
	users    *repository.UserRepo
	cache    *cache.Cache
	external *external.Client
}

func NewUserService(users *repository.UserRepo, c *cache.Cache, ext *external.Client) *UserService {
	return &UserService{users: users, cache: c, external: ext}
}

func userCacheKey(id string) string { return "user:" + id }

func (s *UserService) GetByID(ctx context.Context, id string) (*models.User, error) {
	var cached models.User
	if err := s.cache.Get(ctx, userCacheKey(id), &cached); err == nil {
		return &cached, nil
	}
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.cache.Set(ctx, userCacheKey(id), u, 5*time.Minute)
	return u, nil
}

func (s *UserService) List(ctx context.Context, limit, offset int) ([]*models.User, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.users.List(ctx, limit, offset)
}

func (s *UserService) UpdateRole(ctx context.Context, userID, roleName string) error {
	role, err := s.users.GetRoleByName(ctx, roleName)
	if err != nil {
		return fmt.Errorf("role lookup: %w", err)
	}
	if err := s.users.UpdateRole(ctx, userID, role.ID); err != nil {
		return err
	}
	_ = s.cache.Del(ctx, userCacheKey(userID))
	return nil
}

func (s *UserService) Deactivate(ctx context.Context, userID string) error {
	if err := s.users.Deactivate(ctx, userID); err != nil {
		return err
	}
	_ = s.cache.Del(ctx, userCacheKey(userID))
	return nil
}

// EnrichedUser combines local user with data from the external API.
type EnrichedUser struct {
	*models.User
	External *external.ExternalUser `json:"external,omitempty"`
}

func (s *UserService) Enrich(ctx context.Context, id string) (*EnrichedUser, error) {
	u, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	out := &EnrichedUser{User: u}
	if u.ExternalID == nil {
		return out, nil
	}
	cacheKey := fmt.Sprintf("ext:user:%d", *u.ExternalID)
	var ext external.ExternalUser
	if err := s.cache.Get(ctx, cacheKey, &ext); err == nil {
		out.External = &ext
		return out, nil
	}
	fetched, err := s.external.GetUser(ctx, *u.ExternalID)
	if err != nil {
		// non-fatal: log via caller; just skip enrichment
		return out, nil
	}
	_ = s.cache.Set(ctx, cacheKey, fetched, 30*time.Minute)
	out.External = fetched
	return out, nil
}

func IsNotFound(err error) bool { return errors.Is(err, repository.ErrNotFound) }
