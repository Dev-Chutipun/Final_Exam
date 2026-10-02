package booking_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"devpool/internal/booking"
	"devpool/internal/httpx"
	"devpool/internal/testdb"
)

// ค่ากลางของ 01 §10: ร้าน capacity 10 เปิด 10:00–22:00, lead 120, cancel 30,
// now = 2026-10-10 08:00 (+07:00), วันที่จอง 2026-10-11
var testNow = at("2026-10-10 08:00")

// at แปลง "2026-10-11 12:00" (เวลา Bangkok) เป็น time.Time
func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, booking.Bangkok)
	if err != nil {
		panic(err)
	}
	return t
}

// day11 แปลง "12:00" เป็นเวลาของวันที่ 11 ต.ค. 2026
func day11(hm string) time.Time { return at("2026-10-11 " + hm) }

type env struct {
	t           *testing.T
	pool        *pgxpool.Pool
	svc         *booking.Service
	restaurants int
}

func newEnv(t *testing.T) *env {
	pool := testdb.New(t)
	return &env{t: t, pool: pool, svc: &booking.Service{Pool: pool}}
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

// restaurant สร้างร้านค่ากลาง (capacity 10, 10:00–22:00) ของ owner
func (e *env) restaurant(owner int64) int64 {
	e.t.Helper()
	e.restaurants++
	var id int64
	err := e.pool.QueryRow(context.Background(), `
		INSERT INTO restaurants (owner_id, name, description, capacity, open_time, close_time)
		VALUES ($1, $2, 'test', 10, '10:00', '22:00')
		RETURNING id`, owner, fmt.Sprintf("ร้านทดสอบ %d", e.restaurants)).Scan(&id)
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *env) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.pool.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatal(err)
	}
}

// existing ใส่การจอง active ลงฐานข้อมูลตรงๆ (ข้ามกติกา) เพื่อจัดสถานการณ์
func (e *env) existing(restaurantID, userID int64, start, end time.Time, size int) {
	e.t.Helper()
	e.exec(`
		INSERT INTO bookings (restaurant_id, user_id, party_size, start_at, end_at, cancel_deadline)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		restaurantID, userID, size, start, end, booking.CancelDeadline(start, 30))
}

func (e *env) cancelled(restaurantID, userID int64, start, end time.Time, size int) {
	e.t.Helper()
	e.exec(`
		INSERT INTO bookings (restaurant_id, user_id, party_size, start_at, end_at, cancel_deadline, status, cancelled_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'cancelled', $7)`,
		restaurantID, userID, size, start, end, booking.CancelDeadline(start, 30), testNow)
}

func (e *env) book(userID, restaurantID int64, size int, start, end time.Time) (booking.Booking, error) {
	return e.svc.Create(context.Background(), userID, restaurantID,
		booking.Request{PartySize: size, StartAt: start, EndAt: end}, testNow)
}

// activeIntervals คืนการจอง active ทั้งหมดของร้าน ใช้ตรวจ PeakAll หลัง test
func (e *env) activeIntervals(restaurantID int64) []booking.Interval {
	e.t.Helper()
	rows, err := e.pool.Query(context.Background(),
		`SELECT start_at, end_at, party_size FROM bookings WHERE restaurant_id = $1 AND status = 'active'`,
		restaurantID)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var all []booking.Interval
	for rows.Next() {
		var iv booking.Interval
		if err := rows.Scan(&iv.Start, &iv.End, &iv.Size); err != nil {
			e.t.Fatal(err)
		}
		all = append(all, iv)
	}
	return all
}

// problemOf คืน *httpx.Problem จาก err (nil ถ้า err เป็น nil)
func problemOf(t *testing.T, err error) *httpx.Problem {
	t.Helper()
	if err == nil {
		return nil
	}
	var p *httpx.Problem
	if !errors.As(err, &p) {
		t.Fatalf("error ไม่ใช่ problem: %v", err)
	}
	return p
}

// wantType ตรวจว่า err เป็น problem type ที่คาด ("" = ต้องสำเร็จ)
func wantType(t *testing.T, err error, want string) {
	t.Helper()
	got := ""
	if p := problemOf(t, err); p != nil {
		got = p.Type
	}
	if got != want {
		t.Fatalf("got %q (%v), want %q", got, err, want)
	}
}
