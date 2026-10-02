// Package restaurant คือการจัดการร้าน: ตั้งค่าการจอง (01 §7.6) และสร้าง/แก้/ลบร้าน รูป หมวด (03)
package restaurant

import (
	"fmt"
	"strconv"
	"time"

	"devpool/internal/httpx"
)

// กฎของข้อมูลร้าน (03 §3) ใช้ร่วมกันทั้งตอนสร้างร้านและตอนแก้ตั้งค่า

const (
	MinCapacity         = 1
	MaxCapacity         = 500
	MinCancelWindow     = 30
	DefaultCancelWindow = 30
	DefaultMinLead      = 120
)

// ParseClock อ่านเวลา "HH:MM" ที่ลงช่อง 30 นาที (นาที 00 หรือ 30) คืนระยะจากเที่ยงคืน
func ParseClock(s string) (time.Duration, bool) {
	if len(s) != 5 || s[2] != ':' {
		return 0, false
	}
	h, err1 := strconv.Atoi(s[:2])
	m, err2 := strconv.Atoi(s[3:])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || (m != 0 && m != 30) {
		return 0, false
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute, true
}

// FormatClock แปลงระยะจากเที่ยงคืนเป็น "HH:MM"
func FormatClock(d time.Duration) string {
	return fmt.Sprintf("%02d:%02d", int(d.Hours()), int(d.Minutes())%60)
}

// validateCapacityAndHours ตรวจ capacity และเวลาเปิด–ปิด (400) เพิ่ม error ลงใน errs
func validateCapacityAndHours(errs []httpx.FieldError, capacity *int, openS, closeS *string) ([]httpx.FieldError, time.Duration, time.Duration) {
	if capacity == nil {
		errs = append(errs, httpx.FieldError{Field: "capacity", Message: "จำเป็นต้องระบุ"})
	} else if *capacity < MinCapacity || *capacity > MaxCapacity {
		errs = append(errs, httpx.FieldError{Field: "capacity", Message: "ต้องเป็นจำนวนเต็ม 1–500"})
	}

	var open, close time.Duration
	var okOpen, okClose bool
	if openS != nil {
		open, okOpen = ParseClock(*openS)
	}
	if closeS != nil {
		close, okClose = ParseClock(*closeS)
	}
	if !okOpen {
		errs = append(errs, httpx.FieldError{Field: "open_time", Message: "ต้องเป็นเวลา HH:MM ที่นาทีเป็น 00 หรือ 30"})
	}
	if !okClose {
		errs = append(errs, httpx.FieldError{Field: "close_time", Message: "ต้องเป็นเวลา HH:MM ที่นาทีเป็น 00 หรือ 30"})
	}
	if okOpen && okClose && open == close {
		errs = append(errs, httpx.FieldError{Field: "close_time", Message: "เวลาปิดต้องไม่เท่ากับเวลาเปิด"})
	}
	return errs, open, close
}

// checkPolicy ตรวจนโยบายร้าน → 422 invalid-restaurant-policy
// (ผิดกติกาเสมอไม่ว่าข้อมูลจะเป็นอย่างไร จึงเป็น 422 ไม่ใช่ 400 หรือ 409)
func checkPolicy(cancelWindow, minLead int) error {
	switch {
	case cancelWindow < MinCancelWindow:
		return httpx.New("invalid-restaurant-policy", "ยกเลิกล่วงหน้าต้องไม่น้อยกว่า 30 นาที")
	case minLead < cancelWindow:
		return httpx.New("invalid-restaurant-policy", "จองล่วงหน้าขั้นต่ำต้องไม่น้อยกว่าเวลายกเลิกล่วงหน้า")
	}
	return nil
}
