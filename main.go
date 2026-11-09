package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/joho/godotenv/autoload" // โหลด .env ให้เองถ้ามีไฟล์ (ในเครื่อง) บน Render ไม่มีไฟล์ก็เงียบ

	"shelfswap/auth"
	"shelfswap/handler"
	"shelfswap/mw"
	"shelfswap/store"
)

func main() {
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		log.Fatal("JWT_SECRET ต้องยาวอย่างน้อย 32 ตัวอักษร (สร้างด้วย openssl rand -hex 32)")
	}

	cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("DATABASE_URL ผิดรูปแบบ: ", err)
	}
	cfg.MaxConns = 10
	cfg.MaxConnIdleTime = 5 * time.Minute
	// Supabase "Transaction pooler" (พอร์ต 6543) ไม่รองรับ prepared statement ต้องใช้ simple protocol
	// ถ้าใช้ "Session pooler" (พอร์ต 5432) ไม่ต้องทำอะไร
	if cfg.ConnConfig.Port == 6543 {
		cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatal("ต่อฐานข้อมูลไม่ติด: ", err)
	}

	st := &store.Store{Pool: pool}
	tickets := &auth.Issuer{Secret: []byte(secret), TTL: 24 * time.Hour}
	h := &handler.Handler{Store: st, Tickets: tickets}

	// login = ต้องมีตั๋ว (และไม่ถูกแบน), admin = login แล้วต้องเป็น admin (ตรวจตั๋วก่อนเสมอ ค่อยดูบทบาท)
	// maybe = Guest เข้าได้ แต่ถ้ามีตั๋วจะรู้ว่าเป็นใคร
	login := func(next http.HandlerFunc) http.HandlerFunc { return mw.RequireLogin(tickets, st, next) }
	admin := func(next http.HandlerFunc) http.HandlerFunc { return login(mw.RequireRole("admin", next)) }
	maybe := func(next http.HandlerFunc) http.HandlerFunc { return mw.MaybeLogin(tickets, st, next) }

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", h.Health)

	// ---------- คนที่ 1: สมาชิก ----------
	mux.HandleFunc("POST /api/auth/register", h.Register)
	mux.HandleFunc("POST /api/auth/login", h.Login)
	mux.HandleFunc("POST /api/auth/logout", h.Logout)
	mux.HandleFunc("GET /api/me", login(h.GetMe))
	mux.HandleFunc("PUT /api/me", login(h.UpdateMe))
	mux.HandleFunc("PUT /api/me/password", login(h.ChangePassword))

	// ---------- คนที่ 1: หน้าร้าน (Guest เข้าได้) ----------
	mux.HandleFunc("GET /api/categories", h.Categories)
	mux.HandleFunc("GET /api/listings", h.SearchListings)
	mux.HandleFunc("GET /api/listings/{id}", maybe(h.GetListing))

	// ---------- คนที่ 2: ประกาศของฉัน ----------
	mux.HandleFunc("POST /api/listings", login(h.CreateListing))
	mux.HandleFunc("PUT /api/listings/{id}", login(h.UpdateListing))
	mux.HandleFunc("DELETE /api/listings/{id}", login(h.DeleteListing))
	mux.HandleFunc("POST /api/listings/{id}/sold", login(h.MarkSold))
	mux.HandleFunc("GET /api/me/listings", login(h.MyListings))

	// ---------- คนที่ 2: การจอง ----------
	mux.HandleFunc("POST /api/listings/{id}/reserve", login(h.Reserve))
	mux.HandleFunc("PATCH /api/reservations/{id}", login(h.DecideReservation))
	mux.HandleFunc("GET /api/me/reservations", login(h.MyReservations))
	mux.HandleFunc("GET /api/me/requests", login(h.MyRequests))

	// ---------- หลังบ้าน: ทุกเส้นทางใต้ /api/admin ผ่าน admin(...) ไม่มีข้อยกเว้น ----------
	// คนที่ 1
	mux.HandleFunc("GET /api/admin/users", admin(h.AdminUsers))
	mux.HandleFunc("PUT /api/admin/users/{id}/ban", admin(h.AdminBan))
	mux.HandleFunc("POST /api/admin/categories", admin(h.AdminCreateCategory))
	mux.HandleFunc("PUT /api/admin/categories/{id}", admin(h.AdminUpdateCategory))
	mux.HandleFunc("DELETE /api/admin/categories/{id}", admin(h.AdminDeleteCategory))
	// คนที่ 2
	mux.HandleFunc("GET /api/admin/stats", admin(h.AdminStats))
	mux.HandleFunc("GET /api/admin/listings", admin(h.AdminListings))
	mux.HandleFunc("PATCH /api/admin/listings/{id}", admin(h.AdminReviewListing))

	origins := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")
	port := os.Getenv("PORT") // Render ส่งมาให้ ในเครื่องไม่มีก็ใช้ 8080
	if port == "" {
		port = "8080"
	}

	// ลำดับ middleware: ตัวนอกสุดทำก่อน Logging -> Recovery -> CORS -> mux
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mw.Logging(mw.Recovery(mw.CORS(origins, mux))),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// graceful shutdown: กด Ctrl+C หรือ Render สั่งหยุด ให้คำขอที่ค้างอยู่ทำจนเสร็จก่อน (Lab 5)
	go func() {
		log.Println("ShelfSwap API ฟังอยู่ที่ :"+port, "อนุญาต origin", origins)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Println("กำลังปิด server...")
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Println("ปิดไม่สวย:", err)
	}
}
