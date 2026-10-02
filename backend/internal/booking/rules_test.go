package booking

import (
	"errors"
	"testing"
	"time"

	"devpool/internal/httpx"
)

// ts แปลง "2026-10-11 12:00" (เวลา Bangkok) เป็น time.Time
func ts(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, Bangkok)
	if err != nil {
		panic(err)
	}
	return t
}

// problemType คืน type ของ problem หรือ "" ถ้า err เป็น nil
func problemType(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		return ""
	}
	var p *httpx.Problem
	if !errors.As(err, &p) {
		t.Fatalf("error ไม่ใช่ *httpx.Problem: %v", err)
	}
	return p.Type
}

// ค่ากลางของ 01 §10: เปิด 10:00–22:00, min_lead 120, now = 2026-10-10 08:00
var (
	defaultHours = Hours{Open: 10 * time.Hour, Close: 22 * time.Hour}
	overnight    = Hours{Open: 18 * time.Hour, Close: 2 * time.Hour} // 18:00–02:00
	testNow      = ts("2026-10-10 08:00")
)

const testLead = 120

// checkAll รันกติกาตามลำดับของ pipeline: รูปแบบ (400) → เทียบ now (422) → เวลาเปิด (422)
func checkAll(start, end time.Time, h Hours) error {
	if err := ValidateShape(start, end); err != nil {
		return err
	}
	if err := CheckTiming(start, testNow, testLead); err != nil {
		return err
	}
	return CheckOpeningHours(start, end, h)
}

func TestRules(t *testing.T) {
	tests := []struct {
		name       string
		start, end string
		hours      Hours
		want       string // problem type, "" = ผ่าน
	}{
		{"R1 ไม่ลงช่อง", "2026-10-11 12:15", "2026-10-11 13:00", defaultHours, "validation"},
		{"R2 เกิน 3 ชม.", "2026-10-11 12:00", "2026-10-11 15:30", defaultHours, "validation"},
		{"R3 เริ่มเท่ากับจบ", "2026-10-11 12:00", "2026-10-11 12:00", defaultHours, "validation"},
		{"R3 จบก่อนเริ่ม", "2026-10-11 13:00", "2026-10-11 12:00", defaultHours, "validation"},
		{"R4 ผ่านไปแล้ว", "2026-10-10 07:30", "2026-10-10 08:30", defaultHours, "booking-in-past"},
		{"R4 เริ่มตรง now", "2026-10-10 08:00", "2026-10-10 09:00", defaultHours, "booking-in-past"},
		{"R5 ล่วงหน้าไม่พอ", "2026-10-10 09:30", "2026-10-10 10:30", defaultHours, "lead-time-not-met"},
		{"R6 ล่วงหน้า 2 ชม. พอดี", "2026-10-10 10:00", "2026-10-10 11:00", defaultHours, ""},
		{"R7 เกิน 30 วัน", "2026-11-10 12:00", "2026-11-10 13:00", defaultHours, "too-far-in-advance"},
		{"R7 30 วันพอดี", "2026-11-09 08:00", "2026-11-09 09:00", Hours{Open: 0, Close: 23*time.Hour + 30*time.Minute}, ""},
		{"R8 จบตรงเวลาปิด", "2026-10-11 21:00", "2026-10-11 22:00", defaultHours, ""},
		{"R9 จบเกินเวลาปิด", "2026-10-11 21:30", "2026-10-11 22:30", defaultHours, "outside-opening-hours"},
		{"R10 เริ่มก่อนเวลาเปิด", "2026-10-11 09:30", "2026-10-11 10:30", defaultHours, "outside-opening-hours"},
		{"R11 ข้ามเที่ยงคืน", "2026-10-11 23:30", "2026-10-12 00:30", overnight, ""},
		{"R12 หน้าต่างของวันที่ 11", "2026-10-12 01:00", "2026-10-12 02:00", overnight, ""},
		{"R13 เลยเวลาปิด 02:00", "2026-10-12 01:30", "2026-10-12 02:30", overnight, "outside-opening-hours"},
		{"R14 เริ่มก่อนเปิด 18:00", "2026-10-11 17:30", "2026-10-11 18:30", overnight, "outside-opening-hours"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := problemType(t, checkAll(ts(tc.start), ts(tc.end), tc.hours))
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// R1 แบบละเอียด: ฟิลด์ที่ผิดต้องอยู่ใน errors[]
func TestValidateShapeFields(t *testing.T) {
	err := ValidateShape(ts("2026-10-11 12:15"), ts("2026-10-11 13:00"))
	var p *httpx.Problem
	if !errors.As(err, &p) || len(p.Errors) != 1 || p.Errors[0].Field != "start_at" {
		t.Fatalf("got %+v, want one error on start_at", err)
	}
}

func TestOnSlot(t *testing.T) {
	base := ts("2026-10-11 12:30")
	tests := []struct {
		name string
		t    time.Time
		want bool
	}{
		{"12:30", base, true},
		{"12:00", ts("2026-10-11 12:00"), true},
		{"12:15", ts("2026-10-11 12:15"), false},
		{"มีวินาที", base.Add(time.Second), false},
		{"มี nanosecond", base.Add(time.Nanosecond), false},
		// 05:30 UTC = 12:30 Bangkok ต้องตรวจตามเวลา Bangkok ไม่ใช่ตาม offset ที่ส่งมา
		{"ส่งมาเป็น UTC", base.UTC(), true},
		// offset +05:45 (เนปาล) นาทีไม่ใช่ 0/30 แต่เวลาจริงคือ 12:30 Bangkok
		{"offset แปลก", base.In(time.FixedZone("NPT", 5*3600+45*60)), true},
	}
	for _, tc := range tests {
		if got := OnSlot(tc.t); got != tc.want {
			t.Errorf("%s: OnSlot = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestOpeningWindow(t *testing.T) {
	ws, we := OpeningWindow(ts("2026-10-11 15:00"), overnight)
	if !ws.Equal(ts("2026-10-11 18:00")) || !we.Equal(ts("2026-10-12 02:00")) {
		t.Errorf("overnight window = %v – %v", ws, we)
	}
	ws, we = OpeningWindow(ts("2026-10-11 00:00"), defaultHours)
	if !ws.Equal(ts("2026-10-11 10:00")) || !we.Equal(ts("2026-10-11 22:00")) {
		t.Errorf("default window = %v – %v", ws, we)
	}
}

// R15 deadline นับรวมเวลาพอดี
func TestCanModify(t *testing.T) {
	start := ts("2026-10-11 12:00")
	deadline := CancelDeadline(start, 30)
	if !deadline.Equal(ts("2026-10-11 11:30")) {
		t.Fatalf("deadline = %v, want 11:30", deadline)
	}
	if !CanModify(deadline, deadline) {
		t.Error("now = deadline พอดี ต้องแก้ได้")
	}
	if CanModify(deadline.Add(time.Minute), deadline) {
		t.Error("deadline + 1 นาที ต้องแก้ไม่ได้")
	}
	if !CanModify(deadline.Add(-time.Hour), deadline) {
		t.Error("ก่อน deadline ต้องแก้ได้")
	}
}
