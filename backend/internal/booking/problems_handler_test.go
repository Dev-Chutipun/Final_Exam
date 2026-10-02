package booking_test

import (
	"fmt"
	"net/http"
	"testing"
)

// 01 §10.9: ทุก problem type ของการจองต้องตอบผ่าน HTTP ได้ถูก status และ type
// (type ของรีวิวและตั้งค่าร้านอยู่ใน handler test ของ package review และ restaurant)
func TestHandlerProblemTypes(t *testing.T) {
	e := newEnv(t)
	owner, me, other := e.user("owner"), e.user("me"), e.user("other")
	r := e.restaurant(owner)
	closed := e.restaurant(owner)
	e.exec(`UPDATE restaurants SET accepting_bookings = false WHERE id = $1`, closed)

	// สถานการณ์: me มีการจอง 12:00–13:00 (2 คน), other จอง 14:00–15:00 เต็ม 10 ที่
	mine := e.mustBook(me, r, 2, day11("12:00"), day11("13:00"))
	e.mustBook(other, r, 10, day11("14:00"), day11("15:00"))
	cancelled := e.mustBook(me, r, 2, day11("16:00"), day11("17:00"))
	if _, err := e.cancel(me, cancelled, testNow); err != nil {
		t.Fatal(err)
	}
	// การจองที่เลย deadline แล้ว: เริ่ม 08:30 deadline 07:30 แต่ now คือ 08:00
	var late int64
	if err := e.pool.QueryRow(t.Context(), `
		INSERT INTO bookings (restaurant_id, user_id, party_size, start_at, end_at, cancel_deadline)
		VALUES ($1, $2, 2, $3, $4, $5) RETURNING id`,
		r, me, at("2026-10-10 08:30"), at("2026-10-10 09:30"), at("2026-10-10 07:30")).Scan(&late); err != nil {
		t.Fatal(err)
	}

	book := func(rid int64) string { return "/api/restaurants/" + itoa(rid) + "/bookings" }
	body := func(n int, start, end string) string {
		return fmt.Sprintf(`{"party_size": %d, "start_at": "2026-%s:00+07:00", "end_at": "2026-%s:00+07:00"}`, n, start, end)
	}

	tests := []struct {
		name         string
		method, path string
		user         int64
		body         string
		status       int
		typ          string
	}{
		{"own-restaurant-booking", "POST", book(r), owner, body(2, "10-11T18:00", "10-11T19:00"), 403, "own-restaurant-booking"},
		{"restaurant-not-accepting-bookings", "POST", book(closed), me, body(2, "10-11T18:00", "10-11T19:00"), 409, "restaurant-not-accepting-bookings"},
		{"booking-in-past", "POST", book(r), me, body(2, "10-10T07:00", "10-10T08:00"), 422, "booking-in-past"},
		{"lead-time-not-met", "POST", book(r), me, body(2, "10-10T09:30", "10-10T10:30"), 422, "lead-time-not-met"},
		{"too-far-in-advance", "POST", book(r), me, body(2, "11-11T12:00", "11-11T13:00"), 422, "too-far-in-advance"},
		{"outside-opening-hours", "POST", book(r), me, body(2, "10-11T21:30", "10-11T22:30"), 422, "outside-opening-hours"},
		{"party-exceeds-capacity", "POST", book(r), me, body(11, "10-11T18:00", "10-11T19:00"), 422, "party-exceeds-capacity"},
		{"overlapping-own-booking", "POST", book(r), me, body(2, "10-11T12:30", "10-11T13:30"), 409, "overlapping-own-booking"},
		{"seat-capacity-exceeded", "POST", book(r), me, body(1, "10-11T14:30", "10-11T15:30"), 409, "seat-capacity-exceeded"},
		{"booking-not-active", "PATCH", "/api/bookings/" + itoa(cancelled), me, body(2, "10-11T16:00", "10-11T17:00"), 409, "booking-not-active"},
		{"cancel-deadline-passed (แก้)", "PATCH", "/api/bookings/" + itoa(late), me, body(2, "10-11T18:00", "10-11T19:00"), 422, "cancel-deadline-passed"},
		{"cancel-deadline-passed (ยกเลิก)", "POST", "/api/bookings/" + itoa(late) + "/cancel", me, "", 422, "cancel-deadline-passed"},
		{"already-cancelled", "POST", "/api/bookings/" + itoa(cancelled) + "/cancel", me, "", 409, "already-cancelled"},
		{"not-found (การจองของคนอื่น)", "PATCH", "/api/bookings/" + itoa(mine), other, body(2, "10-11T12:00", "10-11T13:00"), 404, "not-found"},
		{"validation", "POST", book(r), me, `{"party_size": "4"}`, 400, "validation"},
		{"unauthenticated", "GET", "/api/me/bookings", 0, "", 401, "unauthenticated"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, typ, out := e.do(tc.method, tc.path, tc.user, tc.body)
			if status != tc.status || typ != "/problems/"+tc.typ {
				t.Errorf("got %d %s, want %d /problems/%s (%v)", status, typ, tc.status, tc.typ, out)
			}
			if out["title"] == nil || out["status"] != float64(tc.status) {
				t.Errorf("problem body ไม่ครบ: %v", out)
			}
		})
	}
	if status, _, _ := e.do(http.MethodGet, "/api/me/bookings", me, ""); status != 200 {
		t.Errorf("me/bookings: %d", status)
	}
}
