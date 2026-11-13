package handler

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"shelfswap/auth"
	"shelfswap/store"
)

// dummyHash ลายนิ้วมือหลอก ใช้เทียบตอนไม่พบอีเมล ให้เสียเวลาเท่ากับกรณีรหัสผิด (กัน timing attack, Lab 12 Stage 7)
var dummyHash, _ = auth.HashPassword("ไม่มีใครใช้รหัสนี้แน่นอน")

// checkPassword ใช้ร่วมกันตอนสมัครและเปลี่ยนรหัส: ขั้นต่ำนับเป็นตัวอักษร (rune) ขั้นสูงนับเป็น byte (เพดาน bcrypt)
func checkNewPassword(pw string) (string, bool) {
	if utf8.RuneCountInString(pw) < 8 || len(pw) > 72 {
		return "รหัสผ่านต้องยาว 8 ตัวอักษรขึ้นไป และไม่เกิน 72 ไบต์", false
	}
	return "", true
}

type authResponse struct {
	Token string     `json:"token"`
	User  store.User `json:"user"`
}

type registerReq struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

// Register สมัครแล้วออกตั๋วให้เลย หน้าเว็บจะได้ไม่ต้องยิง login ซ้ำ
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if !decode(w, r, &req) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	name := strings.TrimSpace(req.DisplayName)
	if !strings.Contains(email, "@") || len(email) > 255 {
		writeError(w, http.StatusBadRequest, "BAD_EMAIL", "รูปแบบอีเมลไม่ถูกต้อง")
		return
	}
	if msg, ok := checkNewPassword(req.Password); !ok {
		writeError(w, http.StatusBadRequest, "BAD_PASSWORD", msg)
		return
	}
	if name == "" || utf8.RuneCountInString(name) > 50 {
		writeError(w, http.StatusBadRequest, "BAD_NAME", "ต้องมีชื่อที่แสดง และยาวไม่เกิน 50 ตัวอักษร")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		internal(w, err)
		return
	}
	u, err := h.Store.CreateUser(r.Context(), email, hash, name)
	if errors.Is(err, store.ErrEmailTaken) {
		writeError(w, http.StatusConflict, "EMAIL_TAKEN", "อีเมลนี้มีบัญชีอยู่แล้ว")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	h.issue(w, http.StatusCreated, u)
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if !decode(w, r, &req) {
		return
	}
	u, err := h.Store.UserByEmail(r.Context(), strings.ToLower(strings.TrimSpace(req.Email)))
	if errors.Is(err, store.ErrNotFound) {
		auth.CheckPassword(dummyHash, req.Password) // เสียเวลาเท่ากับกรณีรหัสผิด
		writeError(w, http.StatusUnauthorized, "BAD_CREDENTIALS", "อีเมลหรือรหัสผ่านไม่ถูกต้อง")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	if !auth.CheckPassword(u.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "BAD_CREDENTIALS", "อีเมลหรือรหัสผ่านไม่ถูกต้อง")
		return
	}
	// เช็กแบนหลังรหัสถูกเท่านั้น คนเดารหัสจะได้ไม่รู้ว่าบัญชีไหนโดนแบน
	if u.IsBanned {
		writeError(w, http.StatusForbidden, "ACCOUNT_BANNED", "บัญชีนี้ถูกระงับการใช้งาน")
		return
	}
	h.issue(w, http.StatusOK, u)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) issue(w http.ResponseWriter, status int, u store.User) {
	token, err := h.Tickets.Issue(u.ID, u.Role)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, status, authResponse{Token: token, User: u})
}
