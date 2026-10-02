package booking

import (
	"context"
	"errors"
	"time"

	"devpool/internal/db"
	"devpool/internal/httpx"
)

type Slot struct {
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	Remaining int       `json:"remaining"`
	Limited   bool      `json:"limited"`
	Bookable  bool      `json:"bookable"`
}

type Availability struct {
	RestaurantID      int64  `json:"restaurant_id"`
	Date              string `json:"date"`
	Capacity          int    `json:"capacity"`
	AcceptingBookings bool   `json:"accepting_bookings"`
	Slots             []Slot `json:"slots"`
}

// ParseDate อ่านวันที่ YYYY-MM-DD เป็นเที่ยงคืนของวันนั้นตามเวลา Bangkok
func ParseDate(field, s string) (time.Time, error) {
	d, err := time.ParseInLocation("2006-01-02", s, Bangkok)
	if err != nil {
		return time.Time{}, httpx.Validation("วันที่ไม่ถูกต้อง",
			httpx.FieldError{Field: field, Message: "ต้องเป็นรูปแบบ YYYY-MM-DD"})
	}
	return d, nil
}

// Availability คืนที่ว่างทีละ 30 นาทีในหน้าต่างเปิดของวันทำการ day (01 §7.5)
//
// ไม่ล็อก ผลลัพธ์เป็น "คำแนะนำ" สำหรับหน้าเว็บเท่านั้น การตัดสินจริงเกิดใน transaction ของ Create
func (s *Service) Availability(ctx context.Context, restaurantID int64, day, now time.Time) (Availability, error) {
	r, err := db.GetRestaurant(ctx, s.Pool, restaurantID)
	if errors.Is(err, db.ErrNotFound) {
		return Availability{}, httpx.New("not-found", "ไม่พบร้านนี้")
	}
	if err != nil {
		return Availability{}, err
	}

	ws, we := OpeningWindow(day, Hours{Open: r.OpenTime, Close: r.CloseTime})
	rows, err := s.Pool.Query(ctx, `
		SELECT start_at, end_at, party_size FROM bookings
		 WHERE restaurant_id = $1 AND status = 'active'
		   AND start_at < $3 AND end_at > $2`,
		r.ID, ws, we)
	if err != nil {
		return Availability{}, err
	}
	var existing []Interval
	for rows.Next() {
		var iv Interval
		if err := rows.Scan(&iv.Start, &iv.End, &iv.Size); err != nil {
			rows.Close()
			return Availability{}, err
		}
		existing = append(existing, iv)
	}
	if err := rows.Err(); err != nil {
		return Availability{}, err
	}

	a := Availability{
		RestaurantID:      r.ID,
		Date:              day.In(Bangkok).Format("2006-01-02"),
		Capacity:          r.Capacity,
		AcceptingBookings: r.AcceptingBookings,
		Slots:             []Slot{},
	}
	for t := ws; t.Before(we); t = t.Add(SlotSize) {
		remaining := r.Capacity - loadAt(existing, t)
		a.Slots = append(a.Slots, Slot{
			Start:     t.In(Bangkok),
			End:       t.Add(SlotSize).In(Bangkok),
			Remaining: remaining,
			// เหลือ ≤ 30% ใช้จำนวนเต็มเพื่อเลี่ยงทศนิยม
			Limited:  remaining > 0 && remaining*10 <= r.Capacity*3,
			Bookable: r.AcceptingBookings && remaining > 0 && CheckTiming(t, now, r.MinLeadMinutes) == nil,
		})
	}
	return a, nil
}
