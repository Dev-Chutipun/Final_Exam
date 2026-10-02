package booking_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"devpool/internal/booking"
)

func (e *env) update(userID, bookingID int64, size int, start, end, now time.Time) (booking.Booking, error) {
	return e.svc.Update(context.Background(), userID, bookingID,
		booking.Request{PartySize: size, StartAt: start, EndAt: end}, now)
}

func (e *env) cancel(userID, bookingID int64, now time.Time) (booking.Booking, error) {
	return e.svc.Cancel(context.Background(), userID, bookingID, now)
}

// mustBook จองผ่าน service แล้วคืน id
func (e *env) mustBook(userID, restaurantID int64, size int, start, end time.Time) int64 {
	e.t.Helper()
	b, err := e.book(userID, restaurantID, size, start, end)
	if err != nil {
		e.t.Fatal(err)
	}
	return b.ID
}

// สไลด์ 8 แบบแก้ไขจริง: A 5 คน + B 3 คน ช่วง 12:00–13:00
// A แก้เป็น 8 คน → 3 + 8 = 11 ✕ · A แก้เป็น 5 คน (เท่าเดิม) → 3 + 5 = 8 ✓ เพราะตัดที่นั่งเดิมของ A ออก
func TestUpdateSlide8ExcludesOwnSeats(t *testing.T) {
	e := newEnv(t)
	owner, a, b := e.user("owner"), e.user("a"), e.user("b")
	r := e.restaurant(owner)
	idA := e.mustBook(a, r, 5, day11("12:00"), day11("13:00"))
	e.mustBook(b, r, 3, day11("12:00"), day11("13:00"))

	_, err := e.update(a, idA, 8, day11("12:00"), day11("13:00"), testNow)
	wantType(t, err, "seat-capacity-exceeded")
	if p := problemOf(t, err); p.Extra["available"] != 7 {
		t.Errorf("available = %v, want 7 (ไม่นับที่นั่งเดิมของ A)", p.Extra["available"])
	}

	got, err := e.update(a, idA, 5, day11("12:00"), day11("13:00"), testNow)
	wantType(t, err, "")
	if got.PartySize != 5 {
		t.Errorf("party_size = %d", got.PartySize)
	}

	// ย้ายเวลาทับตัวเองเดิมได้ (ตัดการจองนี้ออกจากการตรวจ overlapping-own-booking ด้วย)
	_, err = e.update(a, idA, 5, day11("12:30"), day11("13:30"), testNow)
	wantType(t, err, "")
}

// S9: แก้หลัง deadline (deadline 11:30)
func TestUpdateS9AfterDeadline(t *testing.T) {
	e := newEnv(t)
	me := e.user("me")
	r := e.restaurant(e.user("owner"))
	id := e.mustBook(me, r, 2, day11("12:00"), day11("13:00"))

	_, err := e.update(me, id, 3, day11("12:00"), day11("13:00"), day11("11:31"))
	wantType(t, err, "cancel-deadline-passed")
}

// S10: แก้การจองที่ยกเลิกแล้ว
func TestUpdateS10Cancelled(t *testing.T) {
	e := newEnv(t)
	me := e.user("me")
	r := e.restaurant(e.user("owner"))
	id := e.mustBook(me, r, 2, day11("12:00"), day11("13:00"))
	if _, err := e.cancel(me, id, testNow); err != nil {
		t.Fatal(err)
	}
	_, err := e.update(me, id, 3, day11("12:00"), day11("13:00"), testNow)
	wantType(t, err, "booking-not-active")
}

// S11: แก้การจองของคนอื่น → 404 (ไม่เปิดเผยว่ามีอยู่)
func TestUpdateS11OthersBooking(t *testing.T) {
	e := newEnv(t)
	me, other := e.user("me"), e.user("other")
	r := e.restaurant(e.user("owner"))
	id := e.mustBook(other, r, 2, day11("12:00"), day11("13:00"))

	_, err := e.update(me, id, 3, day11("12:00"), day11("13:00"), testNow)
	wantType(t, err, "not-found")
	_, err = e.update(me, 999999, 3, day11("12:00"), day11("13:00"), testNow)
	wantType(t, err, "not-found")
}

// S12: แก้แล้ว cancel_deadline คำนวณใหม่ตามนโยบายปัจจุบันของร้าน
func TestUpdateS12RecomputesDeadline(t *testing.T) {
	e := newEnv(t)
	me := e.user("me")
	r := e.restaurant(e.user("owner"))
	id := e.mustBook(me, r, 2, day11("12:00"), day11("13:00")) // deadline 11:30 (นโยบาย 30 นาที)
	e.exec(`UPDATE restaurants SET cancel_window_minutes = 60 WHERE id = $1`, r)

	got, err := e.update(me, id, 2, day11("13:00"), day11("14:00"), testNow)
	wantType(t, err, "")
	if !got.CancelDeadline.Equal(day11("12:00")) {
		t.Errorf("cancel_deadline = %v, want 12:00 (13:00 − 60 นาที)", got.CancelDeadline)
	}
}

