package handler

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"shelfswap/store"
)

// AdminUsers GET /api/admin/users?q=
func (h *Handler) AdminUsers(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.ListUsers(r.Context(), strings.TrimSpace(r.URL.Query().Get("q")))
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type banReq struct {
	Banned bool `json:"banned"`
}

// AdminBan PUT /api/admin/users/{id}/ban  body {"banned": true|false}
// หลังแบน คำขอถัดไปของคนนั้นจะได้ 403 ทันที เพราะ RequireLogin ถาม DB ทุกครั้ง
func (h *Handler) AdminBan(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req banReq
	if !decode(w, r, &req) {
		return
	}
	u, err := h.Store.SetBanned(r.Context(), id, req.Banned)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "ไม่พบผู้ใช้นี้")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "CANNOT_BAN_ADMIN", "แบนผู้ดูแลระบบไม่ได้")
	case err != nil:
		internal(w, err)
	default:
		writeJSON(w, http.StatusOK, u)
	}
}

var slugRe = regexp.MustCompile(`^[a-z0-9-]{1,50}$`)

type categoryReq struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (c *categoryReq) validate(w http.ResponseWriter) bool {
	c.Name, c.Slug = strings.TrimSpace(c.Name), strings.TrimSpace(c.Slug)
	if c.Name == "" || utf8.RuneCountInString(c.Name) > 50 {
		writeError(w, http.StatusBadRequest, "BAD_NAME", "ชื่อหมวดต้องมี และยาวไม่เกิน 50 ตัวอักษร")
		return false
	}
	if !slugRe.MatchString(c.Slug) {
		writeError(w, http.StatusBadRequest, "BAD_SLUG", "slug ใช้ได้เฉพาะ a-z 0-9 และ - เช่น textbooks")
		return false
	}
	return true
}

func categoryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "CATEGORY_NOT_FOUND", "ไม่พบหมวดหมู่นี้")
	case errors.Is(err, store.ErrDuplicate):
		writeError(w, http.StatusConflict, "CATEGORY_TAKEN", "มีหมวดชื่อหรือ slug นี้อยู่แล้ว")
	case errors.Is(err, store.ErrInUse):
		writeError(w, http.StatusConflict, "CATEGORY_IN_USE", "หมวดนี้ยังมีประกาศอยู่ ลบไม่ได้")
	default:
		internal(w, err)
	}
}

// AdminCreateCategory POST /api/admin/categories
func (h *Handler) AdminCreateCategory(w http.ResponseWriter, r *http.Request) {
	var req categoryReq
	if !decode(w, r, &req) || !req.validate(w) {
		return
	}
	c, err := h.Store.CreateCategory(r.Context(), req.Name, req.Slug)
	if err != nil {
		categoryError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

// AdminUpdateCategory PUT /api/admin/categories/{id}
func (h *Handler) AdminUpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req categoryReq
	if !decode(w, r, &req) || !req.validate(w) {
		return
	}
	c, err := h.Store.UpdateCategory(r.Context(), id, req.Name, req.Slug)
	if err != nil {
		categoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// AdminDeleteCategory DELETE /api/admin/categories/{id}
func (h *Handler) AdminDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := h.Store.DeleteCategory(r.Context(), id); err != nil {
		categoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
