package handler

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"shelfswap/auth"
	"shelfswap/store"
)

// GetMe ข้อมูลของคนที่ถือตั๋ว (หน้าเว็บเรียกตอนเปิดแอป เพื่ออัปเดต role/ชื่อล่าสุด)
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	u, err := h.Store.UserByID(r.Context(), me(r).ID)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

var phoneRe = regexp.MustCompile(`^[0-9+\- ]{0,20}$`)

type updateMeReq struct {
	DisplayName string `json:"displayName"`
	Phone       string `json:"phone"`
}

func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	var req updateMeReq
	if !decode(w, r, &req) {
		return
	}
	name, phone := strings.TrimSpace(req.DisplayName), strings.TrimSpace(req.Phone)
	if name == "" || utf8.RuneCountInString(name) > 50 {
		writeError(w, http.StatusBadRequest, "BAD_NAME", "ต้องมีชื่อที่แสดง และยาวไม่เกิน 50 ตัวอักษร")
		return
	}
	if !phoneRe.MatchString(phone) {
		writeError(w, http.StatusBadRequest, "BAD_PHONE", "เบอร์โทรใช้ได้เฉพาะตัวเลข + - และเว้นวรรค ไม่เกิน 20 ตัว")
		return
	}
	u, err := h.Store.UpdateProfile(r.Context(), me(r).ID, name, phone)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

type changePasswordReq struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// ChangePassword ต้องยืนยันรหัสเดิมก่อน กันคนแอบใช้เครื่องที่ล็อกอินค้างไว้
// รหัสเดิมผิดตอบ 400 ไม่ใช่ 401 เพราะ 401 จะทำให้หน้าเว็บคิดว่าตั๋วหมดอายุแล้วเด้งออก
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordReq
	if !decode(w, r, &req) {
		return
	}
	u, err := h.Store.UserByID(r.Context(), me(r).ID)
	if err != nil {
		internal(w, err)
		return
	}
	if !auth.CheckPassword(u.PasswordHash, req.CurrentPassword) {
		writeError(w, http.StatusBadRequest, "WRONG_PASSWORD", "รหัสผ่านปัจจุบันไม่ถูกต้อง")
		return
	}
	if msg, ok := checkNewPassword(req.NewPassword); !ok {
		writeError(w, http.StatusBadRequest, "BAD_PASSWORD", msg)
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		internal(w, err)
		return
	}
	if err := h.Store.UpdatePassword(r.Context(), u.ID, hash); err != nil && !errors.Is(err, store.ErrNotFound) {
		internal(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