// S13: ยกเลิกตรง deadline / หลัง 1 นาที / ซ้ำ / ของคนอื่น
func TestCancelS13(t *testing.T) {
	e := newEnv(t)
	me, other := e.user("me"), e.user("other")
	r := e.restaurant(e.user("owner"))
	id1 := e.mustBook(me, r, 2, day11("12:00"), day11("13:00"))
	id2 := e.mustBook(me, r, 2, day11("14:00"), day11("15:00"))

	got, err := e.cancel(me, id1, day11("11:30")) // deadline พอดี
	wantType(t, err, "")
	if got.Status != "cancelled" {
		t.Errorf("status = %s", got.Status)
	}
	_, err = e.cancel(me, id2, day11("13:31")) // deadline 13:30 + 1 นาที
	wantType(t, err, "cancel-deadline-passed")
	_, err = e.cancel(me, id1, day11("11:00"))
	wantType(t, err, "already-cancelled")
	_, err = e.cancel(other, id2, day11("11:00"))
	wantType(t, err, "not-found")
}

// S14: ร้านเปลี่ยนเวลายกเลิก 30 → 120 หลังมีการจอง deadline เดิมไม่เปลี่ยน
func TestS14PolicyChangeDoesNotAffectExisting(t *testing.T) {
	e := newEnv(t)
	me := e.user("me")
	r := e.restaurant(e.user("owner"))
	id := e.mustBook(me, r, 2, day11("12:00"), day11("13:00"))
	e.exec(`UPDATE restaurants SET cancel_window_minutes = 120, min_lead_minutes = 120 WHERE id = $1`, r)

	var deadline time.Time
	if err := e.pool.QueryRow(context.Background(),
		`SELECT cancel_deadline FROM bookings WHERE id = $1`, id).Scan(&deadline); err != nil {
		t.Fatal(err)
	}
	if !deadline.Equal(day11("11:30")) {
		t.Errorf("deadline = %v, want 11:30", deadline)
	}
	// ยังยกเลิกได้ถึง 11:30 ตามที่ตกลงตอนจอง
	_, err := e.cancel(me, id, day11("11:30"))
	wantType(t, err, "")
}

