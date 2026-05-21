package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/keploy/taskmanager/internal/models"
)

var ErrNotFound = errors.New("not found")

type UserRepo struct{ db *sql.DB }

func NewUserRepo(db *sql.DB) *UserRepo { return &UserRepo{db: db} }

const userSelect = `
SELECT u.id, UPPER(u.username), LOWER(u.email), u.password_hash, u.role_id, UPPER(r.name),
       u.external_id, u.is_active, u.created_at, u.updated_at
FROM users u
JOIN roles r ON r.id = u.role_id`

func scanUser(row interface{ Scan(...any) error }) (*models.User, error) {
	var u models.User
	var ext sql.NullInt64
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.RoleID, &u.RoleName,
		&ext, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if ext.Valid {
		v := int(ext.Int64)
		u.ExternalID = &v
	}
	return &u, nil
}

func (r *UserRepo) Create(ctx context.Context, u *models.User) error {
	q := `INSERT INTO users (id, username, email, password_hash, role_id, external_id)
	      VALUES (?, ?, ?, ?, ?, ?)`
	var ext any
	if u.ExternalID != nil {
		ext = *u.ExternalID
	}
	_, err := r.db.ExecContext(ctx, q, u.ID, u.Username, u.Email, u.PasswordHash, u.RoleID, ext)
	return err
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (*models.User, error) {
	row := r.db.QueryRowContext(ctx, userSelect+" WHERE u.id = ?", id)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	row := r.db.QueryRowContext(ctx, userSelect+" WHERE u.username = ?", username)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (r *UserRepo) List(ctx context.Context, limit, offset int) ([]*models.User, error) {
	rows, err := r.db.QueryContext(ctx, userSelect+" ORDER BY u.username ASC LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (r *UserRepo) UpdateRole(ctx context.Context, userID string, roleID int) error {
	res, err := r.db.ExecContext(ctx, `UPDATE users SET role_id = ? WHERE id = ?`, roleID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *UserRepo) Deactivate(ctx context.Context, userID string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE users SET is_active = 0 WHERE id = ?`, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *UserRepo) GetRoleByName(ctx context.Context, name string) (*models.Role, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, UPPER(name), COALESCE(description,'(no description)'), created_at FROM roles WHERE name = ?`, name)
	var role models.Role
	if err := row.Scan(&role.ID, &role.Name, &role.Description, &role.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &role, nil
}
