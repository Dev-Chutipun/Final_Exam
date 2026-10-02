package booking

import (
	"fmt"
	"time"

	"devpool/internal/httpx"
)

// Bangkok คือ timezone ธุรกิจ (UTC+7 ไม่มี DST) โหลดครั้งเดียวตอนเริ่มโปรแกรม
// cmd/api ต้อง import _ "time/tzdata" เพื่อให้โหลดได้แม้เครื่องไม่มีฐานข้อมูล timezone
var Bangkok = mustLoadLocation("Asia/Bangkok")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic("โหลด timezone " + name + " ไม่ได้: " + err.Error())
	}
	return loc
}

const (
	SlotSize    = 30 * time.Minute    // ช่องเวลา
	MinDuration = 30 * time.Minute    // จองสั้นสุด
	MaxDuration = 3 * time.Hour       // จองยาวสุด
	MaxAdvance  = 30 * 24 * time.Hour // จองล่วงหน้าได้ไม่เกิน 30 วัน
)

// Hours คือเวลาเปิด–ปิดของร้าน นับจากเที่ยงคืน
// Close < Open แปลว่าข้ามเที่ยงคืน (เท่ากันไม่ได้ ตรวจตอนตั้งค่าร้าน)
type Hours struct {
	Open, Close time.Duration
}

// ValidateShape ตรวจรูปแบบเวลาก่อนเข้า transaction (ขั้น 2 ของ 01 §7.1) → 400 validation
//   - start และ end ลงช่อง 30 นาทีตามเวลา Bangkok
//   - start < end และระยะเวลา 30 นาที–3 ชั่วโมง
func ValidateShape(start, end time.Time) error {
	var errs []httpx.FieldError
	if !OnSlot(start) {
		errs = append(errs, httpx.FieldError{Field: "start_at", Message: "ต้องเป็นนาทีที่ 00 หรือ 30"})
	}
	if !OnSlot(end) {
		errs = append(errs, httpx.FieldError{Field: "end_at", Message: "ต้องเป็นนาทีที่ 00 หรือ 30"})
	}
	if d := end.Sub(start); d < MinDuration || d > MaxDuration {
		errs = append(errs, httpx.FieldError{Field: "end_at", Message: "ต้องหลังเวลาเริ่ม 30 นาทีถึง 3 ชั่วโมง"})
	}
	if len(errs) > 0 {
		return httpx.Validation("เวลาจองไม่ถูกต้อง", errs...)
	}
	return nil
}

// OnSlot บอกว่า t ลงช่อง 30 นาทีหรือไม่ เมื่อดูเป็นเวลา Bangkok
// (นาที 0 หรือ 30 และวินาทีกับ nanosecond เป็น 0)
func OnSlot(t time.Time) bool {
	l := t.In(Bangkok)
	return (l.Minute() == 0 || l.Minute() == 30) && l.Second() == 0 && l.Nanosecond() == 0
}

// CheckTiming ตรวจเงื่อนไขเทียบกับ now ตามลำดับใน 01 §5 → 422
// เรียงลำดับนี้เพื่อให้ error บอกเหตุผลตรงที่สุด
func CheckTiming(start, now time.Time, minLeadMinutes int) error {
	lead := time.Duration(minLeadMinutes) * time.Minute
	switch {
	case !start.After(now):
		return httpx.New("booking-in-past", "เวลาเริ่มผ่านไปแล้ว")
	case start.Before(now.Add(lead)):
		return httpx.New("lead-time-not-met",
			fmt.Sprintf("ร้านนี้ต้องจองล่วงหน้าอย่างน้อย %d นาที", minLeadMinutes))
	case start.After(now.Add(MaxAdvance)):
		return httpx.New("too-far-in-advance", "จองล่วงหน้าได้ไม่เกิน 30 วัน")
	}
	return nil
}

// OpeningWindow คืนหน้าต่างเปิดร้าน [ws, we) ของวันทำการ day (ใช้แค่ปี เดือน วัน ตามเวลา Bangkok)
// ร้านข้ามเที่ยงคืน เช่น 18:00–02:00 ของวันที่ 11 คือ 11 18:00 → 12 02:00
func OpeningWindow(day time.Time, h Hours) (ws, we time.Time) {
	y, m, d := day.In(Bangkok).Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, Bangkok)
	ws = midnight.Add(h.Open)
	if h.Close > h.Open {
		we = midnight.Add(h.Close)
	} else {
		we = midnight.AddDate(0, 0, 1).Add(h.Close)
	}
	return ws, we
}

// WithinOpeningHours บอกว่า [start, end) อยู่ในหน้าต่างเปิดร้านหน้าต่างเดียวตลอดทั้งช่วงหรือไม่
//
// ลองหน้าต่างของวันทำการ D−1 และ D โดย D คือวันที่ของ start
// ต้องดู D−1 ด้วยเพราะร้านข้ามเที่ยงคืน: จอง 01:00 ของวันที่ 12 อยู่ในหน้าต่างของวันที่ 11
func WithinOpeningHours(start, end time.Time, h Hours) bool {
	d := start.In(Bangkok)
	for _, day := range []time.Time{d.AddDate(0, 0, -1), d} {
		ws, we := OpeningWindow(day, h)
		if !start.Before(ws) && !end.After(we) {
			return true
		}
	}
	return false
}

// CheckOpeningHours คือ WithinOpeningHours ในรูป error (ขั้น 7 ของ 01 §7.1) → 422
func CheckOpeningHours(start, end time.Time, h Hours) error {
	if !WithinOpeningHours(start, end, h) {
		return httpx.New("outside-opening-hours", "ช่วงเวลาที่จองอยู่นอกเวลาเปิดร้าน")
	}
	return nil
}

// CancelDeadline = start − cancel_window_minutes ใช้นโยบายของร้าน ณ ตอนสร้างหรือแก้การจอง
// แล้วบันทึกลงแถวการจอง นโยบายที่เปลี่ยนภายหลังจึงไม่ย้อนหลัง
func CancelDeadline(start time.Time, cancelWindowMinutes int) time.Time {
	return start.Add(-time.Duration(cancelWindowMinutes) * time.Minute)
}

// CanModify บอกว่ายังแก้ไขหรือยกเลิกได้หรือไม่: now ≤ deadline (นับรวมเวลาพอดี)
func CanModify(now, deadline time.Time) bool {
	return !now.After(deadline)
}
