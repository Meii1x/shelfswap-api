package store

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type Listing struct {
	ID              int64     `json:"id"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	Price           int       `json:"price"`
	Condition       string    `json:"condition"`
	ImageURL        string    `json:"imageUrl"`
	Status          string    `json:"status"`
	RejectReason    string    `json:"rejectReason,omitempty"`
	CategoryID      int64     `json:"categoryId"`
	CategoryName    string    `json:"categoryName"`
	SellerID        int64     `json:"sellerId"`
	SellerName      string    `json:"sellerName"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	PendingRequests int       `json:"pendingRequests"` // ใช้ในหน้า "ขายของฉัน" เท่านั้น ที่อื่นเป็น 0
}

// listingFrom: JOIN หมวดและผู้ขายมาด้วย หน้าเว็บจะได้ไม่ต้องยิง API เพิ่มเพื่อเอาชื่อ
const listingSelect = `
	SELECT l.id, l.title, l.description, l.price, l.condition, l.image_url, l.status, l.reject_reason,
	       l.category_id, c.name, l.seller_id, u.display_name, l.created_at, l.updated_at`

const listingFrom = `
	FROM listings l
	JOIN categories c ON c.id = l.category_id
	JOIN users u ON u.id = l.seller_id`

func scanListing(row pgx.Row, extra ...any) (Listing, error) {
	var l Listing
	dest := append([]any{&l.ID, &l.Title, &l.Description, &l.Price, &l.Condition, &l.ImageURL, &l.Status,
		&l.RejectReason, &l.CategoryID, &l.CategoryName, &l.SellerID, &l.SellerName, &l.CreatedAt, &l.UpdatedAt}, extra...)
	err := row.Scan(dest...)
	if errors.Is(err, pgx.ErrNoRows) {
		return l, ErrNotFound
	}
	return l, err
}

