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

// ----- Create -----

func TestTaskHandler_Create_201Created(t *testing.T) {
	f := newTaskFixture(t, "u-1", models.RoleUser)

	f.mock.ExpectExec(`INSERT INTO tasks`).
		WithArgs(sqlmock.AnyArg(), "Buy milk", "", "pending", "medium", sqlmock.AnyArg(), "u-1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	f.mock.ExpectQuery(`SELECT id, title.*FROM tasks WHERE id`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(taskRow("t-1", "u-1"))
	f.mock.ExpectQuery(`SELECT user_id FROM task_assignments WHERE task_id`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(emptyAssignees())

	w := doJSON(f.g, http.MethodPost, "/tasks", map[string]any{"title": "Buy milk"})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestTaskHandler_Create_400MissingTitle(t *testing.T) {
	f := newTaskFixture(t, "u-1", models.RoleUser)
	w := doJSON(f.g, http.MethodPost, "/tasks", map[string]any{})
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestTaskHandler_Create_400InvalidJSON(t *testing.T) {
	f := newTaskFixture(t, "u-1", models.RoleUser)
	w := doRaw(f.g, http.MethodPost, "/tasks", "{not-json")
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestTaskHandler_Create_400InvalidDueDate(t *testing.T) {
	f := newTaskFixture(t, "u-1", models.RoleUser)
	due := "tomorrow"
	w := doJSON(f.g, http.MethodPost, "/tasks", map[string]any{
		"title":    "Buy milk",
		"due_date": due,
	})
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "due_date must be RFC3339")
	require.NoError(t, f.mock.ExpectationsWereMet())
}

// ----- Get -----

func TestTaskHandler_Get_200OK(t *testing.T) {
	f := newTaskFixture(t, "u-1", models.RoleUser)

	expectGetTaskByID(f.mock, "t-1", "u-1")

	w := doJSON(f.g, http.MethodGet, "/tasks/t-1", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var got models.Task
	decodeBody(t, w, &got)
	require.Equal(t, "t-1", got.ID)
	require.Equal(t, "u-1", got.OwnerID)
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestTaskHandler_Get_404NotFound(t *testing.T) {
	f := newTaskFixture(t, "u-1", models.RoleUser)

	f.mock.ExpectQuery(`SELECT id, title.*FROM tasks WHERE id`).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	w := doJSON(f.g, http.MethodGet, "/tasks/missing", nil)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestTaskHandler_Get_CacheHitSkipsDB(t *testing.T) {
	f := newTaskFixture(t, "u-1", models.RoleUser)

	// Pre-populate the cache so the service short-circuits before any SQL.
	cached := `{"id":"t-1","title":"From Cache","owner_id":"u-1","status":"pending","priority":"medium"}`
	f.mr.Set("task:t-1", cached)

	w := doJSON(f.g, http.MethodGet, "/tasks/t-1", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var got models.Task
	decodeBody(t, w, &got)
	require.Equal(t, "From Cache", got.Title)
	// Crucially, no SQL expectations were registered — verify nothing fired.
	require.NoError(t, f.mock.ExpectationsWereMet())
}

// ----- List -----

func TestTaskHandler_List_NonAdminScopedToOwnerID(t *testing.T) {
	f := newTaskFixture(t, "u-1", models.RoleUser)

	// We expect the WHERE clause to include t.owner_id = ? with caller's uid.
	f.mock.ExpectQuery(`FROM tasks t.*WHERE t.owner_id.*ORDER BY t.created_at DESC`).
		WithArgs("u-1", 50, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "description", "status", "priority",
			"due_date", "owner_id", "created_at", "updated_at",
		}))

	w := doJSON(f.g, http.MethodGet, "/tasks", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestTaskHandler_List_AdminNotScoped(t *testing.T) {
	f := newTaskFixture(t, "admin-1", models.RoleAdmin)

	// No WHERE clause expected; only LIMIT/OFFSET args.
	f.mock.ExpectQuery(`FROM tasks t.*ORDER BY t.created_at DESC`).
		WithArgs(50, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "description", "status", "priority",
			"due_date", "owner_id", "created_at", "updated_at",
		}))

	w := doJSON(f.g, http.MethodGet, "/tasks", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestTaskHandler_List_500OnError(t *testing.T) {
	f := newTaskFixture(t, "admin-1", models.RoleAdmin)
	f.mock.ExpectQuery(`FROM tasks t`).
		WithArgs(50, 0).
		WillReturnError(errors.New("boom"))
	w := doJSON(f.g, http.MethodGet, "/tasks", nil)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.NoError(t, f.mock.ExpectationsWereMet())
}

// ----- Update -----

func TestTaskHandler_Update_200AdminTitleChange(t *testing.T) {
	f := newTaskFixture(t, "admin-1", models.RoleAdmin)

	// Admin path skips the authorize SELECT and goes straight to the tx.
	f.mock.ExpectBegin()
	f.mock.ExpectQuery(`SELECT id, title.*FROM tasks WHERE id.*FOR UPDATE`).
		WithArgs("t-1").
		WillReturnRows(taskRow("t-1", "u-1"))
	f.mock.ExpectExec(`INSERT INTO task_history`).
		WithArgs("t-1", "admin-1", "title", "Hello", "New title").
		WillReturnResult(sqlmock.NewResult(1, 1))
	f.mock.ExpectExec(`UPDATE tasks SET`).
		WithArgs("New title", "t-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	f.mock.ExpectCommit()

	// service then calls cache.Del and re-fetches the task.
	expectGetTaskByID(f.mock, "t-1", "u-1")

	w := doJSON(f.g, http.MethodPatch, "/tasks/t-1", map[string]any{"title": "New title"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestTaskHandler_Update_403ForbiddenForOther(t *testing.T) {
	f := newTaskFixture(t, "u-2", models.RoleUser)

	// Non-admin: service.authorizeTaskMutation does GetByID and finds the
	// task is owned by someone else and has no assignees.
	expectGetTaskByID(f.mock, "t-1", "u-1") // owner is u-1, caller is u-2

	w := doJSON(f.g, http.MethodPatch, "/tasks/t-1", map[string]any{"title": "x"})
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestTaskHandler_Update_404NotFoundAdmin(t *testing.T) {
	f := newTaskFixture(t, "admin-1", models.RoleAdmin)

	// Admin path: tx begins, SELECT FOR UPDATE returns no rows.
	f.mock.ExpectBegin()
	f.mock.ExpectQuery(`SELECT id, title.*FROM tasks WHERE id.*FOR UPDATE`).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)
	f.mock.ExpectRollback()

	w := doJSON(f.g, http.MethodPatch, "/tasks/missing", map[string]any{"title": "x"})
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestTaskHandler_Update_400InvalidStatusAdmin(t *testing.T) {
	f := newTaskFixture(t, "admin-1", models.RoleAdmin)
	// Admin skips authorize; validation fails before any SQL.
	w := doJSON(f.g, http.MethodPatch, "/tasks/t-1", map[string]any{"status": "weird"})
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "invalid status")
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestTaskHandler_Update_400InvalidDueDate(t *testing.T) {
	f := newTaskFixture(t, "admin-1", models.RoleAdmin)
	w := doJSON(f.g, http.MethodPatch, "/tasks/t-1", map[string]any{"due_date": "later"})
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "due_date must be RFC3339")
	require.NoError(t, f.mock.ExpectationsWereMet())
}

// ----- Delete -----

func TestTaskHandler_Delete_200Admin(t *testing.T) {
	f := newTaskFixture(t, "admin-1", models.RoleAdmin)

	f.mock.ExpectExec(`DELETE FROM tasks WHERE id`).
		WithArgs("t-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	w := doJSON(f.g, http.MethodDelete, "/tasks/t-1", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestTaskHandler_Delete_403NonOwner(t *testing.T) {
	f := newTaskFixture(t, "u-2", models.RoleUser)

	// Non-admin path checks ownership via GetByID before deleting.
	expectGetTaskByID(f.mock, "t-1", "u-1") // owned by u-1, caller is u-2

	w := doJSON(f.g, http.MethodDelete, "/tasks/t-1", nil)
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestTaskHandler_Delete_404Admin(t *testing.T) {
	f := newTaskFixture(t, "admin-1", models.RoleAdmin)

	f.mock.ExpectExec(`DELETE FROM tasks WHERE id`).
		WithArgs("missing").
		WillReturnResult(sqlmock.NewResult(0, 0))

	w := doJSON(f.g, http.MethodDelete, "/tasks/missing", nil)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

// ----- Assign -----

func TestTaskHandler_Assign_200Admin(t *testing.T) {
	f := newTaskFixture(t, "admin-1", models.RoleAdmin)

	// users.GetByID for assignee.
	f.mock.ExpectQuery(`SELECT u.id, u.username.*FROM users u.*WHERE u.id`).
		WithArgs("u-2").
		WillReturnRows(userRow("u-2", "bob", "b@x", "h", "user", true))
	f.mock.ExpectExec(`INSERT IGNORE INTO task_assignments`).
		WithArgs("t-1", "u-2", "admin-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	w := doJSON(f.g, http.MethodPost, "/tasks/t-1/assign", map[string]any{"user_id": "u-2"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestTaskHandler_Assign_400InvalidBody(t *testing.T) {
	f := newTaskFixture(t, "admin-1", models.RoleAdmin)
	// missing user_id
	w := doJSON(f.g, http.MethodPost, "/tasks/t-1/assign", map[string]any{})
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestTaskHandler_Assign_403NonOwner(t *testing.T) {
	f := newTaskFixture(t, "u-2", models.RoleUser)
	expectGetTaskByID(f.mock, "t-1", "u-1") // owned by u-1
	w := doJSON(f.g, http.MethodPost, "/tasks/t-1/assign", map[string]any{"user_id": "u-3"})
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

// ----- Unassign -----

func TestTaskHandler_Unassign_200Admin(t *testing.T) {
	f := newTaskFixture(t, "admin-1", models.RoleAdmin)
	f.mock.ExpectExec(`DELETE FROM task_assignments WHERE task_id`).
		WithArgs("t-1", "u-2").
		WillReturnResult(sqlmock.NewResult(0, 1))
	w := doJSON(f.g, http.MethodDelete, "/tasks/t-1/assign/u-2", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestTaskHandler_Unassign_404Admin(t *testing.T) {
	f := newTaskFixture(t, "admin-1", models.RoleAdmin)
	f.mock.ExpectExec(`DELETE FROM task_assignments WHERE task_id`).
		WithArgs("t-1", "u-2").
		WillReturnResult(sqlmock.NewResult(0, 0))
	w := doJSON(f.g, http.MethodDelete, "/tasks/t-1/assign/u-2", nil)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

// ----- History -----

func TestTaskHandler_History_200OK(t *testing.T) {
	f := newTaskFixture(t, "admin-1", models.RoleAdmin)
	f.mock.ExpectQuery(`FROM task_history WHERE task_id`).
		WithArgs("t-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "task_id", "changed_by", "field", "old_value", "new_value", "changed_at",
		}))
	w := doJSON(f.g, http.MethodGet, "/tasks/t-1/history", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestTaskHandler_History_500OnError(t *testing.T) {
	f := newTaskFixture(t, "admin-1", models.RoleAdmin)
	f.mock.ExpectQuery(`FROM task_history WHERE task_id`).
		WithArgs("t-1").
		WillReturnError(errors.New("boom"))
	w := doJSON(f.g, http.MethodGet, "/tasks/t-1/history", nil)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.NoError(t, f.mock.ExpectationsWereMet())
}
