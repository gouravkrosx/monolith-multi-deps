package handlers

import (
	"database/sql"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/keploy/taskmanager/internal/auth"
	"github.com/stretchr/testify/require"
)

func TestAuthHandler_Register_201Created(t *testing.T) {
	f := newAuthFixture(t)

	// service.Register: GetByUsername -> ErrNotFound, GetRoleByName, Create, GetByID.
	f.mock.ExpectQuery(`SELECT u.id, u.username.*FROM users u.*WHERE u.username`).
		WithArgs("alice").
		WillReturnError(sql.ErrNoRows)
	f.mock.ExpectQuery(`SELECT id, name.*FROM roles WHERE name`).
		WithArgs("user").
		WillReturnRows(roleRow(2, "user"))
	f.mock.ExpectExec(`INSERT INTO users`).
		WithArgs(sqlmock.AnyArg(), "alice", "alice@example.com", sqlmock.AnyArg(), 2, nil).
		WillReturnResult(sqlmock.NewResult(1, 1))
	f.mock.ExpectQuery(`SELECT u.id, u.username.*FROM users u.*WHERE u.id`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(userRow("u-new", "alice", "alice@example.com", "h", "user", true))

	w := doJSON(f.g, http.MethodPost, "/auth/register", map[string]any{
		"username": "alice",
		"email":    "alice@example.com",
		"password": "hunter2",
	})

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestAuthHandler_Register_400InvalidBody(t *testing.T) {
	f := newAuthFixture(t)
	w := doRaw(f.g, http.MethodPost, "/auth/register", "{not-json")
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.NoError(t, f.mock.ExpectationsWereMet()) // no SQL should fire
}

func TestAuthHandler_Register_403NonUserRole(t *testing.T) {
	f := newAuthFixture(t)
	w := doJSON(f.g, http.MethodPost, "/auth/register", map[string]any{
		"username": "alice",
		"email":    "alice@example.com",
		"password": "hunter2",
		"role":     "admin",
	})
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "cannot self-register")
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestAuthHandler_Register_409UserExists(t *testing.T) {
	f := newAuthFixture(t)
	f.mock.ExpectQuery(`SELECT u.id, u.username.*FROM users u.*WHERE u.username`).
		WithArgs("alice").
		WillReturnRows(userRow("u-existing", "alice", "a@x", "h", "user", true))

	w := doJSON(f.g, http.MethodPost, "/auth/register", map[string]any{
		"username": "alice",
		"email":    "alice@example.com",
		"password": "hunter2",
	})
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestAuthHandler_Register_400ShortPassword(t *testing.T) {
	f := newAuthFixture(t)
	w := doJSON(f.g, http.MethodPost, "/auth/register", map[string]any{
		"username": "alice",
		"email":    "alice@example.com",
		"password": "hi",
	})
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestAuthHandler_Login_200OK(t *testing.T) {
	f := newAuthFixture(t)

	hash, err := auth.HashPassword("hunter2")
	require.NoError(t, err)

	f.mock.ExpectQuery(`SELECT u.id, u.username.*FROM users u.*WHERE u.username`).
		WithArgs("alice").
		WillReturnRows(userRow("u-1", "alice", "a@x", hash, "user", true))

	w := doJSON(f.g, http.MethodPost, "/auth/login", map[string]any{
		"username": "alice",
		"password": "hunter2",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var got struct {
		AccessToken string `json:"access_token"`
		User        struct {
			ID       string `json:"id"`
			Username string `json:"username"`
		} `json:"user"`
	}
	decodeBody(t, w, &got)
	require.NotEmpty(t, got.AccessToken)
	require.Equal(t, "u-1", got.User.ID)

	// token round-trips through the same JWT manager
	claims, err := f.jm.Parse(got.AccessToken)
	require.NoError(t, err)
	require.Equal(t, "u-1", claims.UserID)
	require.Equal(t, "user", claims.Role)

	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestAuthHandler_Login_400InvalidBody(t *testing.T) {
	f := newAuthFixture(t)
	// missing required fields
	w := doJSON(f.g, http.MethodPost, "/auth/login", map[string]any{"username": "alice"})
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestAuthHandler_Login_401UnknownUser(t *testing.T) {
	f := newAuthFixture(t)
	f.mock.ExpectQuery(`SELECT u.id, u.username.*FROM users u.*WHERE u.username`).
		WithArgs("ghost").
		WillReturnError(sql.ErrNoRows)

	w := doJSON(f.g, http.MethodPost, "/auth/login", map[string]any{
		"username": "ghost",
		"password": "whatever",
	})
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestAuthHandler_Login_401WrongPassword(t *testing.T) {
	f := newAuthFixture(t)
	hash, _ := auth.HashPassword("hunter2")

	f.mock.ExpectQuery(`SELECT u.id, u.username.*FROM users u.*WHERE u.username`).
		WithArgs("alice").
		WillReturnRows(userRow("u-1", "alice", "a@x", hash, "user", true))

	w := doJSON(f.g, http.MethodPost, "/auth/login", map[string]any{
		"username": "alice",
		"password": "wrong",
	})
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestAuthHandler_Login_403Inactive(t *testing.T) {
	f := newAuthFixture(t)
	hash, _ := auth.HashPassword("hunter2")

	f.mock.ExpectQuery(`SELECT u.id, u.username.*FROM users u.*WHERE u.username`).
		WithArgs("alice").
		WillReturnRows(userRow("u-1", "alice", "a@x", hash, "user", false))

	w := doJSON(f.g, http.MethodPost, "/auth/login", map[string]any{
		"username": "alice",
		"password": "hunter2",
	})
	require.Equal(t, http.StatusForbidden, w.Code)
	require.NoError(t, f.mock.ExpectationsWereMet())
}
