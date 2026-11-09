package mw

import (
	"net/http"
	"strings"
)

// CORS อนุญาตเฉพาะ origin ที่อยู่ในรายชื่อ (ALLOWED_ORIGINS คั่นด้วย ,)
// ต่างจาก moviehub: เพิ่ม PATCH เพราะ ShelfSwap ใช้ PATCH ตอบรับการจอง/อนุมัติประกาศ
// ถ้าลืม เบราว์เซอร์จะตอบ "Failed to fetch" ทั้งที่ curl ยิงผ่าน
func CORS(allowed []string, next http.Handler) http.Handler {
	ok := map[string]bool{}
	for _, o := range allowed {
		if o = strings.TrimRight(strings.TrimSpace(o), "/"); o != "" {
			ok[o] = true
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if ok[origin] {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Add("Vary", "Origin")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			h.Set("Access-Control-Max-Age", "600")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
