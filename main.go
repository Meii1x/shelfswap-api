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
	_ "github.com/joho/godotenv/autoload"

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

	login := func(next http.HandlerFunc) http.HandlerFunc { return mw.RequireLogin(tickets, st, next) }
	admin := func(next http.HandlerFunc) http.HandlerFunc { return login(mw.RequireRole("admin", next)) }
	maybe := func(next http.HandlerFunc) http.HandlerFunc { return mw.MaybeLogin(tickets, st, next) }

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", h.Health)

	// ---------- สมาชิก ----------
	mux.HandleFunc("POST /api/auth/register", h.Register)
	mux.HandleFunc("POST /api/auth/login", h.Login)
	mux.HandleFunc("POST /api/auth/logout", h.Logout)
	mux.HandleFunc("GET /api/me", login(h.GetMe))
	mux.HandleFunc("PUT /api/me", login(h.UpdateMe))
	mux.HandleFunc("PUT /api/me/password", login(h.ChangePassword))

	// ---------- หน้าร้าน (Guest เข้าได้) ----------
	mux.HandleFunc("GET /api/categories", h.Categories)
	mux.HandleFunc("GET /api/listings", h.SearchListings)
	mux.HandleFunc("GET /api/listings/{id}", maybe(h.GetListing))

	// ---------- ประกาศของฉัน ----------
	mux.HandleFunc("POST /api/listings", login(h.CreateListing))
	mux.HandleFunc("PUT /api/listings/{id}", login(h.UpdateListing))
	mux.HandleFunc("DELETE /api/listings/{id}", login(h.DeleteListing))
	mux.HandleFunc("POST /api/listings/{id}/sold", login(h.MarkSold))
	mux.HandleFunc("GET /api/me/listings", login(h.MyListings))
	mux.HandleFunc("POST /api/uploads", login(h.UploadImage))

	// ---------- การจอง ----------
	mux.HandleFunc("POST /api/listings/{id}/reserve", login(h.Reserve))
	mux.HandleFunc("PATCH /api/reservations/{id}", login(h.DecideReservation))
	mux.HandleFunc("GET /api/me/reservations", login(h.MyReservations))
	mux.HandleFunc("GET /api/me/requests", login(h.MyRequests))

	// ---------- หลังบ้าน: ทุกเส้นทางใต้ /api/admin ผ่าน admin(...) ไม่มีข้อยกเว้น ----------
	mux.HandleFunc("GET /api/admin/users", admin(h.AdminUsers))
	mux.HandleFunc("PUT /api/admin/users/{id}/ban", admin(h.AdminBan))
	mux.HandleFunc("POST /api/admin/categories", admin(h.AdminCreateCategory))
	mux.HandleFunc("PUT /api/admin/categories/{id}", admin(h.AdminUpdateCategory))
	mux.HandleFunc("DELETE /api/admin/categories/{id}", admin(h.AdminDeleteCategory))
	
	mux.HandleFunc("GET /api/admin/stats", admin(h.AdminStats))
	mux.HandleFunc("GET /api/admin/listings", admin(h.AdminListings))
	mux.HandleFunc("PATCH /api/admin/listings/{id}", admin(h.AdminReviewListing))

	origins := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mw.Logging(mw.Recovery(mw.CORS(origins, mux))),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

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
