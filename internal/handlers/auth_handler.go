package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/keploy/taskmanager/internal/service"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(a *service.AuthService) *AuthHandler { return &AuthHandler{auth: a} }

type registerReq struct {
	Username   string `json:"username"`
	Email      string `json:"email"`
	Password   string `json:"password"`
	Role       string `json:"role,omitempty"`
	ExternalID *int   `json:"external_id,omitempty"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Self-service registration is restricted to "user" role.
	if req.Role != "" && req.Role != "user" {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot self-register as " + req.Role})
		return
	}
	u, err := h.auth.Register(c.Request.Context(), service.RegisterInput{
		Username: req.Username, Email: req.Email, Password: req.Password,
		Role: req.Role, ExternalID: req.ExternalID,
	})
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, service.ErrUserExists) {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, u)
}

type loginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.auth.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, service.ErrInactiveUser) {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token": res.Token,
		"expires_at":   res.ExpiresAt,
		"user":         res.User,
	})
}
