package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/keploy/taskmanager/internal/middleware"
	"github.com/keploy/taskmanager/internal/service"
)

type UserHandler struct {
	users *service.UserService
}

func NewUserHandler(u *service.UserService) *UserHandler { return &UserHandler{users: u} }

func (h *UserHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	out, err := h.users.List(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": out})
}

func (h *UserHandler) Get(c *gin.Context) {
	u, err := h.users.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if service.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *UserHandler) Me(c *gin.Context) {
	id, _ := c.Get(middleware.CtxUserID)
	u, err := h.users.Enrich(c.Request.Context(), id.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, u)
}

type updateRoleReq struct {
	Role string `json:"role" binding:"required"`
}

func (h *UserHandler) UpdateRole(c *gin.Context) {
	var req updateRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.users.UpdateRole(c.Request.Context(), c.Param("id"), req.Role); err != nil {
		if service.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "role updated"})
}

func (h *UserHandler) Deactivate(c *gin.Context) {
	if err := h.users.Deactivate(c.Request.Context(), c.Param("id")); err != nil {
		if service.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "user deactivated"})
}
