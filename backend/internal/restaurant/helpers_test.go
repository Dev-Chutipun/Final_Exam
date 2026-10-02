package restaurant_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"devpool/internal/auth"
	"devpool/internal/booking"
	"devpool/internal/restaurant"
	"devpool/internal/testdb"
)

// ค่ากลางของ 01 §10: now = 2026-10-10 08:00 (+07:00)
var testNow = at("2026-10-10 08:00")

func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, booking.Bangkok)
	if err != nil {
		panic(err)
	}
	return t
}

func day11(hm string) time.Time { return at("2026-10-11 " + hm) }

type env struct {
	t           *testing.T
	pool        *pgxpool.Pool
	svc         *restaurant.Service
	bookings    *booking.Service
	restaurants int
}

func newEnv(t *testing.T) *env {
	pool := testdb.New(t)
	return &env{t: t, pool: pool, svc: &restaurant.Service{Pool: pool}, bookings: &booking.Service{Pool: pool}}
}

func (e *env) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.pool.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) user(name string) int64 {
	e.t.Helper()
	var id int64
	err := e.pool.QueryRow(context.Background(),
		`INSERT INTO users (issuer, subject, display_name) VALUES ('test://local', $1, $1) RETURNING id`,
		name).Scan(&id)
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

// restaurant สร้างร้านค่ากลาง (capacity 10, 10:00–22:00, cancel 30, lead 120)
func (e *env) restaurant(owner int64) int64 {
	e.t.Helper()
	e.restaurants++
	var id int64
	err := e.pool.QueryRow(context.Background(), `
		INSERT INTO restaurants (owner_id, name, description, capacity, open_time, close_time)
		VALUES ($1, $2, 'test', 10, '10:00', '22:00') RETURNING id`,
		owner, fmt.Sprintf("ร้านทดสอบ %d", e.restaurants)).Scan(&id)
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

// existing ใส่การจอง active ลงฐานข้อมูลตรงๆ แล้วคืน id
func (e *env) existing(restaurantID, userID int64, start, end time.Time, size int) int64 {
	e.t.Helper()
	var id int64
	err := e.pool.QueryRow(context.Background(), `
		INSERT INTO bookings (restaurant_id, user_id, party_size, start_at, end_at, cancel_deadline)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		restaurantID, userID, size, start, end, booking.CancelDeadline(start, 30)).Scan(&id)
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

// fakeAuth อ่าน userID จาก header X-Test-User (มีเฉพาะใน _test.go)
func fakeAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v := r.Header.Get("X-Test-User"); v != "" {
			id, _ := strconv.ParseInt(v, 10, 64)
			r = r.WithContext(auth.WithUserID(r.Context(), id))
		}
		next.ServeHTTP(w, r)
	})
}

func (e *env) server() http.Handler {
	mux := http.NewServeMux()
	(&restaurant.Handler{Svc: e.svc, Now: func() time.Time { return testNow }}).Register(mux)
	return fakeAuth(mux)
}

// do ส่งคำขอ JSON แล้วคืน status, type ของ problem และ body
func (e *env) do(method, path string, user int64, body string) (int, string, map[string]any) {
	e.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if user != 0 {
		req.Header.Set("X-Test-User", strconv.FormatInt(user, 10))
	}
	rec := httptest.NewRecorder()
	e.server().ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	typ := ""
	if rec.Header().Get("Content-Type") == "application/problem+json" {
		typ, _ = out["type"].(string)
	}
	return rec.Code, typ, out
}

func itoa(id int64) string { return strconv.FormatInt(id, 10) }
