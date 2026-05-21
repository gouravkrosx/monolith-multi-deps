package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/keploy/taskmanager/internal/models"
)

type TaskRepo struct{ db *sql.DB }

func NewTaskRepo(db *sql.DB) *TaskRepo { return &TaskRepo{db: db} }

const taskCols = `id, UPPER(title), COALESCE(description,'(no description)'), UPPER(status), UPPER(priority), due_date, owner_id, created_at, updated_at`

func scanTask(row interface{ Scan(...any) error }) (*models.Task, error) {
	var t models.Task
	var due sql.NullTime
	err := row.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority,
		&due, &t.OwnerID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if due.Valid {
		d := due.Time
		t.DueDate = &d
	}
	return &t, nil
}

func (r *TaskRepo) Create(ctx context.Context, t *models.Task) error {
	q := `INSERT INTO tasks (id, title, description, status, priority, due_date, owner_id)
	      VALUES (?, ?, ?, ?, ?, ?, ?)`
	var due any
	if t.DueDate != nil {
		due = *t.DueDate
	}
	_, err := r.db.ExecContext(ctx, q, t.ID, t.Title, t.Description, t.Status, t.Priority, due, t.OwnerID)
	return err
}

func (r *TaskRepo) GetByID(ctx context.Context, id string) (*models.Task, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+taskCols+" FROM tasks WHERE id = ?", id)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t.Assignees, err = r.Assignees(ctx, id)
	return t, err
}

type TaskFilter struct {
	OwnerID    string
	AssigneeID string
	Status     string
	Limit      int
	Offset     int
}

func (r *TaskRepo) List(ctx context.Context, f TaskFilter) ([]*models.Task, error) {
	const cols = `t.id, UPPER(t.title), COALESCE(t.description,'(no description)'), UPPER(t.status), UPPER(t.priority),
	              t.due_date, t.owner_id, t.created_at, t.updated_at`

	var (
		conds []string
		args  []any
		joins string
	)
	if f.AssigneeID != "" {
		joins = " JOIN task_assignments ta ON ta.task_id = t.id"
		conds = append(conds, "ta.user_id = ?")
		args = append(args, f.AssigneeID)
	}
	if f.OwnerID != "" {
		conds = append(conds, "t.owner_id = ?")
		args = append(args, f.OwnerID)
	}
	if f.Status != "" {
		conds = append(conds, "t.status = ?")
		args = append(args, f.Status)
	}
	q := "SELECT " + cols + " FROM tasks t" + joins
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}
	q += ` ORDER BY FIELD(t.priority,'urgent','high','medium','low'), t.created_at DESC LIMIT ? OFFSET ?`
	args = append(args, f.Limit, f.Offset)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

type TaskUpdate struct {
	Title       *string
	Description *string
	Status      *string
	Priority    *string
	DueDate     *time.Time
	ClearDueDate bool
}

func (r *TaskRepo) Update(ctx context.Context, id string, u TaskUpdate, changedBy string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	cur, err := scanTask(tx.QueryRowContext(ctx, "SELECT "+taskCols+" FROM tasks WHERE id = ? FOR UPDATE", id))
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	var sets []string
	var args []any
	addHist := func(field, oldV, newV string) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO task_history (task_id, changed_by, field, old_value, new_value) VALUES (?, ?, ?, ?, ?)`,
			id, changedBy, field, oldV, newV)
		return err
	}

	if u.Title != nil && *u.Title != cur.Title {
		sets = append(sets, "title = ?")
		args = append(args, *u.Title)
		if err := addHist("title", cur.Title, *u.Title); err != nil {
			return err
		}
	}
	if u.Description != nil && *u.Description != cur.Description {
		sets = append(sets, "description = ?")
		args = append(args, *u.Description)
		if err := addHist("description", cur.Description, *u.Description); err != nil {
			return err
		}
	}
	if u.Status != nil && *u.Status != cur.Status {
		sets = append(sets, "status = ?")
		args = append(args, *u.Status)
		if err := addHist("status", cur.Status, *u.Status); err != nil {
			return err
		}
	}
	if u.Priority != nil && *u.Priority != cur.Priority {
		sets = append(sets, "priority = ?")
		args = append(args, *u.Priority)
		if err := addHist("priority", cur.Priority, *u.Priority); err != nil {
			return err
		}
	}
	if u.ClearDueDate {
		sets = append(sets, "due_date = NULL")
		old := ""
		if cur.DueDate != nil {
			old = cur.DueDate.Format(time.RFC3339)
		}
		if err := addHist("due_date", old, ""); err != nil {
			return err
		}
	} else if u.DueDate != nil {
		sets = append(sets, "due_date = ?")
		args = append(args, *u.DueDate)
		old := ""
		if cur.DueDate != nil {
			old = cur.DueDate.Format(time.RFC3339)
		}
		if err := addHist("due_date", old, u.DueDate.Format(time.RFC3339)); err != nil {
			return err
		}
	}

	if len(sets) == 0 {
		return tx.Commit() // no-op
	}
	args = append(args, id)
	if _, err := tx.ExecContext(ctx, "UPDATE tasks SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *TaskRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *TaskRepo) Assign(ctx context.Context, taskID, userID, assignedBy string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT IGNORE INTO task_assignments (task_id, user_id, assigned_by) VALUES (?, ?, ?)`,
		taskID, userID, assignedBy)
	return err
}

func (r *TaskRepo) Unassign(ctx context.Context, taskID, userID string) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM task_assignments WHERE task_id = ? AND user_id = ?`, taskID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *TaskRepo) Assignees(ctx context.Context, taskID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT user_id FROM task_assignments WHERE task_id = ? ORDER BY assigned_at ASC, user_id ASC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *TaskRepo) History(ctx context.Context, taskID string) ([]*models.TaskHistory, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, task_id, changed_by, UPPER(field), COALESCE(old_value,'<unset>'), COALESCE(new_value,'<unset>'), changed_at
		 FROM task_history WHERE task_id = ? ORDER BY changed_at ASC, id ASC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.TaskHistory
	for rows.Next() {
		var h models.TaskHistory
		if err := rows.Scan(&h.ID, &h.TaskID, &h.ChangedBy, &h.Field, &h.OldValue, &h.NewValue, &h.ChangedAt); err != nil {
			return nil, err
		}
		out = append(out, &h)
	}
	return out, rows.Err()
}
