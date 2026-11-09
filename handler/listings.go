package handler

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"shelfswap/store"
)

var validConditions = map[string]bool{"new": true, "like_new": true, "good": true, "fair": true}

// validateListing ตรวจทุกช่องก่อนถึง DB (DB มี CHECK ซ้ำอีกชั้น แต่ข้อความจากตรงนี้อ่านง่ายกว่า)
func validateListing(in *store.ListingInput) (code, msg string, ok bool) {
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	in.ImageURL = strings.TrimSpace(in.ImageURL)
	switch {
	case utf8.RuneCountInString(in.Title) < 3 || utf8.RuneCountInString(in.Title) > 100:
		return "BAD_TITLE", "ชื่อสินค้าต้องยาว 3-100 ตัวอักษร", false
	case utf8.RuneCountInString(in.Description) > 2000:
		return "BAD_DESCRIPTION", "รายละเอียดยาวได้ไม่เกิน 2000 ตัวอักษร", false
	case in.Price < 0 || in.Price > 1000000:
		return "BAD_PRICE", "ราคาต้องอยู่ระหว่าง 0 ถึง 1,000,000 บาท", false
	case !validConditions[in.Condition]:
		return "BAD_CONDITION", "เลือกสภาพสินค้า: new, like_new, good หรือ fair", false
	case in.ImageURL != "" && !(strings.HasPrefix(in.ImageURL, "https://") || strings.HasPrefix(in.ImageURL, "http://")):
		return "BAD_IMAGE", "ลิงก์รูปต้องขึ้นต้นด้วย http:// หรือ https://", false
	case len(in.ImageURL) > 500:
		return "BAD_IMAGE", "ลิงก์รูปยาวเกินไป", false
	case in.CategoryID <= 0:
		return "BAD_CATEGORY", "กรุณาเลือกหมวดหมู่", false
	}
	return "", "", true
}

// listingError แปล error ของ store ฝั่งประกาศเป็นสถานะ HTTP
func listingError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "LISTING_NOT_FOUND", "ไม่พบประกาศนี้ในรายการของคุณ")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "WRONG_STATE", "สถานะของประกาศตอนนี้ทำรายการนี้ไม่ได้")
	case errors.Is(err, store.ErrBadCategory):
		writeError(w, http.StatusBadRequest, "BAD_CATEGORY", "ไม่มีหมวดหมู่นี้")
	default:
		internal(w, err)
	}
}

// CreateListing POST /api/listings  ผู้ขาย = คนถือตั๋ว (ไม่รับ sellerId จาก body)
func (h *Handler) CreateListing(w http.ResponseWriter, r *http.Request) {
	var in store.ListingInput
	if !decode(w, r, &in) {
		return
	}
	if code, msg, ok := validateListing(&in); !ok {
		writeError(w, http.StatusBadRequest, code, msg)
		return
	}
	l, err := h.Store.CreateListing(r.Context(), me(r).ID, in)
	if err != nil {
		listingError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, l)
}

// UpdateListing PUT /api/listings/{id}  SQL มี AND seller_id = $2 แก้ของคนอื่นจะได้ 404
func (h *Handler) UpdateListing(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in store.ListingInput
	if !decode(w, r, &in) {
		return
	}
	if code, msg, ok := validateListing(&in); !ok {
		writeError(w, http.StatusBadRequest, code, msg)
		return
	}
	l, err := h.Store.UpdateListing(r.Context(), id, me(r).ID, in)
	if err != nil {
		listingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (h *Handler) DeleteListing(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := h.Store.DeleteListing(r.Context(), id, me(r).ID); err != nil {
		listingError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// MarkSold POST /api/listings/{id}/sold
func (h *Handler) MarkSold(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	l, err := h.Store.MarkSold(r.Context(), id, me(r).ID)
	if err != nil {
		listingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

// MyListings GET /api/me/listings
func (h *Handler) MyListings(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.ListMyListings(r.Context(), me(r).ID)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
