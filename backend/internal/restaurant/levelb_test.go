package restaurant_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"devpool/internal/booking"
)

// P1–P3 [B] พักรับจอง: ไม่ใช่เจ้าของ 403 · ส่งซ้ำ 200 · สถานะสะท้อนในรายละเอียด
func TestBookingStatusP1toP3(t *testing.T) {
	e := newEnv(t)
	owner, other := e.user("owner"), e.user("other")
	r := e.createViaAPI(owner, "ร้าน")
	path := "/api/restaurants/" + itoa(r) + "/booking-status"

	if status, typ, _ := e.do("PATCH", path, other, `{"accepting_bookings": false}`); status != 403 || typ != "/problems/not-restaurant-owner" {
		t.Errorf("P1: %d %s", status, typ)
	}
	for range 2 { // P2 ส่งซ้ำได้
		if status, _, body := e.do("PATCH", path, owner, `{"accepting_bookings": false}`); status != 200 || body["accepting_bookings"] != false {
			t.Errorf("P2: %d %v", status, body)
		}
	}
	if _, _, body := e.do("GET", "/api/restaurants/"+itoa(r), 0, ""); body["accepting_bookings"] != false { // P3
		t.Errorf("P3 detail: %v", body["accepting_bookings"])
	}
	if status, typ, _ := e.do("PATCH", path, owner, `{}`); status != 400 || typ != "/problems/validation" {
		t.Errorf("missing field: %d %s", status, typ)
	}
	if status, _, body := e.do("PATCH", path, owner, `{"accepting_bookings": true}`); status != 200 || body["accepting_bookings"] != true {
		t.Errorf("reopen: %d %v", status, body)
	}
}

// B1 [B] การจองรายวัน: ร้านข้ามเที่ยงคืน, ไม่ใช่เจ้าของ 403, ค่า peak ถูกต้อง
func TestDailyBookingsB1(t *testing.T) {
	e := newEnv(t)
	owner, other := e.user("owner"), e.user("other")
	a, b := e.user("แอน"), e.user("บี")
	r := e.createViaAPI(owner, "ร้านดึก")
	e.exec(`UPDATE restaurants SET open_time = '18:00', close_time = '02:00' WHERE id = $1`, r)

	b1 := e.existing(r, a, day11("19:00"), day11("21:00"), 4)
	b2 := e.existing(r, b, day11("20:00"), day11("21:00"), 3)                 // peak 7 ที่ 20:00
	b3 := e.existing(r, a, at("2026-10-12 01:00"), at("2026-10-12 02:00"), 2) // หลังเที่ยงคืนยังเป็นวันทำการที่ 11
	cancelled := e.existing(r, b, day11("20:00"), day11("21:00"), 5)
	e.exec(`UPDATE bookings SET status = 'cancelled', cancelled_at = now() WHERE id = $1`, cancelled)
	e.existing(r, a, at("2026-10-12 19:00"), at("2026-10-12 20:00"), 2) // วันทำการที่ 12 ไม่ปรากฏ

	path := "/api/restaurants/" + itoa(r) + "/bookings?date=2026-10-11"
	if status, typ, _ := e.do("GET", path, other, ""); status != 403 || typ != "/problems/not-restaurant-owner" {
		t.Errorf("other: %d %s", status, typ)
	}
	status, _, body := e.do("GET", path, owner, "")
	if status != 200 {
		t.Fatalf("got %d %v", status, body)
	}
	var ids []string
	for _, it := range body["items"].([]any) {
		ids = append(ids, fmt.Sprint(it.(map[string]any)["id"]))
	}
	want := fmt.Sprint(b1, " ", b2, " ", cancelled, " ", b3)
	if strings.Join(ids, " ") != want {
		t.Errorf("ids = %v, want %s", ids, want)
	}
	first := body["items"].([]any)[0].(map[string]any)
	if first["customer_display_name"] != "แอน" || first["party_size"] != float64(4) || first["email"] != nil {
		t.Errorf("first = %v", first)
	}
	if body["peak"] != float64(7) || body["peak_at"] != "2026-10-11T20:00:00+07:00" {
		t.Errorf("peak = %v at %v", body["peak"], body["peak_at"])
	}

	// วันที่ไม่มีการจอง → peak 0, peak_at null, items []
	_, _, empty := e.do("GET", "/api/restaurants/"+itoa(r)+"/bookings?date=2026-10-20", owner, "")
	if empty["peak"] != float64(0) || empty["peak_at"] != nil || len(empty["items"].([]any)) != 0 {
		t.Errorf("empty = %v", empty)
	}
	if status, typ, _ := e.do("GET", "/api/restaurants/"+itoa(r)+"/bookings?date=bad", owner, ""); status != 400 || typ != "/problems/validation" {
		t.Errorf("bad date: %d %s", status, typ)
	}
}

// K8 [B] สั่งพักรับจองพร้อมกับ 20 คนจอง → หลังพักตอบ 200 แล้ว คำขอจองถัดไปได้ 409 ทุกคำขอ
func TestConcurrentK8SuspendWhileBooking(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	r := e.createViaAPI(owner, "ร้าน")
	users := make([]int64, 25)
	for i := range users {
		users[i] = e.user(fmt.Sprint("k8-", i))
	}
	book := func(u int64) error {
		_, err := e.bookings.Create(context.Background(), u, r,
			booking.Request{PartySize: 1, StartAt: day11("12:00"), EndAt: day11("13:00")}, testNow)
		return err
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	var suspendErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		suspendErr = e.svc.SetAcceptingBookings(context.Background(), owner, r, false, testNow)
	}()
	for _, u := range users[:20] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_ = book(u)
		}()
	}
	close(start)
	wg.Wait()
	if suspendErr != nil {
		t.Fatal(suspendErr)
	}
	for _, u := range users[20:] {
		if err := book(u); err == nil || !strings.Contains(err.Error(), "restaurant-not-accepting-bookings") {
			t.Errorf("หลังพักรับจอง: err = %v", err)
		}
	}
}
