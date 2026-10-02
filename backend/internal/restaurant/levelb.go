package restaurant

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"devpool/internal/booking"
	"devpool/internal/db"
	"devpool/internal/httpx"
)

// [B] พักรับจอง และการจองรายวันของร้าน (03 §4.4, §4.6)

// SetAcceptingBookings เปิดหรือพักรับจอง ส่งค่าเดิมซ้ำได้ไม่ถือว่าผิด
//
// ล็อกแถวร้านเหมือนงานจอง: คำขอจองที่กำลังรอ lock จะเห็นสถานะใหม่ทันทีหลังคำสั่งนี้ commit
func (s *Service) SetAcceptingBookings(ctx context.Context, userID, restaurantID int64, accepting bool, now time.Time) error {
	return db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockOwned(ctx, tx, restaurantID, userID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`UPDATE restaurants SET accepting_bookings = $2, updated_at = $3 WHERE id = $1`,
			restaurantID, accepting, now)
		return err
	})
}

type DailyBooking struct {
	ID                  int64     `json:"id"`
	StartAt             time.Time `json:"start_at"`
	EndAt               time.Time `json:"end_at"`
	PartySize           int       `json:"party_size"`
	Status              string    `json:"status"`
	CustomerDisplayName string    `json:"customer_display_name"` // ไม่มีอีเมล
}

type DailyBookings struct {
	RestaurantID int64          `json:"restaurant_id"`
	Date         string         `json:"date"`
	Capacity     int            `json:"capacity"`
	Peak         int            `json:"peak"`
	PeakAt       *time.Time     `json:"peak_at"` // null เมื่อไม่มีการจอง active
	Items        []DailyBooking `json:"items"`
}

// DailyBookings คืนการจองที่ start_at อยู่ในหน้าต่างเปิดร้านของวันทำการ day
// (นิยามเดียวกับ availability จึงรองรับร้านข้ามเที่ยงคืน) เฉพาะเจ้าของร้าน
func (s *Service) DailyBookings(ctx context.Context, userID, restaurantID int64, day time.Time) (DailyBookings, error) {
	r, err := db.GetRestaurant(ctx, s.Pool, restaurantID)
	if errors.Is(err, db.ErrNotFound) {
		return DailyBookings{}, httpx.New("not-found", "ไม่พบร้านนี้")
	}
	if err != nil {
		return DailyBookings{}, err
	}
	if r.OwnerID != userID {
		return DailyBookings{}, httpx.New("not-restaurant-owner", "ดูได้เฉพาะร้านของตัวเอง")
	}

	ws, we := booking.OpeningWindow(day, booking.Hours{Open: r.OpenTime, Close: r.CloseTime})
	rows, err := s.Pool.Query(ctx, `
		SELECT b.id, b.start_at, b.end_at, b.party_size, b.status, u.display_name
		  FROM bookings b JOIN users u ON u.id = b.user_id
		 WHERE b.restaurant_id = $1 AND b.start_at >= $2 AND b.start_at < $3
		 ORDER BY b.start_at, b.id`, r.ID, ws, we)
	if err != nil {
		return DailyBookings{}, err
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (DailyBooking, error) {
		var b DailyBooking
		err := row.Scan(&b.ID, &b.StartAt, &b.EndAt, &b.PartySize, &b.Status, &b.CustomerDisplayName)
		b.StartAt, b.EndAt = b.StartAt.In(booking.Bangkok), b.EndAt.In(booking.Bangkok)
		return b, err
	})
	if err != nil {
		return DailyBookings{}, err
	}

	var active []booking.Interval
	for _, b := range items {
		if b.Status == "active" {
			active = append(active, booking.Interval{Start: b.StartAt, End: b.EndAt, Size: b.PartySize})
		}
	}
	res := DailyBookings{
		RestaurantID: r.ID,
		Date:         day.In(booking.Bangkok).Format("2006-01-02"),
		Capacity:     r.Capacity,
		Items:        items,
	}
	if res.Items == nil {
		res.Items = []DailyBooking{}
	}
	if peak, at := booking.PeakAll(active); peak > 0 {
		at = at.In(booking.Bangkok)
		res.Peak, res.PeakAt = peak, &at
	}
	return res, nil
}
