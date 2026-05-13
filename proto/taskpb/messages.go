package taskpb

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
	UserID      string `json:"user_id"`
	Role        string `json:"role"`
}

type Task struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	DueDate     string   `json:"due_date"`
	OwnerID     string   `json:"owner_id"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	Assignees   []string `json:"assignees,omitempty"`
}

type CreateTaskRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	DueDate     string `json:"due_date"`
}

type GetTaskRequest struct {
	ID string `json:"id"`
}

type ListTasksRequest struct {
	OwnerID    string `json:"owner_id"`
	AssigneeID string `json:"assignee_id"`
	Status     string `json:"status"`
	Limit      int32  `json:"limit"`
	Offset     int32  `json:"offset"`
}

type ListTasksResponse struct {
	Tasks []*Task `json:"tasks"`
}

type UpdateTaskRequest struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	Priority     string `json:"priority"`
	DueDate      string `json:"due_date"`
	ClearDueDate bool   `json:"clear_due_date"`
}

type DeleteTaskRequest struct {
	ID string `json:"id"`
}
type DeleteTaskResponse struct {
	Status string `json:"status"`
}

type AssignTaskRequest struct {
	TaskID string `json:"task_id"`
	UserID string `json:"user_id"`
}
type UnassignTaskRequest struct {
	TaskID string `json:"task_id"`
	UserID string `json:"user_id"`
}
type AssignResponse struct {
	Status string `json:"status"`
}
