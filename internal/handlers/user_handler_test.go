package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/keploy/taskmanager/internal/models"
	"github.com/stretchr/testify/require"
)

// ----- List -----

func TestUserHandler_List_200OK(t *testing.T) {
	f := newUserFixture(t, "admin-1", models.RoleAdmin)
	f.mock.ExpectQuery(`FROM users u.*ORDER BY u.created_at DESC`).
		WithArgs(50, 0).
		WillReturnRows(userRow("u-1", "alice", "a@x", "h", "user", true))
	w := doJSON(f.g, http.MethodGet, "/users", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestUserHandler_List_500OnError(t *testing.T) {
	f := newUserFixture(t, "admin-1", models.RoleAdmin)
	f.mock.ExpectQuery(`FROM users u`).
		WithArgs(50, 0).
		WillReturnError(errors.New("boom"))
	w := doJSON(f.g, http.MethodGet, "/users", nil)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.NoError(t, f.mock.ExpectationsWereMet())
}

// ----- Get -----

func TestUserHandler_Get_200OK(t *testing.T) {
	f := newUserFixture(t, "admin-1", models.RoleAdmin)
	f.mock.ExpectQuery(`FROM users u.*WHERE u.id`).
		WithArgs("u-1").
		WillReturnRows(userRow("u-1", "alice", "a@x", "h", "user", true))
	w := doJSON(f.g, http.MethodGet, "/users/u-1", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var got models.User
	decodeBody(t, w, &got)
	require.Equal(t, "u-1", got.ID)
	require.Empty(t, got.PasswordHash, "password_hash must not be returned")
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestUserHandler_Get_404NotFound(t *testing.T) {
	f := newUserFixture(t, "admin-1", models.RoleAdmin)
	f.mock.ExpectQuery(`FROM users u.*WHERE u.id`).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)
	w := doJSON(f.g, http.MethodGet, "/users/missing", nil)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

// ----- Me -----

func TestUserHandler_Me_200NoExternal(t *testing.T) {
	f := newUserFixture(t, "u-1", models.RoleUser)
	// ExternalID is nil in userRow → Enrich returns early without hitting external.
	f.mock.ExpectQuery(`FROM users u.*WHERE u.id`).
		WithArgs("u-1").
		WillReturnRows(userRow("u-1", "alice", "a@x", "h", "user", true))
	w := doJSON(f.g, http.MethodGet, "/me", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestUserHandler_Me_500OnError(t *testing.T) {
	f := newUserFixture(t, "u-1", models.RoleUser)
	f.mock.ExpectQuery(`FROM users u.*WHERE u.id`).
		WithArgs("u-1").
		WillReturnError(errors.New("boom"))
	w := doJSON(f.g, http.MethodGet, "/me", nil)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.NoError(t, f.mock.ExpectationsWereMet())
}

// ----- UpdateRole -----

func TestUserHandler_UpdateRole_200OK(t *testing.T) {
	f := newUserFixture(t, "admin-1", models.RoleAdmin)
	f.mock.ExpectQuery(`SELECT id, name.*FROM roles WHERE name`).
		WithArgs("admin").
		WillReturnRows(roleRow(1, "admin"))
	f.mock.ExpectExec(`UPDATE users SET role_id`).
		WithArgs(1, "u-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	w := doJSON(f.g, http.MethodPatch, "/users/u-1/role", map[string]any{"role": "admin"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestUserHandler_UpdateRole_400InvalidBody(t *testing.T) {
	f := newUserFixture(t, "admin-1", models.RoleAdmin)
	w := doJSON(f.g, http.MethodPatch, "/users/u-1/role", map[string]any{})
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestUserHandler_UpdateRole_404UnknownRole(t *testing.T) {
	f := newUserFixture(t, "admin-1", models.RoleAdmin)
	// Role lookup itself returns ErrNotFound; service wraps it but the wrap
	// preserves errors.Is so IsNotFound is true → 404.
	f.mock.ExpectQuery(`SELECT id, name.*FROM roles WHERE name`).
		WithArgs("ghost").
		WillReturnError(sql.ErrNoRows)
	w := doJSON(f.g, http.MethodPatch, "/users/u-1/role", map[string]any{"role": "ghost"})
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestUserHandler_UpdateRole_404UnknownUser(t *testing.T) {
	f := newUserFixture(t, "admin-1", models.RoleAdmin)
	f.mock.ExpectQuery(`SELECT id, name.*FROM roles WHERE name`).
		WithArgs("admin").
		WillReturnRows(roleRow(1, "admin"))
	f.mock.ExpectExec(`UPDATE users SET role_id`).
		WithArgs(1, "missing").
		WillReturnResult(sqlmock.NewResult(0, 0))
	w := doJSON(f.g, http.MethodPatch, "/users/missing/role", map[string]any{"role": "admin"})
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

// ----- Deactivate -----

func TestUserHandler_Deactivate_200OK(t *testing.T) {
	f := newUserFixture(t, "admin-1", models.RoleAdmin)
	f.mock.ExpectExec(`UPDATE users SET is_active`).
		WithArgs("u-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	w := doJSON(f.g, http.MethodDelete, "/users/u-1", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestUserHandler_Deactivate_404NotFound(t *testing.T) {
	f := newUserFixture(t, "admin-1", models.RoleAdmin)
	f.mock.ExpectExec(`UPDATE users SET is_active`).
		WithArgs("missing").
		WillReturnResult(sqlmock.NewResult(0, 0))
	w := doJSON(f.g, http.MethodDelete, "/users/missing", nil)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}
