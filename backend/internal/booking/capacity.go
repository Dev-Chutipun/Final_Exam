// Package booking คือการจอง: อัลกอริทึมที่นั่ง (capacity.go), กติกาเวลา (rules.go)
// สองไฟล์นี้เป็น pure function ไม่แตะฐานข้อมูลและไม่เรียก time.Now()
package booking

import (
	"slices"
	"time"
)

// Interval คือการจองหนึ่งรายการ ช่วงเวลาแบบ half-open [Start, End)
// จบ 13:00 กับเริ่ม 13:00 จึงไม่ทับกัน
type Interval struct {
	Start, End time.Time
	Size       int
}

// MaxLoad คืนโหลดสูงสุดของ existing ภายในช่วง [s, e) และจุดเวลาแรกที่เกิดโหลดนั้น
//
// จุดตรวจ = s และ Start ของทุกการจองที่เริ่มภายใน (s, e)
// เหตุผล: ภายใน [s, e) โหลดจะเพิ่มขึ้นได้เฉพาะตอนมีการจองเริ่ม
// ค่าสูงสุดจึงต้องอยู่ที่จุดใดจุดหนึ่งในชุดนี้เสมอ ไม่ต้องไล่ทุกนาที
//
// existing มีช่วงที่ไม่ทับ [s, e) ปนมาได้ เพราะ loadAt นับเฉพาะช่วงที่ครอบจุดนั้น
// ไม่มีการจองเลย → 0, s
func MaxLoad(existing []Interval, s, e time.Time) (load int, at time.Time) {
	points := []time.Time{s}
	for _, b := range existing {
		if b.Start.After(s) && b.Start.Before(e) {
			points = append(points, b.Start)
		}
	}
	slices.SortFunc(points, func(a, b time.Time) int { return a.Compare(b) })

	load, at = 0, s
	for _, t := range points {
		if l := loadAt(existing, t); l > load { // ใช้ > เพื่อให้ได้จุดแรกเมื่อเท่ากัน
			load, at = l, t
		}
	}
	return load, at
}

// loadAt คือผลรวมที่นั่งของการจองที่ครอบเวลา t ตามแบบ half-open: Start ≤ t < End
func loadAt(existing []Interval, t time.Time) int {
	sum := 0
	for _, b := range existing {
		if !b.Start.After(t) && t.Before(b.End) {
			sum += b.Size
		}
	}
	return sum
}

// PeakAll คืน peak ของทุกช่วงรวมกัน และจุดเวลาแรกที่เกิด peak (sweep line)
//
// แต่ละการจองให้ event +Size ที่ Start และ −Size ที่ End แล้วเดินตามเวลา
// ถ้าเวลาเท่ากัน event จบ (−) ต้องมาก่อน event เริ่ม (+) ซึ่งคือ half-open ในรูปของอัลกอริทึม:
// [12:00–13:00, 10] กับ [13:00–14:00, 10] มี peak 10 ไม่ใช่ 20
func PeakAll(all []Interval) (peak int, at time.Time) {
	type event struct {
		at    time.Time
		delta int
	}
	events := make([]event, 0, 2*len(all))
	for _, b := range all {
		events = append(events, event{b.Start, b.Size}, event{b.End, -b.Size})
	}
	slices.SortFunc(events, func(a, b event) int {
		if c := a.at.Compare(b.at); c != 0 {
			return c
		}
		return a.delta - b.delta // ค่าลบ (จบ) มาก่อนค่าบวก (เริ่ม)
	})

	cur := 0
	for _, ev := range events {
		cur += ev.delta
		if cur > peak {
			peak, at = cur, ev.at
		}
	}
	return peak, at
}
