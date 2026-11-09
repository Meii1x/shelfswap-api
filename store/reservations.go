package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type Reservation struct {
	ID            int64      `json:"id"`
	ListingID     int64      `json:"listingId"`
	ListingTitle  string     `json:"listingTitle"`
	ListingImage  string     `json:"listingImage"`
	ListingPrice  int        `json:"listingPrice"`
	ListingStatus string     `json:"listingStatus"`
	BuyerID       int64      `json:"buyerId"`
	BuyerName     string     `json:"buyerName"`
	BuyerPhone    string     `json:"buyerPhone"` // ว่างจนกว่าการจองจะถูกยืนยัน
	SellerID      int64      `json:"sellerId"`
	SellerName    string     `json:"sellerName"`
	SellerPhone   string     `json:"sellerPhone"` // ว่างจนกว่าการจองจะถูกยืนยัน
	Message       string     `json:"message"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"createdAt"`
	DecidedAt     *time.Time `json:"decidedAt"` // pointer เพราะเป็น NULL ได้
}

// เบอร์โทรเปิดให้เห็นเฉพาะตอน accepted/completed: SQL ตัดสินให้เลย ไม่ต้องพึ่ง React ซ่อน
const reservationSelect = `
	SELECT r.id, r.listing_id, l.title, l.image_url, l.price, l.status,
	       r.buyer_id, b.display_name,
	       CASE WHEN r.status IN ('accepted', 'completed') THEN b.phone ELSE '' END,
	       l.seller_id, s.display_name,
	       CASE WHEN r.status IN ('accepted', 'completed') THEN s.phone ELSE '' END,
	       r.message, r.status, r.created_at, r.decided_at
	FROM reservations r
	JOIN listings l ON l.id = r.listing_id
	JOIN users b ON b.id = r.buyer_id
	JOIN users s ON s.id = l.seller_id`