func collectListings(rows pgx.Rows, err error) ([]Listing, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Listing{}
	for rows.Next() {
		l, err := scanListing(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// ============================================================================
// ฝั่งอ่าน (คนที่ 1): หน้าร้าน
// ============================================================================

type SearchParams struct {
	Q          string
	CategoryID int64
	Condition  string
	MinPrice   int
	MaxPrice   int // 0 = ไม่จำกัด
	Sort       string
	Page       int
	PageSize   int
}

// sortSQL เป็น whitelist: ORDER BY ใส่ $1 ไม่ได้ จึงห้ามเอาค่าจากผู้ใช้ไปต่อ SQL ตรง ๆ
var sortSQL = map[string]string{
	"newest":     "l.created_at DESC",
	"oldest":     "l.created_at ASC",
	"price_asc":  "l.price ASC, l.created_at DESC",
	"price_desc": "l.price DESC, l.created_at DESC",
}

// SearchListings คืนของที่ขึ้นหน้าร้าน (available + reserved) และจำนวนทั้งหมดสำหรับแบ่งหน้า
// WHERE ประกอบทีละเงื่อนไข แต่ค่าทุกตัวยังไปทาง $n ไม่มีการต่อสตริงค่าของผู้ใช้
func (s *Store) SearchListings(ctx context.Context, p SearchParams) ([]Listing, int, error) {
	where := []string{"l.status IN ('available', 'reserved')", "u.is_banned = FALSE"}
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.ReplaceAll(cond, "?", "$"+strconv.Itoa(len(args))))
	}
	if p.Q != "" {
		add("(l.title ILIKE '%' || ? || '%' OR l.description ILIKE '%' || ? || '%')", p.Q)
	}
	if p.CategoryID > 0 {
		add("l.category_id = ?", p.CategoryID)
	}
	if p.Condition != "" {
		add("l.condition = ?", p.Condition)
	}
	if p.MinPrice > 0 {
		add("l.price >= ?", p.MinPrice)
	}
	if p.MaxPrice > 0 {
		add("l.price <= ?", p.MaxPrice)
	}
	whereSQL := " WHERE " + strings.Join(where, " AND ")

	var total int
	if err := s.Pool.QueryRow(ctx, `SELECT COUNT(*)`+listingFrom+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	order, ok := sortSQL[p.Sort]
	if !ok {
		order = sortSQL["newest"]
	}
	n := len(args)
	args = append(args, p.PageSize, (p.Page-1)*p.PageSize)
	q := listingSelect + listingFrom + whereSQL + " ORDER BY " + order +
		" LIMIT $" + strconv.Itoa(n+1) + " OFFSET $" + strconv.Itoa(n+2)

	items, err := collectListings(s.Pool.Query(ctx, q, args...))
	return items, total, err
}

// GetListing คืนประกาศทุกสถานะ handler เป็นคนตัดสินว่าใครเห็นได้
func (s *Store) GetListing(ctx context.Context, id int64) (Listing, error) {
	return scanListing(s.Pool.QueryRow(ctx, listingSelect+listingFrom+` WHERE l.id = $1`, id))
}


type ListingInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Price       int    `json:"price"`
	Condition   string `json:"condition"`
	ImageURL    string `json:"imageUrl"`
	CategoryID  int64  `json:"categoryId"`
}

// CreateListing ประกาศใหม่เริ่มที่ pending เสมอ ต้องรอ admin อนุมัติก่อนขึ้นหน้าร้าน
func (s *Store) CreateListing(ctx context.Context, sellerID int64, in ListingInput) (Listing, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO listings (seller_id, category_id, title, description, price, condition, image_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		sellerID, in.CategoryID, in.Title, in.Description, in.Price, in.Condition, in.ImageURL).Scan(&id)
	if pgCode(err) == "23503" {
		return Listing{}, ErrBadCategory
	}
	if err != nil {
		return Listing{}, err
	}
	return s.GetListing(ctx, id)
}

// whyNot ใช้หลัง UPDATE/DELETE ที่ไม่โดนแถวไหนเลย เพื่อแยก 404 กับ 409
// ไม่ใช่ของคุณ = 404 (ไม่บอกว่ามีอยู่) / ของคุณแต่สถานะไม่ให้ทำ = 409
func (s *Store) whyNot(ctx context.Context, id, sellerID int64) error {
	var status string
	err := s.Pool.QueryRow(ctx, `SELECT status FROM listings WHERE id = $1 AND seller_id = $2`, id, sellerID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return ErrConflict
}

// UpdateListing เจ้าของแก้ได้เมื่อยังไม่มีคนจอง/ยังไม่ขาย
// ถ้าเคยโดน reject แก้แล้วจะกลับไปรอ admin ตรวจใหม่
func (s *Store) UpdateListing(ctx context.Context, id, sellerID int64, in ListingInput) (Listing, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE listings
		SET title = $3, description = $4, price = $5, condition = $6, image_url = $7, category_id = $8,
		    status = CASE WHEN status = 'rejected' THEN 'pending' ELSE status END,
		    reject_reason = '', updated_at = now()
		WHERE id = $1 AND seller_id = $2 AND status IN ('pending', 'available', 'rejected')`,
		id, sellerID, in.Title, in.Description, in.Price, in.Condition, in.ImageURL, in.CategoryID)
	if pgCode(err) == "23503" {
		return Listing{}, ErrBadCategory
	}
	if err != nil {
		return Listing{}, err
	}
	if tag.RowsAffected() == 0 {
		return Listing{}, s.whyNot(ctx, id, sellerID)
	}
	return s.GetListing(ctx, id)
}

// DeleteListing ลบได้ทุกสถานะยกเว้นตอนมีคนจองอยู่ (การจองถูกลบตามด้วย ON DELETE CASCADE)
func (s *Store) DeleteListing(ctx context.Context, id, sellerID int64) error {
	tag, err := s.Pool.Exec(ctx,
		`DELETE FROM listings WHERE id = $1 AND seller_id = $2 AND status <> 'reserved'`, id, sellerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return s.whyNot(ctx, id, sellerID)
	}
	return nil
}

// MarkSold ปุ่ม "ขายแล้ว" ต้องแก้ 2 ตารางพร้อมกัน จึงใช้ทรานแซกชัน (ถ้าพังกลางทางจะ Rollback ทั้งคู่)
// การจองที่ยืนยันแล้ว -> completed, ที่ยังรอ -> rejected
func (s *Store) MarkSold(ctx context.Context, id, sellerID int64) (Listing, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Listing{}, err
	}
	defer tx.Rollback(ctx) // Commit แล้ว Rollback จะไม่ทำอะไร

	tag, err := tx.Exec(ctx, `
		UPDATE listings SET status = 'sold', updated_at = now()
		WHERE id = $1 AND seller_id = $2 AND status IN ('available', 'reserved')`, id, sellerID)
	if err != nil {
		return Listing{}, err
	}
	if tag.RowsAffected() == 0 {
		return Listing{}, s.whyNot(ctx, id, sellerID)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE reservations
		SET status = CASE WHEN status = 'accepted' THEN 'completed' ELSE 'rejected' END, decided_at = now()
		WHERE listing_id = $1 AND status IN ('pending', 'accepted')`, id); err != nil {
		return Listing{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Listing{}, err
	}
	return s.GetListing(ctx, id)
}

// ListMyListings ทุกสถานะของผู้ขายคนนี้ พร้อมจำนวนคำขอจองที่รอตอบ
func (s *Store) ListMyListings(ctx context.Context, sellerID int64) ([]Listing, error) {
	rows, err := s.Pool.Query(ctx, listingSelect+`,
		(SELECT COUNT(*) FROM reservations r WHERE r.listing_id = l.id AND r.status = 'pending')`+
		listingFrom+` WHERE l.seller_id = $1 ORDER BY l.created_at DESC`, sellerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Listing{}
	for rows.Next() {
		var l Listing
		var pending int
		if l, err = scanListing(rows, &pending); err != nil {
			return nil, err
		}
		l.PendingRequests = pending
		out = append(out, l)
	}
	return out, rows.Err()
}
