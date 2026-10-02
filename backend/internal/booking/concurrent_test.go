package booking_test

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"devpool/internal/booking"
	"devpool/internal/httpx"
)

// รันแบบ -race -count=100 ด้วย ./scripts/test-race.ps1 -Concurrent
//
// วิธีปล่อยพร้อมกัน: ทุก goroutine รอ channel start แล้ว close(start) ครั้งเดียว
// ทุกตัวจึงหลุดออกมาพร้อมกันและแย่งกันจองจริง

// K1: ร้าน 10 ที่ มีจองแล้ว 4 เหลือ 6 — ผู้ใช้ 20 คนที่ต่างกันจองคนละ 6 ที่ช่วงเดียวกันพร้อมกัน
// ต้องสำเร็จ 1 คน อีก 19 ได้ 409 และ PeakAll ≤ 10
// (ต้องเป็นผู้ใช้ต่างกัน ไม่อย่างนั้น overlapping-own-booking จะตัดสินก่อนและไม่ได้ทดสอบ race ของที่นั่ง)
func TestConcurrentK1LastSeats(t *testing.T) {
	e := newEnv(t)
	r := e.restaurant(e.user("owner"))
	e.existing(r, e.user("early"), day11("12:00"), day11("13:00"), 4)

	const n = 20
	users := make([]int64, n)
	for i := range users {
		users[i] = e.user(fmt.Sprintf("k1-%d", i))
	}

	var ok, full, other atomic.Int32
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, u := range users {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := e.book(u, r, 6, day11("12:00"), day11("13:00"))
			switch {
			case err == nil:
				ok.Add(1)
			case isProblem(err, "seat-capacity-exceeded"):
				full.Add(1)
			default:
				other.Add(1)
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if ok.Load() != 1 || full.Load() != n-1 || other.Load() != 0 {
		t.Errorf("สำเร็จ %d, 409 %d, อื่นๆ %d; want 1, %d, 0", ok.Load(), full.Load(), other.Load(), n-1)
	}
	if peak, at := booking.PeakAll(e.activeIntervals(r)); peak > 10 {
		t.Errorf("PeakAll = %d at %v, เกิน capacity 10", peak, at)
	}
}

// K2: 50 goroutine จองแบบสุ่มขนาดและเวลาพร้อมกัน → PeakAll ≤ capacity หลังจบ
func TestConcurrentK2RandomBookings(t *testing.T) {
	e := newEnv(t)
	r := e.restaurant(e.user("owner"))

	seed := uint64(time.Now().UnixNano())
	rng := rand.New(rand.NewPCG(seed, seed))
	t.Logf("seed %d", seed)

	// เตรียมคำขอก่อนปล่อย goroutine: ช่วงเวลาภายใน 10:00–22:00 ยาว 1–4 ช่อง ขนาด 1–6 คน
	const n = 50
	type req struct {
		user       int64
		size       int
		start, end time.Time
	}
	reqs := make([]req, n)
	open := day11("10:00")
	for i := range reqs {
		slots := 1 + rng.IntN(4)
		first := rng.IntN(24 - slots + 1) // 24 ช่องใน 10:00–22:00
		s := open.Add(time.Duration(first) * booking.SlotSize)
		reqs[i] = req{e.user(fmt.Sprintf("k2-%d", i)), 1 + rng.IntN(6), s, s.Add(time.Duration(slots) * booking.SlotSize)}
	}

	var ok atomic.Int32
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, q := range reqs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := e.book(q.user, r, q.size, q.start, q.end)
			if err == nil {
				ok.Add(1)
			} else if !isProblem(err, "seat-capacity-exceeded") {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if ok.Load() == 0 {
		t.Error("ไม่มีการจองสำเร็จเลย")
	}
	if peak, at := booking.PeakAll(e.activeIntervals(r)); peak > 10 {
		t.Errorf("seed %d: PeakAll = %d at %v, เกิน capacity 10", seed, peak, at)
	}
}

// isProblem ใช้ใน goroutine จึงไม่เรียก t.Fatal (problemOf ใช้ได้เฉพาะ goroutine หลักของ test)
func isProblem(err error, typ string) bool {
	var p *httpx.Problem
	return errors.As(err, &p) && p.Type == typ
}
