// คำสั่ง api คือ HTTP API ของระบบ (รันบนเครื่องโดยตรง ไม่รันใน Docker เพราะ issuer ของ token ต้องตรงกัน)
//
//	cd backend
//	go run ./cmd/api
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	_ "time/tzdata" // ฝังฐานข้อมูล timezone เพื่อโหลด Asia/Bangkok ได้แม้เครื่องไม่มี

	"devpool/internal/auth"
	"devpool/internal/booking"
	"devpool/internal/config"
	"devpool/internal/db"
	"devpool/internal/restaurant"
	"devpool/internal/review"
)

func main() {
	if err := config.Load(); err != nil {
		log.Fatal(err)
	}
	env := func(key string) string {
		v, err := config.Require(key)
		if err != nil {
			log.Fatal(err)
		}
		return v
	}
	publicBaseURL := env("PUBLIC_BASE_URL")
	ctx := context.Background()

	pool, err := db.Connect(ctx, env("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	// ต่อ Keycloak ตอนเริ่มโปรแกรม ถ้าต่อไม่ได้ให้หยุดพร้อมข้อความชัดเจน
	provider, err := auth.NewOIDC(ctx, env("OIDC_ISSUER"), env("OIDC_CLIENT_ID"), env("OIDC_CLIENT_SECRET"), publicBaseURL)
	if err != nil {
		log.Fatal(err)
	}

	// handler เป็นที่เดียวที่อ่านเวลาปัจจุบัน แล้วส่ง now ให้ service
	now := time.Now

	authHandler := &auth.Handler{
		Pool:          pool,
		OAuth:         provider.OAuth,
		Exchanger:     provider,
		EndSessionURL: provider.EndSessionURL,
		PublicBaseURL: publicBaseURL,
		CookieSecure:  env("COOKIE_SECURE") == "true",
		Now:           now,
	}

	mux := http.NewServeMux()
	authHandler.Register(mux)
	(&booking.Handler{Svc: &booking.Service{Pool: pool}, Now: now}).Register(mux)
	(&restaurant.Handler{Svc: &restaurant.Service{Pool: pool}, Now: now}).Register(mux)
	(&review.Handler{Svc: &review.Service{Pool: pool}, Now: now}).Register(mux)

	// ลำดับ: CSRF ตรวจก่อน → middleware อ่าน session → mux
	handler, err := auth.CSRF(publicBaseURL, authHandler.Middleware(mux))
	if err != nil {
		log.Fatal(err)
	}

	port := env("PORT")
	log.Printf("API ฟังที่ :%s · issuer %s", port, env("OIDC_ISSUER"))
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
