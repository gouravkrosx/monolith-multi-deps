package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/keploy/taskmanager/internal/auth"
	"github.com/keploy/taskmanager/internal/middleware"
)

type Router struct {
	Auth  *AuthHandler
	Users *UserHandler
	Tasks *TaskHandler
}

func (r *Router) Build(jm *auth.Manager, log *slog.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	g := gin.New()
	g.Use(middleware.RequestLogger(log), middleware.Recovery(log))

	g.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := g.Group("/api/v1")
	{
		api.POST("/auth/register", r.Auth.Register)
		api.POST("/auth/login", r.Auth.Login)
	}

	authed := api.Group("")
	authed.Use(middleware.RequireAuth(jm))
	{
		authed.GET("/me", r.Users.Me)

		authed.GET("/users", r.Users.List)
		authed.GET("/users/:id", r.Users.Get)

		// admin-only user management
		admin := authed.Group("/users", middleware.RequireRole("admin"))
		admin.PATCH("/:id/role", r.Users.UpdateRole)
		admin.DELETE("/:id", r.Users.Deactivate)

		authed.POST("/tasks", r.Tasks.Create)
		authed.GET("/tasks", r.Tasks.List)
		authed.GET("/tasks/:id", r.Tasks.Get)
		authed.PATCH("/tasks/:id", r.Tasks.Update)
		authed.DELETE("/tasks/:id", r.Tasks.Delete)
		authed.GET("/tasks/:id/history", r.Tasks.History)
		authed.POST("/tasks/:id/assign", r.Tasks.Assign)
		authed.DELETE("/tasks/:id/assign/:user_id", r.Tasks.Unassign)
	}

	return g
}
