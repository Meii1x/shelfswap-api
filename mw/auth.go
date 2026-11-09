// Package mw คือด่านที่คำขอต้องผ่านก่อนถึง handler
package mw

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"shelfswap/auth"
	"shelfswap/store"
)

type ctxKey int

const whoKey ctxKey = 0

// Who คือ "คนที่ถือตั๋ว" ที่ผ่านด่านแล้ว handler อ่านได้ด้วย WhoFrom
type Who struct {
	ID   int64
	Role string
}

// WhoFrom คืนตัวตนจาก context ok = false แปลว่าคำขอนี้เป็น Guest
func WhoFrom(ctx context.Context) (Who, bool) {
	w, ok := ctx.Value(whoKey).(Who)
	return w, ok
}

// UserStatus คือสิ่งที่ด่านต้องถามฐานข้อมูลทุกครั้ง (store.Store ทำให้แล้ว)
// แยกเป็น interface เพื่อให้เทสต์ส่งของปลอมเข้ามาแทนได้
type UserStatus interface {
	AuthStatus(ctx context.Context, id int64) (role string, banned bool, err error)
}

func fail(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

func bearer(r *http.Request) (string, bool) {
	return strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
}

// RequireLogin ตรวจตั๋ว แล้วถามฐานข้อมูลซ้ำว่าคนนี้ยังอยู่ ไม่ถูกแบน และตอนนี้มี role อะไร
//
// ทำไมต้องถาม DB ทุกครั้ง: JWT ที่ออกไปแล้วใช้ได้จนหมดอายุ ถ้าเชื่อ role/สถานะบนตั๋วอย่างเดียว
// admin แบนใครไป คนนั้นก็ยังใช้ตั๋วเดิมได้อีกเป็นชั่วโมง (ปัญหาเดียวกับ role ค้างใน Lab 12)
// ราคาที่จ่ายคือ query เล็ก ๆ 1 ครั้งต่อคำขอ (ใช้ primary key จึงเร็วมาก)
func RequireLogin(iss *auth.Issuer, users UserStatus, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok, ok := bearer(r)
		if !ok {
			fail(w, http.StatusUnauthorized, "NO_TOKEN", "กรุณาเข้าสู่ระบบก่อน")
			return
		}
		id, _, err := iss.Parse(tok)
		if err != nil {
			fail(w, http.StatusUnauthorized, "BAD_TOKEN", "ตั๋วไม่ถูกต้องหรือหมดอายุ กรุณาเข้าสู่ระบบใหม่")
			return
		}
		role, banned, err := users.AuthStatus(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			fail(w, http.StatusUnauthorized, "USER_GONE", "ไม่พบบัญชีนี้แล้ว กรุณาเข้าสู่ระบบใหม่")
			return
		}
		if err != nil {
			fail(w, http.StatusInternalServerError, "INTERNAL", "ระบบขัดข้อง ลองใหม่อีกครั้ง")
			return
		}
		if banned {
			fail(w, http.StatusForbidden, "ACCOUNT_BANNED", "บัญชีนี้ถูกระงับการใช้งาน")
			return
		}
		ctx := context.WithValue(r.Context(), whoKey, Who{ID: id, Role: role})
		next(w, r.WithContext(ctx))
	}
}

// MaybeLogin ใช้กับหน้าที่ Guest เข้าได้ แต่ถ้า login อยู่จะเห็นมากขึ้น (เช่น เจ้าของเห็นประกาศที่รออนุมัติ)
// ตั๋วเสียหรือถูกแบน = ถือเป็น Guest ไม่ตอบ error
func MaybeLogin(iss *auth.Issuer, users UserStatus, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if tok, ok := bearer(r); ok {
			if id, _, err := iss.Parse(tok); err == nil {
				if role, banned, err := users.AuthStatus(r.Context(), id); err == nil && !banned {
					r = r.WithContext(context.WithValue(r.Context(), whoKey, Who{ID: id, Role: role}))
				}
			}
		}
		next(w, r)
	}
}

// RequireRole ใช้ต่อจาก RequireLogin เสมอ: ตั๋วถูกต้องแต่บทบาทไม่ตรง ตอบ 403
func RequireRole(role string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if who, _ := WhoFrom(r.Context()); who.Role != role {
			fail(w, http.StatusForbidden, "FORBIDDEN", "บัญชีนี้ไม่มีสิทธิ์ทำรายการนี้")
			return
		}
		next(w, r)
	}
}
