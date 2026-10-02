package booking_test

import (
	"context"
	"testing"
	"time"
)

// S1 = C1 (สไลด์ 7 ซ้าย): มี 7 คน 12:00–13:00 ขอ 5 คน 12:30–13:00 → 409 พร้อม available 3, full_from 12:30
func TestCreateS1SeatCapacityExceeded(t *testing.T) {
	e := newEnv(t)
	owner, other, me := e.user("owner"), e.user("other"), e.user("me")
	r := e.restaurant(owner)
	e.existing(r, other, day11("12:00"), day11("13:00"), 7)

	_, err := e.book(me, r, 5, day11("12:30"), day11("13:00"))
	wantType(t, err, "seat-capacity-exceeded")
	p := problemOf(t, err)
	if p.Extra["available"] != 3 || p.Extra["full_from"] != "2026-10-11T12:30:00+07:00" {
		t.Errorf("extra = %v, want available 3, full_from 12:30+07:00", p.Extra)
	}
}

// S2 = C2 (สไลด์ 7 ขวา): 7 คน 12:00–12:30 และ 7 คน 12:30–13:00 ขอ 3 คน 12:00–13:00 → สำเร็จ
func TestCreateS2HalfOpen(t *testing.T) {
	e := newEnv(t)
	owner, a, b, me := e.user("owner"), e.user("a"), e.user("b"), e.user("me")
	r := e.restaurant(owner)
	e.existing(r, a, day11("12:00"), day11("12:30"), 7)
	e.existing(r, b, day11("12:30"), day11("13:00"), 7)

	got, err := e.book(me, r, 3, day11("12:00"), day11("13:00"))
	wantType(t, err, "")
	if got.ID == 0 || got.Status != "active" {
		t.Errorf("booking = %+v", got)
	}
}

// S3–S4 = C3–C4 (สไลด์ 8) ในรูปการจองใหม่: เหลือ B 3 คน 12:00–13:00
// ขอ 8 คน → 409 / ขอ 5 คน → สำเร็จ (กรณีแก้ไขการจองจริงอยู่ในขั้น 4)
func TestCreateS3S4(t *testing.T) {
	for _, tc := range []struct {
		name string
		n    int
		want string
	}{
		{"S3 8 คน", 8, "seat-capacity-exceeded"},
		{"S4 5 คน", 5, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			owner, b, me := e.user("owner"), e.user("b"), e.user("me")
			r := e.restaurant(owner)
			e.existing(r, b, day11("12:00"), day11("13:00"), 3)

			_, err := e.book(me, r, tc.n, day11("12:00"), day11("13:00"))
			wantType(t, err, tc.want)
		})
	}
}

// S5: ไม่นับการจองที่ยกเลิกแล้ว
func TestCreateS5IgnoresCancelled(t *testing.T) {
	e := newEnv(t)
	owner, a, b, me := e.user("owner"), e.user("a"), e.user("b"), e.user("me")
	r := e.restaurant(owner)
	e.existing(r, a, day11("12:00"), day11("13:00"), 7)
	e.cancelled(r, b, day11("12:00"), day11("13:00"), 5)

	_, err := e.book(me, r, 3, day11("12:00"), day11("13:00"))
	wantType(t, err, "")
}

// S6: ขอ 11 คนในร้าน 10 ที่
func TestCreateS6PartyExceedsCapacity(t *testing.T) {
	e := newEnv(t)
	r := e.restaurant(e.user("owner"))
	_, err := e.book(e.user("me"), r, 11, day11("12:00"), day11("13:00"))
	wantType(t, err, "party-exceeds-capacity")
}

// S7: เจ้าของจองร้านตัวเอง
func TestCreateS7OwnRestaurant(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	r := e.restaurant(owner)
	_, err := e.book(owner, r, 2, day11("12:00"), day11("13:00"))
	wantType(t, err, "own-restaurant-booking")
}

