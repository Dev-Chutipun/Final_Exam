package restaurant_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"sync"
	"testing"
	"time"

	"devpool/internal/booking"
	"devpool/internal/restaurant"
)

// settings สร้าง body ของ PATCH /settings
func settings(capacity int, open, close string, cancel, lead int) string {
	return fmt.Sprintf(`{"capacity": %d, "open_time": %q, "close_time": %q, "cancel_window_minutes": %d, "min_lead_minutes": %d}`,
		capacity, open, close, cancel, lead)
}

func (e *env) patchSettings(r, user int64, body string) (int, string, map[string]any) {
	return e.do(http.MethodPatch, "/api/restaurants/"+itoa(r)+"/settings", user, body)
}

func conflictsOf(t *testing.T, body map[string]any) map[float64]string {
	t.Helper()
	list, ok := body["conflicts"].([]any)
	if !ok {
		t.Fatalf("ไม่มี conflicts: %v", body)
	}
	out := map[float64]string{}
	for _, c := range list {
		m := c.(map[string]any)
		out[m["booking_id"].(float64)] = m["reason"].(string)
	}
	return out
}

// T1: ลดที่นั่ง 10 → 8 ขณะที่อนาคตมี peak 10 → 409 พร้อมรายการการจองที่ชน
func TestSettingsT1CapacityConflict(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	r := e.restaurant(owner)
	b1 := e.existing(r, e.user("a"), day11("12:00"), day11("13:00"), 6)
	b2 := e.existing(r, e.user("b"), day11("12:30"), day11("13:30"), 4)

	status, typ, body := e.patchSettings(r, owner, settings(8, "10:00", "22:00", 30, 120))
	if status != 409 || typ != "/problems/settings-conflict-with-bookings" {
		t.Fatalf("got %d %s %v", status, typ, body)
	}
	got := conflictsOf(t, body)
	if len(got) != 2 || got[float64(b1)] != "capacity" || got[float64(b2)] != "capacity" {
		t.Errorf("conflicts = %v", got)
	}
}

// T2: ลด 10 → 8 ขณะที่ peak 8 → 200 และค่าถูกบันทึก
func TestSettingsT2CapacityOK(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	r := e.restaurant(owner)
	e.existing(r, e.user("a"), day11("12:00"), day11("13:00"), 5)
	e.existing(r, e.user("b"), day11("12:30"), day11("13:30"), 3)

	status, _, body := e.patchSettings(r, owner, settings(8, "09:00", "23:30", 60, 180))
	if status != 200 || body["capacity"] != float64(8) || body["open_time"] != "09:00" || body["close_time"] != "23:30" {
		t.Fatalf("got %d %v", status, body)
	}
	var capacity, cancel, lead int
	var open, close string
	err := e.pool.QueryRow(context.Background(), `
		SELECT capacity, to_char(open_time, 'HH24:MI'), to_char(close_time, 'HH24:MI'), cancel_window_minutes, min_lead_minutes
		  FROM restaurants WHERE id = $1`, r).Scan(&capacity, &open, &close, &cancel, &lead)
	if err != nil {
		t.Fatal(err)
	}
	if capacity != 8 || open != "09:00" || close != "23:30" || cancel != 60 || lead != 180 {
		t.Errorf("row = %d %s %s %d %d", capacity, open, close, cancel, lead)
	}
}

// T3: เปลี่ยนเวลาเปิดจนการจองเดิมหลุดเวลา → 409 reason opening_hours
func TestSettingsT3OpeningHoursConflict(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	r := e.restaurant(owner)
	early := e.existing(r, e.user("a"), day11("10:00"), day11("11:00"), 2)
	e.existing(r, e.user("b"), day11("12:00"), day11("13:00"), 2)

	status, typ, body := e.patchSettings(r, owner, settings(10, "11:00", "22:00", 30, 120))
	if status != 409 || typ != "/problems/settings-conflict-with-bookings" {
		t.Fatalf("got %d %s", status, typ)
	}
	if got := conflictsOf(t, body); len(got) != 1 || got[float64(early)] != "opening_hours" {
		t.Errorf("conflicts = %v", got)
	}

	// เปลี่ยนเป็นร้านข้ามเที่ยงคืนที่ยังครอบการจองทั้งหมด → ผ่าน
	if status, _, body := e.patchSettings(r, owner, settings(10, "10:00", "02:00", 30, 120)); status != 200 {
		t.Errorf("overnight: %d %v", status, body)
	}
}

// T4: cancel 20 / lead 20 กับ cancel 30 → 422
func TestSettingsT4Policy(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	r := e.restaurant(owner)
	for _, body := range []string{
		settings(10, "10:00", "22:00", 20, 120),
		settings(10, "10:00", "22:00", 30, 20),
	} {
		if status, typ, _ := e.patchSettings(r, owner, body); status != 422 || typ != "/problems/invalid-restaurant-policy" {
			t.Errorf("%s: got %d %s", body, status, typ)
		}
	}
}

// T5: ไม่ใช่เจ้าของร้าน → 403 · ไม่ได้ login → 401 · ร้านถูกลบ → 404
func TestSettingsT5Permissions(t *testing.T) {
	e := newEnv(t)
	owner, other := e.user("owner"), e.user("other")
	r := e.restaurant(owner)
	body := settings(10, "10:00", "22:00", 30, 120)

	if status, typ, _ := e.patchSettings(r, other, body); status != 403 || typ != "/problems/not-restaurant-owner" {
		t.Errorf("other: %d %s", status, typ)
	}
	if status, typ, _ := e.patchSettings(r, 0, body); status != 401 || typ != "/problems/unauthenticated" {
		t.Errorf("anonymous: %d %s", status, typ)
	}
	e.exec(`UPDATE restaurants SET deleted_at = now() WHERE id = $1`, r)
	if status, typ, _ := e.patchSettings(r, owner, body); status != 404 || typ != "/problems/not-found" {
		t.Errorf("deleted: %d %s", status, typ)
	}
}

