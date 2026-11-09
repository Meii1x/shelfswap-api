// Package handler แปล HTTP <-> store: อ่าน/ตรวจ input, เรียก store, เลือกสถานะ HTTP ให้ถูก
//
// ตาราง error -> สถานะ ที่ใช้ทั้งโปรเจกต์
//
//	ข้อมูลที่ส่งมาผิด / ผิดกติกา       400  BAD_JSON, BAD_xxx, OWN_LISTING, BAD_CATEGORY
//	ไม่มีตั๋ว / ตั๋วเสีย / รหัสผิด      401  NO_TOKEN, BAD_TOKEN, BAD_CREDENTIALS
//	มีตั๋วแต่ไม่มีสิทธิ์ / ถูกแบน      403  FORBIDDEN, ACCOUNT_BANNED
//	ไม่มี หรือ "มีแต่ไม่ใช่ของคุณ"     404  xxx_NOT_FOUND
//	ขัดกับสถานะ/ข้อมูลที่มีอยู่        409  EMAIL_TAKEN, NOT_AVAILABLE, WRONG_STATE, IN_USE
//	อื่น ๆ (DB ล่ม ฯลฯ)               500  INTERNAL (รายละเอียดอยู่ใน log เท่านั้น)
package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"shelfswap/auth"
	"shelfswap/mw"
	"shelfswap/store"
)

type Handler struct {
	Store   *store.Store
	Tickets *auth.Issuer
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// รูปแบบ error เดียวกันทุก endpoint: {"error": {"code": "...", "message": "..."}}
// React อ่าน data.error.message ไปโชว์ และใช้ code ตัดสินใจได้
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

// internal จดรายละเอียดไว้ใน log ฝั่งเรา แต่บอกผู้ใช้แค่ว่าระบบขัดข้อง (ไม่เผยชื่อตาราง/SQL)
func internal(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "INTERNAL", "ระบบขัดข้อง ลองใหม่อีกครั้ง")
}

// decode อ่าน body JSON จำกัดขนาด 1 MB กันคนส่งไฟล์ยักษ์มาถ่วง server
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", "อ่านข้อมูล JSON ไม่ออก")
		return false
	}
	return true
}

// pathID อ่านเลขจาก path เช่น /api/listings/{id} ถ้าไม่ใช่เลขบวกตอบ 400 ให้เลย
func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "BAD_ID", "รหัสใน URL ต้องเป็นตัวเลข")
		return 0, false
	}
	return id, true
}

// me คืนตัวตนที่ RequireLogin ใส่ไว้ เลขผู้ใช้มาจากตั๋วเท่านั้น ห้ามรับจาก body/query
func me(r *http.Request) mw.Who {
	who, _ := mw.WhoFrom(r.Context())
	return who
}

// Health ให้ Render เช็กว่า server ยังอยู่ และใช้ปลุก server ก่อนอัดคลิป
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.Store.Pool.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "DB_DOWN", "ต่อฐานข้อมูลไม่ได้")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
