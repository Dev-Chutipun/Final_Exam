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

// RestaurantBookings คือรายการการจองของร้านแบบแบ่งหน้า (ไม่ผูกกับวันใดวันหนึ่ง)
// ไม่มี peak เพราะ "ใช้ที่นั่งพร้อมกันสูงสุด" วัดได้เฉพาะในหน้าต่างเปิดร้านของวันเดียว
type RestaurantBookings struct {
	httpx.PageResult[DailyBooking]
	RestaurantID int64             `json:"restaurant_id"`
	Capacity     int               `json:"capacity"`
	Counts       booking.TabCounts `json:"counts"`
}

// ownerTabs แมปชื่อแท็บกับเงื่อนไขและลำดับ ($2 คือ now) เป็นสตริงคงที่ ไม่มีค่าจากผู้ใช้ปนใน SQL
// ใช้นิยามเดียวกับแท็บของลูกค้าใน booking.tabs เพื่อให้ตัวเลขสองฝั่งตรงกัน
// แท็บ cancelled ไม่ได้ใช้ now ในการกรอง แต่ต้องอ้างถึง $2 ไว้ ไม่อย่างนั้น PostgreSQL
// หาชนิดของพารามิเตอร์ไม่เจอเพราะไม่มีที่ไหนในคำสั่งบอกชนิดเลย (SQLSTATE 42P18)
var ownerTabs = map[string]struct{ where, order string }{
	"upcoming":  {"b.status = 'active' AND b.end_at > $2::timestamptz", "b.start_at ASC, b.id ASC"},
	"past":      {"b.status = 'active' AND b.end_at <= $2::timestamptz", "b.start_at DESC, b.id DESC"},
	"cancelled": {"b.status = 'cancelled' AND $2::timestamptz IS NOT NULL", "b.cancelled_at DESC, b.id DESC"},
}

// ValidBookingTab ตรวจชื่อแท็บ ค่าว่างคือ upcoming
func ValidBookingTab(tab string) (string, error) {
	if tab == "" {
		return "upcoming", nil
	}
	if _, ok := ownerTabs[tab]; !ok {
		return "", httpx.Validation("แท็บไม่ถูกต้อง",
			httpx.FieldError{Field: "tab", Message: "ต้องเป็น upcoming, past หรือ cancelled"})
	}
	return tab, nil
}

// Bookings คืนการจองทั้งหมดของร้านตามแท็บ แบ่งหน้า เฉพาะเจ้าของร้าน (03 §4.7)
// มีไว้ให้เจ้าของเห็นว่าลูกค้าจองวันไหนบ้างโดยไม่ต้องไล่เปิดทีละวัน
func (s *Service) Bookings(ctx context.Context, userID, restaurantID int64, tab string, page httpx.Page, now time.Time) (RestaurantBookings, error) {
	var res RestaurantBookings
	r, err := s.ownedRestaurant(ctx, userID, restaurantID)
	if err != nil {
		return res, err
	}
	res.RestaurantID, res.Capacity = r.ID, r.Capacity

	err = s.Pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status = 'active' AND end_at > $2),
		       count(*) FILTER (WHERE status = 'active' AND end_at <= $2),
		       count(*) FILTER (WHERE status = 'cancelled')
		  FROM bookings WHERE restaurant_id = $1`, r.ID, now).
		Scan(&res.Counts.Upcoming, &res.Counts.Past, &res.Counts.Cancelled)
	if err != nil {
		return res, err
	}

	t := ownerTabs[tab]
	rows, err := s.Pool.Query(ctx, `
		SELECT b.id, b.start_at, b.end_at, b.party_size, b.status, u.display_name
		  FROM bookings b JOIN users u ON u.id = b.user_id
		 WHERE b.restaurant_id = $1 AND `+t.where+`
		 ORDER BY `+t.order+`
		 LIMIT $3 OFFSET $4`, r.ID, now, page.Size, page.Offset())
	if err != nil {
		return res, err
	}
	items, err := pgx.CollectRows(rows, scanDailyBooking)
	if err != nil {
		return res, err
	}
	total := map[string]int{"upcoming": res.Counts.Upcoming, "past": res.Counts.Past, "cancelled": res.Counts.Cancelled}[tab]
	res.PageResult = httpx.NewPageResult(items, page, total)
	return res, nil
}

// ownedRestaurant อ่านร้านแล้วตรวจว่าเป็นของผู้ใช้คนนี้ (อ่านอย่างเดียว ไม่ต้องล็อกแถว)
func (s *Service) ownedRestaurant(ctx context.Context, userID, restaurantID int64) (db.Restaurant, error) {
	r, err := db.GetRestaurant(ctx, s.Pool, restaurantID)
	if errors.Is(err, db.ErrNotFound) {
		return r, httpx.New("not-found", "ไม่พบร้านนี้")
	}
	if err != nil {
		return r, err
	}
	if r.OwnerID != userID {
		return r, httpx.New("not-restaurant-owner", "ดูได้เฉพาะร้านของตัวเอง")
	}
	return r, nil
}

func scanDailyBooking(row pgx.CollectableRow) (DailyBooking, error) {
	var b DailyBooking
	err := row.Scan(&b.ID, &b.StartAt, &b.EndAt, &b.PartySize, &b.Status, &b.CustomerDisplayName)
	b.StartAt, b.EndAt = b.StartAt.In(booking.Bangkok), b.EndAt.In(booking.Bangkok)
	return b, err
}

// DailyBookings คืนการจองที่ start_at อยู่ในหน้าต่างเปิดร้านของวันทำการ day
// (นิยามเดียวกับ availability จึงรองรับร้านข้ามเที่ยงคืน) เฉพาะเจ้าของร้าน
func (s *Service) DailyBookings(ctx context.Context, userID, restaurantID int64, day time.Time) (DailyBookings, error) {
	r, err := s.ownedRestaurant(ctx, userID, restaurantID)
	if err != nil {
		return DailyBookings{}, err
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
	items, err := pgx.CollectRows(rows, scanDailyBooking)
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
