package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/keploy/taskmanager/internal/auth"
	"github.com/keploy/taskmanager/internal/cache"
	"github.com/keploy/taskmanager/internal/config"
	"github.com/keploy/taskmanager/internal/external"
	"github.com/keploy/taskmanager/internal/middleware"
	"github.com/keploy/taskmanager/internal/repository"
	"github.com/keploy/taskmanager/internal/service"
	"github.com/stretchr/testify/require"
)

func init() { gin.SetMode(gin.TestMode) }

func newDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}

func newCache(t *testing.T) (*cache.Cache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	c, err := cache.New(context.Background(), config.RedisConfig{Host: mr.Host(), Port: mr.Port()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c, mr
}

func withCaller(uid, role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if uid != "" {
			c.Set(middleware.CtxUserID, uid)
		}
		if role != "" {
			c.Set(middleware.CtxRole, role)
		}
		c.Next()
	}
}

func doJSON(g *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	return w
}

func doRaw(g *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	return w
}

// taskRow returns a sqlmock row matching the columns scanned by repository.scanTask.
func taskRow(id, owner string) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"id", "title", "description", "status", "priority",
		"due_date", "owner_id", "created_at", "updated_at",
	}).AddRow(id, "Hello", "", "pending", "medium", nil, owner, now, now)
}

func emptyAssignees() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"user_id"})
}

// userRowCols / userRow build the columns scanned by repository.scanUser.
func userRow(id, username, email, hash, roleName string, active bool) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"id", "username", "email", "password_hash", "role_id", "name",
		"external_id", "is_active", "created_at", "updated_at",
	}).AddRow(id, username, email, hash, 2, roleName, nil, active, now, now)
}

func roleRow(id int, name string) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{"id", "name", "description", "created_at"}).
		AddRow(id, name, "", now)
}

// expectGetTaskByID wires up the SELECT + assignees pair the repo issues for a
// successful GetByID.
func expectGetTaskByID(mock sqlmock.Sqlmock, id, owner string) {
	mock.ExpectQuery(`SELECT id, title.*FROM tasks WHERE id`).
		WithArgs(id).
		WillReturnRows(taskRow(id, owner))
	mock.ExpectQuery(`SELECT user_id FROM task_assignments WHERE task_id`).
		WithArgs(id).
		WillReturnRows(emptyAssignees())
}

// taskFixture wires a real TaskService over a sqlmock DB and miniredis cache,
// then mounts every TaskHandler route on a fresh gin.Engine.
type taskFixture struct {
	g    *gin.Engine
	mock sqlmock.Sqlmock
	mr   *miniredis.Miniredis
}

func newTaskFixture(t *testing.T, uid, role string) *taskFixture {
	t.Helper()
	db, mock := newDB(t)
	rcache, mr := newCache(t)
	svc := service.NewTaskService(repository.NewTaskRepo(db), repository.NewUserRepo(db), rcache)
	h := NewTaskHandler(svc)

	g := gin.New()
	g.Use(withCaller(uid, role))
	g.POST("/tasks", h.Create)
	g.GET("/tasks", h.List)
	g.GET("/tasks/:id", h.Get)
	g.PATCH("/tasks/:id", h.Update)
	g.DELETE("/tasks/:id", h.Delete)
	g.GET("/tasks/:id/history", h.History)
	g.POST("/tasks/:id/assign", h.Assign)
	g.DELETE("/tasks/:id/assign/:user_id", h.Unassign)
	return &taskFixture{g: g, mock: mock, mr: mr}
}

type authFixture struct {
	g    *gin.Engine
	mock sqlmock.Sqlmock
	jm   *auth.Manager
}

func newAuthFixture(t *testing.T) *authFixture {
	t.Helper()
	db, mock := newDB(t)
	jm := auth.NewManager("test-secret", time.Hour)
	svc := service.NewAuthService(repository.NewUserRepo(db), jm)
	h := NewAuthHandler(svc)

	g := gin.New()
	g.POST("/auth/register", h.Register)
	g.POST("/auth/login", h.Login)
	return &authFixture{g: g, mock: mock, jm: jm}
}

type userFixture struct {
	g    *gin.Engine
	mock sqlmock.Sqlmock
	mr   *miniredis.Miniredis
}

func newUserFixture(t *testing.T, uid, role string) *userFixture {
	t.Helper()
	db, mock := newDB(t)
	rcache, mr := newCache(t)
	// External client points to an unreachable URL; the only handler that
	// hits it (Me) is exercised with users that have ExternalID == nil, so
	// it never fires.
	ext := external.NewClient("http://127.0.0.1:1")
	svc := service.NewUserService(repository.NewUserRepo(db), rcache, ext)
	h := NewUserHandler(svc)

	g := gin.New()
	g.Use(withCaller(uid, role))
	g.GET("/users", h.List)
	g.GET("/users/:id", h.Get)
	g.GET("/me", h.Me)
	g.PATCH("/users/:id/role", h.UpdateRole)
	g.DELETE("/users/:id", h.Deactivate)
	return &userFixture{g: g, mock: mock, mr: mr}
}

// decodeBody helper for asserting JSON shape.
func decodeBody(t *testing.T, w *httptest.ResponseRecorder, dst any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), dst))
}