// T6: การจองที่ end_at ≤ now ไม่ถูกนับ
func TestSettingsT6PastBookingsIgnored(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	r := e.restaurant(owner)
	e.existing(r, e.user("a"), at("2026-10-09 12:00"), at("2026-10-09 13:00"), 10)
	e.existing(r, e.user("b"), at("2026-10-10 07:00"), at("2026-10-10 08:00"), 10) // จบตรง now พอดี

	if status, _, body := e.patchSettings(r, owner, settings(5, "12:00", "13:00", 30, 120)); status != 200 {
		t.Errorf("got %d %v", status, body)
	}
}

// T7: รูปแบบผิด → 400
func TestSettingsT7Validation(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	r := e.restaurant(owner)
	for _, body := range []string{
		settings(0, "10:00", "22:00", 30, 120),
		settings(501, "10:00", "22:00", 30, 120),
		settings(10, "10:15", "22:00", 30, 120),
		settings(10, "10:00", "10:00", 30, 120),
		settings(10, "9:00", "22:00", 30, 120),
		settings(10, "24:00", "22:00", 30, 120),
		`{"capacity": 10, "open_time": "10:00", "close_time": "22:00", "cancel_window_minutes": 30}`,
		`{"capacity": 10.5, "open_time": "10:00", "close_time": "22:00", "cancel_window_minutes": 30, "min_lead_minutes": 120}`,
	} {
		if status, typ, _ := e.patchSettings(r, owner, body); status != 400 || typ != "/problems/validation" {
			t.Errorf("%s: got %d %s", body, status, typ)
		}
	}
}

// T8: ลดที่นั่งจนชนสองช่วงเวลา → ส่งเฉพาะการจองของจุด peak แรก
func TestSettingsT8FirstPeakOnly(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	r := e.restaurant(owner)
	a := e.existing(r, e.user("a"), day11("12:00"), day11("13:00"), 10)
	e.existing(r, e.user("b"), day11("18:00"), day11("19:00"), 10)

	status, _, body := e.patchSettings(r, owner, settings(8, "10:00", "22:00", 30, 120))
	if status != 409 {
		t.Fatalf("got %d", status)
	}
	if got := conflictsOf(t, body); len(got) != 1 || got[float64(a)] != "capacity" {
		t.Errorf("conflicts = %v, want only booking %d", got, a)
	}
}

// K4: ลดที่นั่งพร้อมกับมีการจองเข้ามา → ค่าสุดท้าย PeakAll(อนาคต) ≤ capacity
func TestConcurrentK4ReduceCapacityWhileBooking(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	r := e.restaurant(owner)
	seed := uint64(time.Now().UnixNano())
	rng := rand.New(rand.NewPCG(seed, seed))

	const n = 20
	// สุ่มทุกอย่างก่อนปล่อย goroutine (rand.Rand ใช้พร้อมกันหลาย goroutine ไม่ได้)
	users := make([]int64, n)
	starts := make([]time.Time, n)
	sizes := make([]int, n)
	for i := range users {
		users[i] = e.user(fmt.Sprintf("k4-%d", i))
		starts[i] = day11("12:00").Add(time.Duration(rng.IntN(4)) * booking.SlotSize)
		sizes[i] = 1 + rng.IntN(3)
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		// ลดเหลือ 6 (อาจได้ 409 ถ้ามีการจองเข้ามาก่อนจนเกิน 6 ซึ่งก็ถูกต้อง)
		_, _ = e.svc.UpdateSettings(context.Background(), owner, r, restaurant.Settings{
			Capacity: 6, Open: 10 * time.Hour, Close: 22 * time.Hour,
			CancelWindowMinutes: 30, MinLeadMinutes: 120,
		}, testNow)
	}()
	for i := range users {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			// error ที่คาดคือ seat-capacity-exceeded ซึ่งไม่ต้องตรวจในที่นี้ สิ่งที่ตรวจคือผลรวมตอนจบ
			_, _ = e.bookings.Create(context.Background(), users[i], r,
				booking.Request{PartySize: sizes[i], StartAt: starts[i], EndAt: starts[i].Add(time.Hour)}, testNow)
		}()
	}
	close(start)
	wg.Wait()

	var capacity int
	if err := e.pool.QueryRow(context.Background(), `SELECT capacity FROM restaurants WHERE id = $1`, r).Scan(&capacity); err != nil {
		t.Fatal(err)
	}
	if peak, at := booking.PeakAll(e.activeIntervals(r)); peak > capacity {
		t.Errorf("seed %d: PeakAll = %d at %v เกิน capacity %d", seed, peak, at, capacity)
	}
}

func (e *env) activeIntervals(r int64) []booking.Interval {
	e.t.Helper()
	rows, err := e.pool.Query(context.Background(),
		`SELECT start_at, end_at, party_size FROM bookings WHERE restaurant_id = $1 AND status = 'active'`, r)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var out []booking.Interval
	for rows.Next() {
		var iv booking.Interval
		if err := rows.Scan(&iv.Start, &iv.End, &iv.Size); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, iv)
	}
	return out
}
