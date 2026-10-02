package booking

import (
	"testing"
	"time"
)

// hm คือเวลา HH:MM ของวันที่ 11 ต.ค. 2026 (Bangkok) ซึ่งเป็นวันจองในค่ากลางของ 01 §10
func hm(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", "2026-10-11 "+s, Bangkok)
	if err != nil {
		panic(err)
	}
	return t
}

func iv(start, end string, size int) Interval {
	return Interval{Start: hm(start), End: hm(end), Size: size}
}

// 01 §10.1 ร้าน capacity 10
func TestMaxLoad(t *testing.T) {
	const capacity = 10
	tests := []struct {
		name      string
		existing  []Interval
		s, e      string
		n         int
		wantLoad  int
		wantAt    string
		wantOK    bool // n + load ≤ capacity
		available int  // capacity − load (ใช้ตอบ seat-capacity-exceeded)
	}{
		{"C1 สไลด์ 7 ซ้าย", []Interval{iv("12:00", "13:00", 7)}, "12:30", "13:00", 5, 7, "12:30", false, 3},
		{"C2 สไลด์ 7 ขวา", []Interval{iv("12:00", "12:30", 7), iv("12:30", "13:00", 7)}, "12:00", "13:00", 3, 7, "12:00", true, 3},
		{"C3 สไลด์ 8 แก้เป็น 8 คน", []Interval{iv("12:00", "13:00", 3)}, "12:00", "13:00", 8, 3, "12:00", false, 7},
		{"C4 สไลด์ 8 แก้เป็น 5 คน", []Interval{iv("12:00", "13:00", 3)}, "12:00", "13:00", 5, 3, "12:00", true, 7},
		{"C5 เต็มพอดี", []Interval{iv("12:00", "13:00", 6)}, "12:00", "13:00", 4, 6, "12:00", true, 4},
		{"C6 ต่อกันพอดี", []Interval{iv("12:00", "13:00", 10)}, "13:00", "14:00", 10, 0, "13:00", true, 10},
		{"C7 n=4", []Interval{iv("12:00", "12:30", 4), iv("12:30", "13:00", 6), iv("13:00", "13:30", 2)}, "12:00", "13:30", 4, 6, "12:30", true, 4},
		{"C7 n=5", []Interval{iv("12:00", "12:30", 4), iv("12:30", "13:00", 6), iv("13:00", "13:30", 2)}, "12:00", "13:30", 5, 6, "12:30", false, 4},
		{"C8 n=2", []Interval{iv("11:00", "12:30", 8)}, "12:00", "13:00", 2, 8, "12:00", true, 2},
		{"C8 n=3", []Interval{iv("11:00", "12:30", 8)}, "12:00", "13:00", 3, 8, "12:00", false, 2},
		{"C9 ไม่มีการจอง", nil, "12:00", "13:00", 10, 0, "12:00", true, 10},
		{"C10 ช่วงที่ไม่ทับไม่ถูกนับ", []Interval{iv("09:00", "10:00", 9)}, "12:00", "13:00", 10, 0, "12:00", true, 10},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			load, at := MaxLoad(tc.existing, hm(tc.s), hm(tc.e))
			if load != tc.wantLoad || !at.Equal(hm(tc.wantAt)) {
				t.Errorf("MaxLoad = %d at %s, want %d at %s",
					load, at.In(Bangkok).Format("15:04"), tc.wantLoad, tc.wantAt)
			}
			if ok := tc.n+load <= capacity; ok != tc.wantOK {
				t.Errorf("จองได้ = %v, want %v", ok, tc.wantOK)
			}
			if avail := capacity - load; avail != tc.available {
				t.Errorf("available = %d, want %d", avail, tc.available)
			}
		})
	}
}

func TestPeakAll(t *testing.T) {
	tests := []struct {
		name     string
		all      []Interval
		wantPeak int
		wantAt   time.Time
	}{
		{"P1 ทับกันครึ่งชั่วโมง", []Interval{iv("12:00", "13:00", 7), iv("12:30", "13:30", 5)}, 12, hm("12:30")},
		{"P2 ต่อกันพอดีไม่รวมกัน", []Interval{iv("12:00", "13:00", 10), iv("13:00", "14:00", 10)}, 10, hm("12:00")},
		{"P3 ว่าง", nil, 0, time.Time{}},
		// ลำดับใน slice ต้องไม่มีผล: ถ้าเรียง event ผิด (เริ่มก่อนจบ) จะได้ 20
		{"P2 สลับลำดับ", []Interval{iv("13:00", "14:00", 10), iv("12:00", "13:00", 10)}, 10, hm("12:00")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			peak, at := PeakAll(tc.all)
			if peak != tc.wantPeak || !at.Equal(tc.wantAt) {
				t.Errorf("PeakAll = %d at %v, want %d at %v", peak, at, tc.wantPeak, tc.wantAt)
			}
		})
	}
}
