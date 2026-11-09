// Package store คือชั้นเดียวที่เขียน SQL ทุกค่าที่มาจากผู้ใช้ส่งผ่าน $1 $2 ... เสมอ (กัน SQL injection)
package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

// error ภาษาของเรา handler แปลงเป็นสถานะ HTTP (ดูตารางใน handler/handler.go)
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

// pgCode คืนรหัส error ของ Postgres เช่น 23505 (ซ้ำ UNIQUE), 23503 (ติด FOREIGN KEY), 23514 (ผิด CHECK)
func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}
