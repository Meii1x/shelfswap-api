package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type AdminUser struct {
	User
	ListingCount int `json:"listingCount"`
}

// ListUsers ค้นด้วยอีเมลหรือชื่อ (q ว่าง = ทั้งหมด)
func (s *Store) ListUsers(ctx context.Context, q string) ([]AdminUser, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT `+userCols+`, (SELECT COUNT(*) FROM listings l WHERE l.seller_id = users.id)
		FROM users
		WHERE $1 = '' OR email ILIKE '%' || $1 || '%' OR display_name ILIKE '%' || $1 || '%'
		ORDER BY id`, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AdminUser{}
	for rows.Next() {
		var a AdminUser
		if a.User, err = scanUser(rows, &a.ListingCount); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// SetBanned แบน/ปลดแบน แบน admin ไม่ได้ (กัน admin แบนกันเองจนไม่เหลือใครเข้าหลังบ้าน)
func (s *Store) SetBanned(ctx context.Context, id int64, banned bool) (User, error) {
	u, err := scanUser(s.Pool.QueryRow(ctx,
		`UPDATE users SET is_banned = $2 WHERE id = $1 AND role <> 'admin' RETURNING `+userCols, id, banned))
	if errors.Is(err, ErrNotFound) {
		if _, err := s.UserByID(ctx, id); err == nil {
			return u, ErrConflict // มีอยู่แต่เป็น admin
		}
	}
	return u, err
}

type Stats struct {
	Users               int `json:"users"`
	BannedUsers         int `json:"bannedUsers"`
	Categories          int `json:"categories"`
	PendingListings     int `json:"pendingListings"`
	AvailableListings   int `json:"availableListings"`
	ReservedListings    int `json:"reservedListings"`
	SoldListings        int `json:"soldListings"`
	PendingReservations int `json:"pendingReservations"`
}

// Stats นับทุกอย่างในคำสั่งเดียว (FILTER = นับเฉพาะแถวที่เข้าเงื่อนไข)
func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var st Stats
	err := s.Pool.QueryRow(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM users),
		  (SELECT COUNT(*) FROM users WHERE is_banned),
		  (SELECT COUNT(*) FROM categories),
		  COUNT(*) FILTER (WHERE status = 'pending'),
		  COUNT(*) FILTER (WHERE status = 'available'),
		  COUNT(*) FILTER (WHERE status = 'reserved'),
		  COUNT(*) FILTER (WHERE status = 'sold'),
		  (SELECT COUNT(*) FROM reservations WHERE status = 'pending')
		FROM listings`).Scan(&st.Users, &st.BannedUsers, &st.Categories, &st.PendingListings,
		&st.AvailableListings, &st.ReservedListings, &st.SoldListings, &st.PendingReservations)
	return st, err
}

// AdminListings ประกาศทุกคน กรองตามสถานะได้ (status ว่าง = ทุกสถานะ) ของเก่าที่รอนานสุดขึ้นก่อน
func (s *Store) AdminListings(ctx context.Context, status string) ([]Listing, error) {
	return collectListings(s.Pool.Query(ctx, listingSelect+listingFrom+`
		WHERE $1 = '' OR l.status = $1
		ORDER BY l.created_at ASC`, status))
}

// ReviewListing อนุมัติ (-> available) หรือไม่อนุมัติ (-> rejected + เหตุผล) ได้เฉพาะประกาศที่ pending
func (s *Store) ReviewListing(ctx context.Context, id int64, approve bool, reason string) (Listing, error) {
	newStatus := "rejected"
	if approve {
		newStatus = "available"
		reason = ""
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE listings SET status = $2, reject_reason = $3, updated_at = now()
		WHERE id = $1 AND status = 'pending'`, id, newStatus, reason)
	if err != nil {
		return Listing{}, err
	}
	if tag.RowsAffected() == 0 {
		var x int
		if err := s.Pool.QueryRow(ctx, `SELECT 1 FROM listings WHERE id = $1`, id).Scan(&x); errors.Is(err, pgx.ErrNoRows) {
			return Listing{}, ErrNotFound
		}
		return Listing{}, ErrConflict
	}
	return s.GetListing(ctx, id)
}
