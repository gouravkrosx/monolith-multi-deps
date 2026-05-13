package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/keploy/taskmanager/internal/middleware"
	"github.com/keploy/taskmanager/internal/repository"
	"github.com/keploy/taskmanager/internal/service"
)

type TaskHandler struct {
	tasks *service.TaskService
}

func NewTaskHandler(t *service.TaskService) *TaskHandler { return &TaskHandler{tasks: t} }

type createTaskReq struct {
	Title       string  `json:"title" binding:"required"`
	Description string  `json:"description"`
	Priority    string  `json:"priority"`
	DueDate     *string `json:"due_date,omitempty"`
}

func parseDue(s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (h *TaskHandler) Create(c *gin.Context) {
	var req createTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	due, err := parseDue(req.DueDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "due_date must be RFC3339"})
		return
	}
	uid, _ := c.Get(middleware.CtxUserID)
	t, err := h.tasks.Create(c.Request.Context(), service.CreateTaskInput{
		Title: req.Title, Description: req.Description, Priority: req.Priority,
		DueDate: due, OwnerID: uid.(string),
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, t)
}

func (h *TaskHandler) Get(c *gin.Context) {
	t, err := h.tasks.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		if service.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, t)
}

func (h *TaskHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	f := repository.TaskFilter{
		OwnerID:    c.Query("owner_id"),
		AssigneeID: c.Query("assignee_id"),
		Status:     c.Query("status"),
		Limit:      limit,
		Offset:     offset,
	}
	// non-admins implicitly scoped to "their" tasks if no filter given
	role, _ := c.Get(middleware.CtxRole)
	uid, _ := c.Get(middleware.CtxUserID)
	if role.(string) != "admin" && f.OwnerID == "" && f.AssigneeID == "" {
		f.OwnerID = uid.(string)
	}
	out, err := h.tasks.List(c.Request.Context(), f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tasks": out})
}

type updateTaskReq struct {
	Title        *string `json:"title,omitempty"`
	Description  *string `json:"description,omitempty"`
	Status       *string `json:"status,omitempty"`
	Priority     *string `json:"priority,omitempty"`
	DueDate      *string `json:"due_date,omitempty"`
	ClearDueDate bool    `json:"clear_due_date,omitempty"`
}

func (h *TaskHandler) Update(c *gin.Context) {
	var req updateTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	due, err := parseDue(req.DueDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "due_date must be RFC3339"})
		return
	}
	uid, _ := c.Get(middleware.CtxUserID)
	role, _ := c.Get(middleware.CtxRole)
	t, err := h.tasks.Update(c.Request.Context(), c.Param("id"), uid.(string), role.(string),
		service.UpdateTaskInput{
			Title: req.Title, Description: req.Description, Status: req.Status,
			Priority: req.Priority, DueDate: due, ClearDueDate: req.ClearDueDate,
		})
	if err != nil {
		mapTaskErr(c, err)
		return
	}
	c.JSON(http.StatusOK, t)
}

func (h *TaskHandler) Delete(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxUserID)
	role, _ := c.Get(middleware.CtxRole)
	if err := h.tasks.Delete(c.Request.Context(), c.Param("id"), uid.(string), role.(string)); err != nil {
		mapTaskErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

type assignReq struct {
	UserID string `json:"user_id" binding:"required"`
}

func (h *TaskHandler) Assign(c *gin.Context) {
	var req assignReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	uid, _ := c.Get(middleware.CtxUserID)
	role, _ := c.Get(middleware.CtxRole)
	if err := h.tasks.Assign(c.Request.Context(), c.Param("id"), req.UserID, uid.(string), role.(string)); err != nil {
		mapTaskErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "assigned"})
}

func (h *TaskHandler) Unassign(c *gin.Context) {
	uid, _ := c.Get(middleware.CtxUserID)
	role, _ := c.Get(middleware.CtxRole)
	if err := h.tasks.Unassign(c.Request.Context(), c.Param("id"), c.Param("user_id"), uid.(string), role.(string)); err != nil {
		mapTaskErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "unassigned"})
}

func (h *TaskHandler) History(c *gin.Context) {
	hist, err := h.tasks.History(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"history": hist})
}

func mapTaskErr(c *gin.Context, err error) {
	switch {
	case service.IsNotFound(err):
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
	case errors.Is(err, service.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}