// S8: จองทับเวลาตัวเองในร้านเดียวกัน 409 / ต่อกันพอดีไม่ทับ ✓ / ผู้ใช้อื่นทับ ✓
func TestCreateS8OverlappingOwnBooking(t *testing.T) {
	e := newEnv(t)
	owner, me, other := e.user("owner"), e.user("me"), e.user("other")
	r := e.restaurant(owner)

	_, err := e.book(me, r, 2, day11("12:00"), day11("13:00"))
	wantType(t, err, "")

	_, err = e.book(me, r, 2, day11("12:30"), day11("13:30"))
	wantType(t, err, "overlapping-own-booking")

	_, err = e.book(me, r, 2, day11("13:00"), day11("14:00")) // half-open: ต่อกันพอดีไม่ทับ
	wantType(t, err, "")

	_, err = e.book(other, r, 2, day11("12:30"), day11("13:30"))
	wantType(t, err, "")

	// ร้านอื่นในเวลาเดียวกันไม่ถือว่าทับ
	r2 := e.restaurant(owner)
	_, err = e.book(me, r2, 2, day11("12:00"), day11("13:00"))
	wantType(t, err, "")
}

// S16: ร้านพักรับจองทั้งที่ที่นั่งพอ
func TestCreateS16NotAcceptingBookings(t *testing.T) {
	e := newEnv(t)
	r := e.restaurant(e.user("owner"))
	e.exec(`UPDATE restaurants SET accepting_bookings = false WHERE id = $1`, r)

	_, err := e.book(e.user("me"), r, 2, day11("12:00"), day11("13:00"))
	wantType(t, err, "restaurant-not-accepting-bookings")
}

// ร้านที่ไม่มีอยู่และร้านที่ถูกลบ → 404 (lockRestaurant กรอง deleted_at)
func TestCreateRestaurantNotFound(t *testing.T) {
	e := newEnv(t)
	me := e.user("me")
	r := e.restaurant(e.user("owner"))
	e.exec(`UPDATE restaurants SET deleted_at = now() WHERE id = $1`, r)

	for _, id := range []int64{r, 999999} {
		_, err := e.book(me, id, 2, day11("12:00"), day11("13:00"))
		wantType(t, err, "not-found")
	}
}

// กติกาเวลาถูกต่อเข้ากับ pipeline ตามลำดับ (กรณีละเอียดอยู่ใน rules_test.go)
func TestCreateTimeRulesWired(t *testing.T) {
	tests := []struct {
		name       string
		size       int
		start, end time.Time
		want       string
	}{
		{"รูปแบบผิด", 2, day11("12:15"), day11("13:00"), "validation"},
		{"party_size 0", 0, day11("12:00"), day11("13:00"), "validation"},
		{"ผ่านไปแล้ว", 2, at("2026-10-10 07:30"), at("2026-10-10 08:30"), "booking-in-past"},
		{"ล่วงหน้าไม่พอ", 2, at("2026-10-10 09:30"), at("2026-10-10 10:30"), "lead-time-not-met"},
		{"เกิน 30 วัน", 2, at("2026-11-10 12:00"), at("2026-11-10 13:00"), "too-far-in-advance"},
		{"นอกเวลาเปิด", 2, day11("21:30"), day11("22:30"), "outside-opening-hours"},
	}
	e := newEnv(t)
	me := e.user("me")
	r := e.restaurant(e.user("owner"))
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := e.book(me, r, tc.size, tc.start, tc.end)
			wantType(t, err, tc.want)
		})
	}
}

// แถวที่บันทึก: cancel_deadline = start − 30 นาที, status active, เวลาเป็น +07:00
func TestCreateStoresBooking(t *testing.T) {
	e := newEnv(t)
	me := e.user("me")
	r := e.restaurant(e.user("owner"))

	got, err := e.book(me, r, 4, day11("12:00"), day11("13:00"))
	wantType(t, err, "")
	if !got.CancelDeadline.Equal(day11("11:30")) {
		t.Errorf("cancel_deadline = %v, want 11:30", got.CancelDeadline)
	}
	if _, off := got.StartAt.Zone(); off != 7*3600 {
		t.Errorf("start_at offset = %d, want +07:00", off)
	}

	var userID int64
	var deadline time.Time
	var status string
	err = e.pool.QueryRow(context.Background(),
		`SELECT user_id, cancel_deadline, status FROM bookings WHERE id = $1`, got.ID).
		Scan(&userID, &deadline, &status)
	if err != nil {
		t.Fatal(err)
	}
	if userID != me || !deadline.Equal(day11("11:30")) || status != "active" {
		t.Errorf("row = user %d, deadline %v, status %s", userID, deadline, status)
	}
}