// S15: การจองของฉัน 3 แท็บ ลำดับ counts is_past can_modify และแบ่งหน้า
func TestMyBookingsS15(t *testing.T) {
	e := newEnv(t)
	me, other := e.user("me"), e.user("other")
	r := e.restaurant(e.user("owner"))
	ins := func(start, end time.Time, status string, cancelledAt any) int64 {
		var id int64
		err := e.pool.QueryRow(context.Background(), `
			INSERT INTO bookings (restaurant_id, user_id, party_size, start_at, end_at, cancel_deadline, status, cancelled_at)
			VALUES ($1, $2, 2, $3, $4, $5, $6, $7) RETURNING id`,
			r, me, start, end, booking.CancelDeadline(start, 30), status, cancelledAt).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	b2 := ins(at("2026-10-12 12:00"), at("2026-10-12 13:00"), "active", nil)
	b1 := ins(day11("12:00"), day11("13:00"), "active", nil)
	ongoing := ins(at("2026-10-10 08:00"), at("2026-10-10 09:00"), "active", nil) // เริ่มแล้ว ยังไม่จบ
	p2 := ins(at("2026-10-08 12:00"), at("2026-10-08 13:00"), "active", nil)
	p1 := ins(at("2026-10-09 12:00"), at("2026-10-09 13:00"), "active", nil)
	c1 := ins(day11("15:00"), day11("16:00"), "cancelled", at("2026-10-09 10:00"))
	c2 := ins(day11("17:00"), day11("18:00"), "cancelled", at("2026-10-09 11:00"))
	e.existing(r, other, day11("12:00"), day11("13:00"), 2) // ของคนอื่นต้องไม่ปรากฏ

	ids := func(res booking.MyBookingsResult) []int64 {
		var out []int64
		for _, it := range res.Items {
			out = append(out, it.ID)
		}
		return out
	}
	get := func(tab string, page, size int) booking.MyBookingsResult {
		t.Helper()
		res, err := e.svc.MyBookings(context.Background(), me, tab, httpxPage(page, size), testNow)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	up := get("upcoming", 1, 20)
	if !equalIDs(ids(up), []int64{ongoing, b1, b2}) {
		t.Errorf("upcoming = %v, want %v", ids(up), []int64{ongoing, b1, b2})
	}
	if up.Counts != (booking.TabCounts{Upcoming: 3, Past: 2, Cancelled: 2}) || up.Total != 3 {
		t.Errorf("counts = %+v total %d", up.Counts, up.Total)
	}
	if it := up.Items[0]; it.IsPast || it.CanModify {
		t.Errorf("ongoing: is_past %v can_modify %v, want false false", it.IsPast, it.CanModify)
	}
	if it := up.Items[1]; it.IsPast || !it.CanModify || it.RestaurantName == "" {
		t.Errorf("b1 = %+v", it)
	}

	past := get("past", 1, 20)
	if !equalIDs(ids(past), []int64{p1, p2}) || !past.Items[0].IsPast || past.Items[0].CanModify {
		t.Errorf("past = %v %+v", ids(past), past.Items)
	}
	canc := get("cancelled", 1, 20)
	if !equalIDs(ids(canc), []int64{c2, c1}) || canc.Items[0].CanModify {
		t.Errorf("cancelled = %v", ids(canc))
	}

	// หน้า 2 ขนาด 2 ของ upcoming
	pg := get("upcoming", 2, 2)
	if !equalIDs(ids(pg), []int64{b2}) || pg.Total != 3 || pg.Page != 2 || pg.PageSize != 2 {
		t.Errorf("page 2 = %v total %d", ids(pg), pg.Total)
	}

	// tab ผิด → 400 / page_size เกิน 50 → 400
	for _, q := range []string{"?tab=all", "?page_size=51", "?page=0", "?page=x"} {
		status, typ, _ := e.do(http.MethodGet, "/api/me/bookings"+q, me, "")
		if status != 400 || typ != "/problems/validation" {
			t.Errorf("%s: got %d %s, want 400 validation", q, status, typ)
		}
	}
	status, _, body := e.do(http.MethodGet, "/api/me/bookings?tab=past", me, "")
	if status != 200 || body["counts"] == nil || body["items"] == nil {
		t.Errorf("handler: %d %v", status, body)
	}
}

// S17: ร้านพักรับจอง: แก้ไข 409 / ยกเลิก 200
func TestS17NotAcceptingEditAndCancel(t *testing.T) {
	e := newEnv(t)
	me := e.user("me")
	r := e.restaurant(e.user("owner"))
	id := e.mustBook(me, r, 2, day11("12:00"), day11("13:00"))
	e.exec(`UPDATE restaurants SET accepting_bookings = false WHERE id = $1`, r)

	_, err := e.update(me, id, 3, day11("12:00"), day11("13:00"), testNow)
	wantType(t, err, "restaurant-not-accepting-bookings")
	_, err = e.cancel(me, id, testNow)
	wantType(t, err, "")
}

// S18: ร้านถูกลบ: จองหรือแก้ไข 404 / การจองเก่ายังอยู่ในการจองของฉันพร้อม restaurant_deleted
func TestS18DeletedRestaurant(t *testing.T) {
	e := newEnv(t)
	me := e.user("me")
	r := e.restaurant(e.user("owner"))
	id := e.mustBook(me, r, 2, day11("12:00"), day11("13:00"))
	e.exec(`UPDATE restaurants SET deleted_at = now() WHERE id = $1`, r)

	_, err := e.book(me, r, 2, day11("14:00"), day11("15:00"))
	wantType(t, err, "not-found")
	_, err = e.update(me, id, 3, day11("12:00"), day11("13:00"), testNow)
	wantType(t, err, "not-found")

	res, err := e.svc.MyBookings(context.Background(), me, "upcoming", httpxPage(1, 20), testNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 || !res.Items[0].RestaurantDeleted || res.Items[0].CanModify {
		t.Errorf("items = %+v", res.Items)
	}
}

// handler: แก้ไขและยกเลิกผ่าน HTTP
func TestHandlerUpdateCancel(t *testing.T) {
	e := newEnv(t)
	me := e.user("me")
	r := e.restaurant(e.user("owner"))
	id := e.mustBook(me, r, 2, day11("12:00"), day11("13:00"))
	path := "/api/bookings/" + itoa(id)

	status, _, body := e.do(http.MethodPatch, path, me,
		`{"party_size": 3, "start_at": "2026-10-11T12:00:00+07:00", "end_at": "2026-10-11T13:30:00+07:00"}`)
	if status != 200 || body["party_size"] != float64(3) {
		t.Errorf("update: %d %v", status, body)
	}
	if status, typ, _ := e.do(http.MethodPost, path+"/cancel", 0, ""); status != 401 || typ != "/problems/unauthenticated" {
		t.Errorf("cancel no auth: %d %s", status, typ)
	}
	if status, _, body := e.do(http.MethodPost, path+"/cancel", me, ""); status != 200 || body["status"] != "cancelled" {
		t.Errorf("cancel: %d %v", status, body)
	}
	if status, typ, _ := e.do(http.MethodPost, path+"/cancel", me, ""); status != 409 || typ != "/problems/already-cancelled" {
		t.Errorf("cancel again: %d %s", status, typ)
	}
}

func equalIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
