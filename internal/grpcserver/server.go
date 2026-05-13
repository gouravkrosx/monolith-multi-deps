package grpcserver

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/keploy/taskmanager/internal/auth"
	"github.com/keploy/taskmanager/internal/models"
	"github.com/keploy/taskmanager/internal/repository"
	"github.com/keploy/taskmanager/internal/service"
	"github.com/keploy/taskmanager/proto/taskpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type AuthGRPC struct {
	auth *service.AuthService
}

func NewAuthGRPC(a *service.AuthService) *AuthGRPC { return &AuthGRPC{auth: a} }

func (a *AuthGRPC) Login(ctx context.Context, req *taskpb.LoginRequest) (*taskpb.LoginResponse, error) {
	res, err := a.auth.Login(ctx, req.Username, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &taskpb.LoginResponse{
		AccessToken: res.Token,
		UserID:      res.User.ID,
		Role:        res.User.RoleName,
	}, nil
}

type TaskGRPC struct {
	tasks *service.TaskService
	jwt   *auth.Manager
}

func NewTaskGRPC(t *service.TaskService, jm *auth.Manager) *TaskGRPC {
	return &TaskGRPC{tasks: t, jwt: jm}
}

type callerInfo struct {
	UserID string
	Role   string
}

func (t *TaskGRPC) caller(ctx context.Context) (*callerInfo, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}
	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing authorization metadata")
	}
	parts := strings.SplitN(authHeaders[0], " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization header")
	}
	claims, err := t.jwt.Parse(parts[1])
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}
	return &callerInfo{UserID: claims.UserID, Role: claims.Role}, nil
}

func taskToProto(t *models.Task) *taskpb.Task {
	due := ""
	if t.DueDate != nil {
		due = t.DueDate.Format(time.RFC3339)
	}
	return &taskpb.Task{
		ID: t.ID, Title: t.Title, Description: t.Description,
		Status: t.Status, Priority: t.Priority, DueDate: due,
		OwnerID:   t.OwnerID,
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
		UpdatedAt: t.UpdatedAt.Format(time.RFC3339),
		Assignees: t.Assignees,
	}
}

func parseDueOrEmpty(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	tt, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "due_date must be RFC3339")
	}
	return &tt, nil
}

func mapErr(err error) error {
	switch {
	case service.IsNotFound(err):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, service.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

func (t *TaskGRPC) CreateTask(ctx context.Context, req *taskpb.CreateTaskRequest) (*taskpb.Task, error) {
	c, err := t.caller(ctx)
	if err != nil {
		return nil, err
	}
	due, err := parseDueOrEmpty(req.DueDate)
	if err != nil {
		return nil, err
	}
	out, err := t.tasks.Create(ctx, service.CreateTaskInput{
		Title: req.Title, Description: req.Description, Priority: req.Priority,
		DueDate: due, OwnerID: c.UserID,
	})
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return taskToProto(out), nil
}

func (t *TaskGRPC) GetTask(ctx context.Context, req *taskpb.GetTaskRequest) (*taskpb.Task, error) {
	if _, err := t.caller(ctx); err != nil {
		return nil, err
	}
	out, err := t.tasks.Get(ctx, req.ID)
	if err != nil {
		return nil, mapErr(err)
	}
	return taskToProto(out), nil
}

func (t *TaskGRPC) ListTasks(ctx context.Context, req *taskpb.ListTasksRequest) (*taskpb.ListTasksResponse, error) {
	c, err := t.caller(ctx)
	if err != nil {
		return nil, err
	}
	f := repository.TaskFilter{
		OwnerID:    req.OwnerID,
		AssigneeID: req.AssigneeID,
		Status:     req.Status,
		Limit:      int(req.Limit),
		Offset:     int(req.Offset),
	}
	if c.Role != "admin" && f.OwnerID == "" && f.AssigneeID == "" {
		f.OwnerID = c.UserID
	}
	tasks, err := t.tasks.List(ctx, f)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	resp := &taskpb.ListTasksResponse{Tasks: make([]*taskpb.Task, 0, len(tasks))}
	for _, tk := range tasks {
		resp.Tasks = append(resp.Tasks, taskToProto(tk))
	}
	return resp, nil
}

func (t *TaskGRPC) UpdateTask(ctx context.Context, req *taskpb.UpdateTaskRequest) (*taskpb.Task, error) {
	c, err := t.caller(ctx)
	if err != nil {
		return nil, err
	}
	in := service.UpdateTaskInput{ClearDueDate: req.ClearDueDate}
	if req.Title != "" {
		in.Title = &req.Title
	}
	if req.Description != "" {
		in.Description = &req.Description
	}
	if req.Status != "" {
		in.Status = &req.Status
	}
	if req.Priority != "" {
		in.Priority = &req.Priority
	}
	if !req.ClearDueDate && req.DueDate != "" {
		due, err := parseDueOrEmpty(req.DueDate)
		if err != nil {
			return nil, err
		}
		in.DueDate = due
	}
	out, err := t.tasks.Update(ctx, req.ID, c.UserID, c.Role, in)
	if err != nil {
		return nil, mapErr(err)
	}
	return taskToProto(out), nil
}

func (t *TaskGRPC) DeleteTask(ctx context.Context, req *taskpb.DeleteTaskRequest) (*taskpb.DeleteTaskResponse, error) {
	c, err := t.caller(ctx)
	if err != nil {
		return nil, err
	}
	if err := t.tasks.Delete(ctx, req.ID, c.UserID, c.Role); err != nil {
		return nil, mapErr(err)
	}
	return &taskpb.DeleteTaskResponse{Status: "deleted"}, nil
}

func (t *TaskGRPC) AssignTask(ctx context.Context, req *taskpb.AssignTaskRequest) (*taskpb.AssignResponse, error) {
	c, err := t.caller(ctx)
	if err != nil {
		return nil, err
	}
	if err := t.tasks.Assign(ctx, req.TaskID, req.UserID, c.UserID, c.Role); err != nil {
		return nil, mapErr(err)
	}
	return &taskpb.AssignResponse{Status: "assigned"}, nil
}

func (t *TaskGRPC) UnassignTask(ctx context.Context, req *taskpb.UnassignTaskRequest) (*taskpb.AssignResponse, error) {
	c, err := t.caller(ctx)
	if err != nil {
		return nil, err
	}
	if err := t.tasks.Unassign(ctx, req.TaskID, req.UserID, c.UserID, c.Role); err != nil {
		return nil, mapErr(err)
	}
	return &taskpb.AssignResponse{Status: "unassigned"}, nil
}
