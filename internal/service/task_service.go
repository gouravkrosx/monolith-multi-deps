package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/keploy/taskmanager/internal/cache"
	"github.com/keploy/taskmanager/internal/models"
	"github.com/keploy/taskmanager/internal/repository"
)

var ErrForbidden = errors.New("forbidden")

type TaskService struct {
	tasks *repository.TaskRepo
	users *repository.UserRepo
	cache *cache.Cache
}

func NewTaskService(tasks *repository.TaskRepo, users *repository.UserRepo, c *cache.Cache) *TaskService {
	return &TaskService{tasks: tasks, users: users, cache: c}
}

func taskCacheKey(id string) string { return "task:" + id }

type CreateTaskInput struct {
	Title       string
	Description string
	Priority    string
	DueDate     *time.Time
	OwnerID     string
}

func (s *TaskService) Create(ctx context.Context, in CreateTaskInput) (*models.Task, error) {
	if in.Title == "" {
		return nil, errors.New("title required")
	}
	if in.Priority == "" {
		in.Priority = models.PriorityMedium
	}
	if !models.ValidPriority(in.Priority) {
		return nil, fmt.Errorf("invalid priority: %s", in.Priority)
	}
	t := &models.Task{
		ID:          uuid.NewString(),
		Title:       in.Title,
		Description: in.Description,
		Status:      models.StatusPending,
		Priority:    in.Priority,
		DueDate:     in.DueDate,
		OwnerID:     in.OwnerID,
	}
	if err := s.tasks.Create(ctx, t); err != nil {
		return nil, err
	}
	return s.tasks.GetByID(ctx, t.ID)
}

func (s *TaskService) Get(ctx context.Context, id string) (*models.Task, error) {
	var cached models.Task
	if err := s.cache.Get(ctx, taskCacheKey(id), &cached); err == nil {
		return &cached, nil
	}
	t, err := s.tasks.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.cache.Set(ctx, taskCacheKey(id), t, 2*time.Minute)
	return t, nil
}

func (s *TaskService) List(ctx context.Context, f repository.TaskFilter) ([]*models.Task, error) {
	return s.tasks.List(ctx, f)
}

type UpdateTaskInput struct {
	Title        *string
	Description  *string
	Status       *string
	Priority     *string
	DueDate      *time.Time
	ClearDueDate bool
}

// authorizeTaskMutation: admin can edit anything; user can edit if owner or assignee.
func (s *TaskService) authorizeTaskMutation(ctx context.Context, taskID, callerID, callerRole string) error {
	if callerRole == models.RoleAdmin {
		return nil
	}
	t, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		return err
	}
	if t.OwnerID == callerID {
		return nil
	}
	for _, a := range t.Assignees {
		if a == callerID {
			return nil
		}
	}
	return ErrForbidden
}

func (s *TaskService) Update(ctx context.Context, taskID, callerID, callerRole string, in UpdateTaskInput) (*models.Task, error) {
	if err := s.authorizeTaskMutation(ctx, taskID, callerID, callerRole); err != nil {
		return nil, err
	}
	if in.Status != nil && !models.ValidStatus(*in.Status) {
		return nil, fmt.Errorf("invalid status: %s", *in.Status)
	}
	if in.Priority != nil && !models.ValidPriority(*in.Priority) {
		return nil, fmt.Errorf("invalid priority: %s", *in.Priority)
	}
	upd := repository.TaskUpdate{
		Title: in.Title, Description: in.Description, Status: in.Status,
		Priority: in.Priority, DueDate: in.DueDate, ClearDueDate: in.ClearDueDate,
	}
	if err := s.tasks.Update(ctx, taskID, upd, callerID); err != nil {
		return nil, err
	}
	_ = s.cache.Del(ctx, taskCacheKey(taskID))
	return s.tasks.GetByID(ctx, taskID)
}

func (s *TaskService) Delete(ctx context.Context, taskID, callerID, callerRole string) error {
	// Only admin or owner can delete
	if callerRole != models.RoleAdmin {
		t, err := s.tasks.GetByID(ctx, taskID)
		if err != nil {
			return err
		}
		if t.OwnerID != callerID {
			return ErrForbidden
		}
	}
	if err := s.tasks.Delete(ctx, taskID); err != nil {
		return err
	}
	_ = s.cache.Del(ctx, taskCacheKey(taskID))
	return nil
}

func (s *TaskService) Assign(ctx context.Context, taskID, assigneeID, callerID, callerRole string) error {
	if err := s.authorizeTaskMutation(ctx, taskID, callerID, callerRole); err != nil {
		return err
	}
	if _, err := s.users.GetByID(ctx, assigneeID); err != nil {
		return fmt.Errorf("assignee: %w", err)
	}
	if err := s.tasks.Assign(ctx, taskID, assigneeID, callerID); err != nil {
		return err
	}
	_ = s.cache.Del(ctx, taskCacheKey(taskID))
	return nil
}

func (s *TaskService) Unassign(ctx context.Context, taskID, assigneeID, callerID, callerRole string) error {
	if err := s.authorizeTaskMutation(ctx, taskID, callerID, callerRole); err != nil {
		return err
	}
	if err := s.tasks.Unassign(ctx, taskID, assigneeID); err != nil {
		return err
	}
	_ = s.cache.Del(ctx, taskCacheKey(taskID))
	return nil
}

func (s *TaskService) History(ctx context.Context, taskID string) ([]*models.TaskHistory, error) {
	return s.tasks.History(ctx, taskID)
}
