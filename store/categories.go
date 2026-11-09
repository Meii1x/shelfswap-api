package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type Category struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	ListingCount int    `json:"listingCount"` // จำนวนของที่ขึ้นหน้าร้านอยู่ (ใช้โชว์ใน FilterSidebar)
}

// ListCategories ใช้ LEFT JOIN เพื่อให้หมวดที่ยังไม่มีของ (นับได้ 0) ยังโผล่มา
func (s *Store) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT c.id, c.name, c.slug,
		       COUNT(l.id) FILTER (WHERE l.status IN ('available', 'reserved'))
		FROM categories c
		LEFT JOIN listings l ON l.category_id = c.id
		GROUP BY c.id
		ORDER BY c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Category{} // ไม่ใช่ nil เพื่อให้ JSON เป็น [] ไม่ใช่ null
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.ListingCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CreateCategory(ctx context.Context, name, slug string) (Category, error) {
	c := Category{Name: name, Slug: slug}
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO categories (name, slug) VALUES ($1, $2) RETURNING id`, name, slug).Scan(&c.ID)
	if pgCode(err) == "23505" {
		return c, ErrDuplicate
	}
	return c, err
}

func (s *Store) UpdateCategory(ctx context.Context, id int64, name, slug string) (Category, error) {
	c := Category{ID: id}
	err := s.Pool.QueryRow(ctx,
		`UPDATE categories SET name = $2, slug = $3 WHERE id = $1 RETURNING name, slug`,
		id, name, slug).Scan(&c.Name, &c.Slug)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return c, ErrNotFound
	case pgCode(err) == "23505":
		return c, ErrDuplicate
	}
	return c, err
}

// DeleteCategory ลบได้เฉพาะหมวดที่ไม่มีประกาศใช้อยู่ FK จะตอบ 23503 กลับมาให้เอง
func (s *Store) DeleteCategory(ctx context.Context, id int64) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM categories WHERE id = $1`, id)
	if pgCode(err) == "23503" {
		return ErrInUse
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
