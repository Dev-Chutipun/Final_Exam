package booking_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"devpool/internal/booking"
	"devpool/internal/httpx"
)

func httpxPage(page, size int) httpx.Page { return httpx.Page{Page: page, Size: size} }

func itoa(id int64) string { return strconv.FormatInt(id, 10) }

func (e *env) availability(r int64, date string, now time.Time) (booking.Availability, error) {
	day, err := booking.ParseDate("date", date)
	if err != nil {
		e.t.Fatal(err)
	}
	return e.svc.Availability(context.Background(), r, day, now)
}

func slotAt(t *testing.T, a booking.Availability, hm string) booking.Slot {
	t.Helper()
	for _, s := range a.Slots {
		if s.Start.Format("15:04") == hm {
			return s
		}
	}
	t.Fatalf("ไม่มีช่อง %s", hm)
	return booking.Slot{}
}

// A1–A3: ที่ว่าง limited และ bookable
func TestAvailabilityA1toA3(t *testing.T) {
	e := newEnv(t)
	r := e.restaurant(e.user("owner"))
	e.existing(r, e.user("a"), day11("12:00"), day11("13:00"), 7)  // เหลือ 3
	e.existing(r, e.user("b"), day11("15:00"), day11("15:30"), 6)  // เหลือ 4
	e.existing(r, e.user("c"), day11("18:00"), day11("18:30"), 10) // เหลือ 0

	a, err := e.availability(r, "2026-10-11", testNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Slots) != 24 || a.Capacity != 10 || !a.AcceptingBookings || a.Date != "2026-10-11" {
		t.Fatalf("slots %d capacity %d accepting %v date %s", len(a.Slots), a.Capacity, a.AcceptingBookings, a.Date)
	}
	tests := []struct {
		hm        string
		remaining int
		limited   bool
		bookable  bool
	}{
		{"12:00", 3, true, true},   // A1
		{"12:30", 3, true, true},   // A1
		{"13:00", 10, false, true}, // A1 จบ 13:00 ไม่นับ (half-open)
		{"15:00", 4, false, true},  // A2 เหลือ 4 จาก 10 ไม่ถือว่าเหลือน้อย
		{"18:00", 0, false, false}, // A3 เต็ม
	}
	for _, tc := range tests {
		s := slotAt(t, a, tc.hm)
		if s.Remaining != tc.remaining || s.Limited != tc.limited || s.Bookable != tc.bookable {
			t.Errorf("%s: %+v, want remaining %d limited %v bookable %v", tc.hm, s, tc.remaining, tc.limited, tc.bookable)
		}
	}
	if s := slotAt(t, a, "12:00"); !s.End.Equal(day11("12:30")) {
		t.Errorf("end = %v", s.End)
	}
}

// A4: ช่องที่เร็วกว่า now + lead จองไม่ได้ (lead 180 นาที now 08:00 → จองได้ตั้งแต่ 11:00)
func TestAvailabilityA4LeadTime(t *testing.T) {
	e := newEnv(t)
	r := e.restaurant(e.user("owner"))
	e.exec(`UPDATE restaurants SET min_lead_minutes = 180 WHERE id = $1`, r)

	a, err := e.availability(r, "2026-10-10", testNow)
	if err != nil {
		t.Fatal(err)
	}
	if slotAt(t, a, "10:30").Bookable || !slotAt(t, a, "11:00").Bookable {
		t.Errorf("10:30 bookable %v, 11:00 bookable %v; want false, true",
			slotAt(t, a, "10:30").Bookable, slotAt(t, a, "11:00").Bookable)
	}
}

// A5: ร้านพักรับจอง → bookable false ทุกช่อง
func TestAvailabilityA5NotAccepting(t *testing.T) {
	e := newEnv(t)
	r := e.restaurant(e.user("owner"))
	e.exec(`UPDATE restaurants SET accepting_bookings = false WHERE id = $1`, r)

	a, err := e.availability(r, "2026-10-11", testNow)
	if err != nil {
		t.Fatal(err)
	}
	if a.AcceptingBookings {
		t.Error("accepting_bookings ต้องเป็น false")
	}
	for _, s := range a.Slots {
		if s.Bookable {
			t.Fatalf("%v bookable ทั้งที่ร้านพักรับจอง", s.Start)
		}
	}
}

// A6: ร้านถูกลบ → 404 · วันที่ผิดรูปแบบ → 400 (ผ่าน handler ซึ่งเป็น endpoint สาธารณะ)
func TestAvailabilityA6AndHandler(t *testing.T) {
	e := newEnv(t)
	r := e.restaurant(e.user("owner"))
	path := "/api/restaurants/" + itoa(r) + "/availability"

	if status, _, body := e.do(http.MethodGet, path+"?date=2026-10-11", 0, ""); status != 200 || len(body["slots"].([]any)) != 24 {
		t.Errorf("public: %d", status)
	}
	for _, q := range []string{"", "?date=11-10-2026", "?date=2026-02-30"} {
		if status, typ, _ := e.do(http.MethodGet, path+q, 0, ""); status != 400 || typ != "/problems/validation" {
			t.Errorf("date %q: %d %s", q, status, typ)
		}
	}
	e.exec(`UPDATE restaurants SET deleted_at = now() WHERE id = $1`, r)
	if status, typ, _ := e.do(http.MethodGet, path+"?date=2026-10-11", 0, ""); status != 404 || typ != "/problems/not-found" {
		t.Errorf("deleted: %d %s", status, typ)
	}
}

// ร้านข้ามเที่ยงคืน 18:00–02:00: วันทำการที่ 11 มี 16 ช่อง ช่องสุดท้าย 01:30 ของวันที่ 12
// และการจอง 01:00–02:00 ของวันที่ 12 ถูกนับในวันทำการที่ 11
func TestAvailabilityOvernight(t *testing.T) {
	e := newEnv(t)
	r := e.restaurant(e.user("owner"))
	e.exec(`UPDATE restaurants SET open_time = '18:00', close_time = '02:00' WHERE id = $1`, r)
	e.existing(r, e.user("a"), at("2026-10-12 01:00"), at("2026-10-12 02:00"), 8)

	a, err := e.availability(r, "2026-10-11", testNow)
	if err != nil {
		t.Fatal(err)
	}
	last := a.Slots[len(a.Slots)-1]
	if len(a.Slots) != 16 || !last.Start.Equal(at("2026-10-12 01:30")) || last.Remaining != 2 {
		t.Errorf("slots %d, last %+v", len(a.Slots), last)
	}
}