func scanReservation(row pgx.Row) (Reservation, error) {
	var r Reservation
	err := row.Scan(&r.ID, &r.ListingID, &r.ListingTitle, &r.ListingImage, &r.ListingPrice, &r.ListingStatus,
		&r.BuyerID, &r.BuyerName, &r.BuyerPhone, &r.SellerID, &r.SellerName, &r.SellerPhone,
		&r.Message, &r.Status, &r.CreatedAt, &r.DecidedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

func (s *Store) getReservation(ctx context.Context, id int64) (Reservation, error) {
	return scanReservation(s.Pool.QueryRow(ctx, reservationSelect+` WHERE r.id = $1`, id))
}

func (s *Store) listReservations(ctx context.Context, where string, arg any) ([]Reservation, error) {
	rows, err := s.Pool.Query(ctx, reservationSelect+` WHERE `+where+`
		ORDER BY (r.status = 'pending') DESC, r.created_at DESC`, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Reservation{}
	for rows.Next() {
		r, err := scanReservation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Reserve จองของ 1 ชิ้น: หัวใจของคลิปคนที่ 2
//
// ปัญหา: สองคนกดจองพร้อมกัน ถ้า "อ่านสถานะ -> เช็ก -> เขียน" แยกกัน ทั้งคู่จะเห็น available แล้วจองซ้อนกัน
// วิธีแก้: SELECT ... FOR UPDATE ล็อกแถวประกาศไว้ในทรานแซกชัน คนที่สองต้องรอจนคนแรก Commit
// พอได้ล็อกก็จะเห็นสถานะใหม่ (reserved) แล้วถูกปฏิเสธ
// (ใช้ FOR UPDATE แทน UPDATE ... WHERE status='available' คำสั่งเดียวแบบ Lab 6
//
//	เพราะต้องรู้ seller_id มาเช็กว่า "จองของตัวเอง" ด้วย และต้อง INSERT การจองในทรานแซกชันเดียวกัน)
//
// ด่านสุดท้ายคือ unique index uq_reservations_active ใน schema.sql
func (s *Store) Reserve(ctx context.Context, listingID, buyerID int64, message string) (Reservation, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Reservation{}, err
	}
	defer tx.Rollback(ctx)

	var sellerID int64
	var status string
	err = tx.QueryRow(ctx, `SELECT seller_id, status FROM listings WHERE id = $1 FOR UPDATE`, listingID).
		Scan(&sellerID, &status)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Reservation{}, ErrNotFound
	case err != nil:
		return Reservation{}, err
	case sellerID == buyerID:
		return Reservation{}, ErrOwnListing
	case status == "pending" || status == "rejected":
		return Reservation{}, ErrNotFound // ยังไม่ขึ้นหน้าร้าน ทำเหมือนไม่มี
	case status != "available":
		return Reservation{}, ErrNotAvailable
	}

	if _, err := tx.Exec(ctx, `UPDATE listings SET status = 'reserved', updated_at = now() WHERE id = $1`, listingID); err != nil {
		return Reservation{}, err
	}
	var id int64
	err = tx.QueryRow(ctx,
		`INSERT INTO reservations (listing_id, buyer_id, message) VALUES ($1, $2, $3) RETURNING id`,
		listingID, buyerID, message).Scan(&id)
	if pgCode(err) == "23505" {
		return Reservation{}, ErrNotAvailable
	}
	if err != nil {
		return Reservation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Reservation{}, err
	}
	return s.getReservation(ctx, id)
}

// Decide เปลี่ยนสถานะการจอง
//   - "accept" / "reject": ผู้ขายเท่านั้น และการจองต้องยัง pending
//   - "cancel": ผู้ซื้อเท่านั้น และการจองต้องยัง pending
//
// reject/cancel คืนของกลับไป available ในทรานแซกชันเดียวกัน
func (s *Store) Decide(ctx context.Context, resID, userID int64, action string) (Reservation, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Reservation{}, err
	}
	defer tx.Rollback(ctx)

	var listingID, buyerID, sellerID int64
	var status string
	err = tx.QueryRow(ctx, `
		SELECT r.listing_id, r.buyer_id, l.seller_id, r.status
		FROM reservations r JOIN listings l ON l.id = r.listing_id
		WHERE r.id = $1
		FOR UPDATE OF r, l`, resID).Scan(&listingID, &buyerID, &sellerID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Reservation{}, ErrNotFound
	}
	if err != nil {
		return Reservation{}, err
	}

	// ไม่ใช่คู่กรณีของการจองนี้ = ทำเหมือนไม่มีการจองนี้อยู่
	isSeller, isBuyer := userID == sellerID, userID == buyerID
	if (action == "cancel" && !isBuyer) || (action != "cancel" && !isSeller) {
		return Reservation{}, ErrNotFound
	}
	if status != "pending" {
		return Reservation{}, ErrConflict
	}

	newStatus := map[string]string{"accept": "accepted", "reject": "rejected", "cancel": "cancelled"}[action]
	if _, err := tx.Exec(ctx,
		`UPDATE reservations SET status = $2, decided_at = now() WHERE id = $1`, resID, newStatus); err != nil {
		return Reservation{}, err
	}
	if newStatus != "accepted" {
		if _, err := tx.Exec(ctx,
			`UPDATE listings SET status = 'available', updated_at = now() WHERE id = $1 AND status = 'reserved'`,
			listingID); err != nil {
			return Reservation{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Reservation{}, err
	}
	return s.getReservation(ctx, resID)
}

// ListByBuyer = "การจองของฉัน"
func (s *Store) ListByBuyer(ctx context.Context, buyerID int64) ([]Reservation, error) {
	return s.listReservations(ctx, `r.buyer_id = $1`, buyerID)
}

// ListBySeller = "คำขอจอง" ที่คนอื่นส่งมาหาของของฉัน
func (s *Store) ListBySeller(ctx context.Context, sellerID int64) ([]Reservation, error) {
	return s.listReservations(ctx, `l.seller_id = $1`, sellerID)
}
