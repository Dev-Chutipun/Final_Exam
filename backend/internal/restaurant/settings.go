package restaurant

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"devpool/internal/booking"
	"devpool/internal/db"
	"devpool/internal/httpx"
)

type Service struct {
	Pool *pgxpool.Pool
}

// Settings คือค่าที่ส่งมาใน PATCH /api/restaurants/{id}/settings (ผ่านการตรวจรูปแบบแล้ว)
type Settings struct {
	Capacity            int
	Open, Close         time.Duration
	CancelWindowMinutes int
	MinLeadMinutes      int
}

type SettingsResponse struct {
	ID                  int64  `json:"id"`
	Capacity            int    `json:"capacity"`
	OpenTime            string `json:"open_time"`
	CloseTime           string `json:"close_time"`
	CancelWindowMinutes int    `json:"cancel_window_minutes"`
	MinLeadMinutes      int    `json:"min_lead_minutes"`
}

// Conflict คือการจองที่ทำให้ตั้งค่าใหม่ไม่ได้
type Conflict struct {
	BookingID int64     `json:"booking_id"`
	StartAt   time.Time `json:"start_at"`
	EndAt     time.Time `json:"end_at"`
	Reason    string    `json:"reason"` // capacity หรือ opening_hours
}

// UpdateSettings แก้จำนวนที่นั่ง เวลาเปิด–ปิด และนโยบาย ตาม 01 §7.6
//
// ล็อกแถวร้านก่อนตรวจการจอง: ระหว่างตรวจจะไม่มีการจองใหม่แทรกเข้ามา
// (การจองใหม่ต้องรอ lock เดียวกัน และเมื่อได้ lock จะเห็นค่าที่นั่งใหม่แล้ว)
func (s *Service) UpdateSettings(ctx context.Context, userID, restaurantID int64, in Settings, now time.Time) (SettingsResponse, error) {
	err := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		r, err := db.LockRestaurant(ctx, tx, restaurantID)
		if errors.Is(err, db.ErrNotFound) {
			return httpx.New("not-found", "ไม่พบร้านนี้")
		}
		if err != nil {
			return err
		}
		if r.OwnerID != userID {
			return httpx.New("not-restaurant-owner", "แก้ได้เฉพาะร้านของตัวเอง")
		}
		if err := checkPolicy(in.CancelWindowMinutes, in.MinLeadMinutes); err != nil {
			return err
		}

		if err := checkFutureBookings(ctx, tx, r.ID, in, now); err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
			UPDATE restaurants
			   SET capacity = $2, open_time = $3, close_time = $4,
			       cancel_window_minutes = $5, min_lead_minutes = $6, updated_at = $7
			 WHERE id = $1`,
			r.ID, in.Capacity, FormatClock(in.Open), FormatClock(in.Close),
			in.CancelWindowMinutes, in.MinLeadMinutes, now)
		return err
	})
	if err != nil {
		return SettingsResponse{}, err
	}
	return SettingsResponse{
		ID:                  restaurantID,
		Capacity:            in.Capacity,
		OpenTime:            FormatClock(in.Open),
		CloseTime:           FormatClock(in.Close),
		CancelWindowMinutes: in.CancelWindowMinutes,
		MinLeadMinutes:      in.MinLeadMinutes,
	}, nil
}

// checkFutureBookings ตรวจว่าการจอง active ที่ยังไม่จบ ยังถูกกติกาภายใต้ค่าใหม่
// (กติกาต้องจริง "ไม่ว่าเวลาไหน" ไม่ใช่แค่ตอนจอง — สไลด์ 7)
func checkFutureBookings(ctx context.Context, tx pgx.Tx, restaurantID int64, in Settings, now time.Time) error {
	rows, err := tx.Query(ctx, `
		SELECT id, start_at, end_at, party_size FROM bookings
		 WHERE restaurant_id = $1 AND status = 'active' AND end_at > $2
		 ORDER BY start_at, id`, restaurantID, now)
	if err != nil {
		return err
	}
	type row struct {
		id int64
		booking.Interval
	}
	list, err := pgx.CollectRows(rows, func(cr pgx.CollectableRow) (row, error) {
		var b row
		err := cr.Scan(&b.id, &b.Start, &b.End, &b.Size)
		return b, err
	})
	if err != nil {
		return err
	}

	var conflicts []Conflict
	add := func(b row, reason string) {
		conflicts = append(conflicts, Conflict{b.id, b.Start.In(booking.Bangkok), b.End.In(booking.Bangkok), reason})
	}

	// ที่นั่ง: ส่งเฉพาะการจองที่ครอบจุด peak แรก (เจ้าของแก้แล้วบันทึกใหม่จะเห็นจุดถัดไป)
	intervals := make([]booking.Interval, len(list))
	for i, b := range list {
		intervals[i] = b.Interval
	}
	peak, peakAt := booking.PeakAll(intervals)
	detail := ""
	if peak > in.Capacity {
		for _, b := range list {
			if !b.Start.After(peakAt) && peakAt.Before(b.End) {
				add(b, "capacity")
			}
		}
		detail = fmt.Sprintf("เวลา %s มีการจองรวม %d ที่ เกินที่นั่งใหม่ %d ที่ (อาจมีช่วงอื่นที่ชนอีก) ",
			peakAt.In(booking.Bangkok).Format("02/01 15:04"), peak, in.Capacity)
	}

	// เวลาเปิด: ส่งทุกการจองที่หลุดหน้าต่างเปิดใหม่
	outside := 0
	for _, b := range list {
		if !booking.WithinOpeningHours(b.Start, b.End, booking.Hours{Open: in.Open, Close: in.Close}) {
			add(b, "opening_hours")
			outside++
		}
	}
	if outside > 0 {
		detail += fmt.Sprintf("มีการจอง %d รายการอยู่นอกเวลาเปิดใหม่", outside)
	}

	if len(conflicts) > 0 {
		return httpx.New("settings-conflict-with-bookings", detail).With("conflicts", conflicts)
	}
	return nil
}
