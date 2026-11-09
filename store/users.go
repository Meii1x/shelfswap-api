package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"displayName"`
	Phone        string    `json:"phone"`
	Role         string    `json:"role"`
	IsBanned     bool      `json:"isBanned"`
	CreatedAt    time.Time `json:"createdAt"`
	PasswordHash string    `json:"-"` // json:"-" = ไม่มีวันหลุดออกไปใน JSON
}

const userCols = `id, email, display_name, phone, role, is_banned, created_at, password_hash`

func scanUser(row pgx.Row, extra ...any) (User, error) {
	var u User
	dest := append([]any{&u.ID, &u.Email, &u.DisplayName, &u.Phone, &u.Role, &u.IsBanned, &u.CreatedAt, &u.PasswordHash}, extra...)
	err := row.Scan(dest...)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

func (s *Store) CreateUser(ctx context.Context, email, hash, name string) (User, error) {
	u, err := scanUser(s.Pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3)
		 RETURNING `+userCols, email, hash, name))
	if pgCode(err) == "23505" {
		return u, ErrEmailTaken
	}
	return u, err
}

func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	return scanUser(s.Pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE email = $1`, email))
}

func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return scanUser(s.Pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id))
}

func (s *Store) UpdateProfile(ctx context.Context, id int64, name, phone string) (User, error) {
	return scanUser(s.Pool.QueryRow(ctx,
		`UPDATE users SET display_name = $2, phone = $3 WHERE id = $1 RETURNING `+userCols, id, name, phone))
}

func (s *Store) UpdatePassword(ctx context.Context, id int64, hash string) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, id, hash)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// AuthStatus ให้ middleware ใช้: role ปัจจุบันกับสถานะแบนจากฐานข้อมูล (ไม่ใช่จากตั๋ว)
func (s *Store) AuthStatus(ctx context.Context, id int64) (string, bool, error) {
	var role string
	var banned bool
	err := s.Pool.QueryRow(ctx, `SELECT role, is_banned FROM users WHERE id = $1`, id).Scan(&role, &banned)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, ErrNotFound
	}
	return role, banned, err
}
