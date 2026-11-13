package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

var (
	ErrNotFound     = errors.New("not found")                  // 404 (รวมกรณี "มีแต่ไม่ใช่ของคุณ" ไม่บอกว่ามีอยู่)
	ErrEmailTaken   = errors.New("email taken")                // 409
	ErrDuplicate    = errors.New("duplicate")                  // 409
	ErrConflict     = errors.New("state does not allow this")  // 409 สถานะตอนนี้ทำแบบนี้ไม่ได้
	ErrNotAvailable = errors.New("listing not available")      // 409 มีคนจองไปแล้ว/ขายแล้ว
	ErrOwnListing   = errors.New("cannot reserve own listing") // 400
	ErrBadCategory  = errors.New("category does not exist")    // 400
	ErrInUse        = errors.New("still in use")               // 409 ลบหมวดที่ยังมีของไม่ได้
)

func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}
