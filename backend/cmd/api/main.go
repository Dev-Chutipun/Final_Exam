// คำสั่ง api คือ HTTP API ของระบบ (รันบนเครื่องโดยตรง ไม่รันใน Docker)
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
	dbURL, err := config.Require("DATABASE_URL")
	if err != nil {
		log.Fatal(err)
	}
	port, err := config.Require("PORT")
	if err != nil {
		log.Fatal(err)
	}

	pool, err := db.Connect(context.Background(), dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	// handler เป็นที่เดียวที่อ่านเวลาปัจจุบัน แล้วส่ง now ให้ service
	now := time.Now

	mux := http.NewServeMux()
	(&booking.Handler{Svc: &booking.Service{Pool: pool}, Now: now}).Register(mux)
	(&restaurant.Handler{Svc: &restaurant.Service{Pool: pool}, Now: now}).Register(mux)
	(&review.Handler{Svc: &review.Service{Pool: pool}, Now: now}).Register(mux)

	log.Printf("API ฟังที่ :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
