package models

import "time"

type Role struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type User struct {
	ID           string     `json:"id"`
	Username     string     `json:"username"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	RoleID       int        `json:"role_id"`
	RoleName     string     `json:"role"`
	ExternalID   *int       `json:"external_id,omitempty"`
	IsActive     bool       `json:"is_active"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type Task struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	OwnerID     string     `json:"owner_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Assignees   []string   `json:"assignees,omitempty"`
}

type TaskAssignment struct {
	ID         int64     `json:"id"`
	TaskID     string    `json:"task_id"`
	UserID     string    `json:"user_id"`
	AssignedBy string    `json:"assigned_by"`
	AssignedAt time.Time `json:"assigned_at"`
}

type TaskHistory struct {
	ID        int64     `json:"id"`
	TaskID    string    `json:"task_id"`
	ChangedBy string    `json:"changed_by"`
	Field     string    `json:"field"`
	OldValue  string    `json:"old_value"`
	NewValue  string    `json:"new_value"`
	ChangedAt time.Time `json:"changed_at"`
}

const (
	RoleAdmin = "admin"
	RoleUser  = "user"

	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusCancelled  = "cancelled"

	PriorityLow    = "low"
	PriorityMedium = "medium"
	PriorityHigh   = "high"
	PriorityUrgent = "urgent"
)

func ValidStatus(s string) bool {
	switch s {
	case StatusPending, StatusInProgress, StatusCompleted, StatusCancelled:
		return true
	}
	return false
}

func ValidPriority(p string) bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent:
		return true
	}
	return false
}
