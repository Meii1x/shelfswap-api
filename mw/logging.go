package mw

import (
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

type recorder struct {
	http.ResponseWriter
	status int
}

func (r *recorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Logging จดแค่ method path status เวลา ไม่จด header ใด ๆ เพราะมีตั๋วอยู่ในนั้น
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &recorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %v", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Microsecond))
	})
}

// Recovery กัน handler ตัวไหน panic แล้วพาทั้ง server ล่ม (Lab 5) ตอบ 500 แทน
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				log.Printf("panic: %v\n%s", v, debug.Stack())
				fail(w, http.StatusInternalServerError, "INTERNAL", "ระบบขัดข้อง ลองใหม่อีกครั้ง")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
