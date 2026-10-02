package booking

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"devpool/internal/httpx"
)

// MyBooking คือรายการในหน้า "การจองของฉัน" (01 §7.4)
// is_past, can_modify, restaurant_deleted คำนวณตอนอ่าน ไม่เก็บลงฐานข้อมูล
type MyBooking struct {
	ID                int64     `json:"id"`
	RestaurantID      int64     `json:"restaurant_id"`
	RestaurantName    string    `json:"restaurant_name"`
	PartySize         int       `json:"party_size"`
	StartAt           time.Time `json:"start_at"`
	EndAt             time.Time `json:"end_at"`
	Status            string    `json:"status"`
	CancelDeadline    time.Time `json:"cancel_deadline"`
	IsPast            bool      `json:"is_past"`
	CanModify         bool      `json:"can_modify"`
	RestaurantDeleted bool      `json:"restaurant_deleted"`
}

type TabCounts struct {
	Upcoming  int `json:"upcoming"`
	Past      int `json:"past"`
	Cancelled int `json:"cancelled"`
}

type MyBookingsResult struct {
	httpx.PageResult[MyBooking]
	Counts TabCounts `json:"counts"`
}

// tabs แมปชื่อแท็บกับเงื่อนไขและลำดับ ($2 คือ now) เป็นสตริงคงที่ ไม่มีค่าจากผู้ใช้ปนใน SQL
// (SELECT ใช้ $2 คำนวณ is_past เสมอ แท็บ cancelled ที่ไม่มี $2 ใน WHERE จึงยังบอกชนิดของ $2 ได้)
var tabs = map[string]struct{ where, order string }{
	"upcoming":  {"b.status = 'active' AND b.end_at > $2", "b.start_at ASC, b.id ASC"},
	"past":      {"b.status = 'active' AND b.end_at <= $2", "b.start_at DESC, b.id DESC"},
	"cancelled": {"b.status = 'cancelled'", "b.cancelled_at DESC, b.id DESC"},
}

// ValidTab ตรวจชื่อแท็บ ค่าว่างคือ upcoming
func ValidTab(tab string) (string, error) {
	if tab == "" {
		return "upcoming", nil
	}
	if _, ok := tabs[tab]; !ok {
		return "", httpx.Validation("แท็บไม่ถูกต้อง",
			httpx.FieldError{Field: "tab", Message: "ต้องเป็น upcoming, past หรือ cancelled"})
	}
	return tab, nil
}

// MyBookings คืนการจองของผู้ใช้ตามแท็บ รวมการจองของร้านที่ถูกลบแล้ว (เป็นประวัติของลูกค้า)
func (s *Service) MyBookings(ctx context.Context, userID int64, tab string, page httpx.Page, now time.Time) (MyBookingsResult, error) {
	var res MyBookingsResult

	err := s.Pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status = 'active' AND end_at > $2),
		       count(*) FILTER (WHERE status = 'active' AND end_at <= $2),
		       count(*) FILTER (WHERE status = 'cancelled')
		  FROM bookings WHERE user_id = $1`, userID, now).
		Scan(&res.Counts.Upcoming, &res.Counts.Past, &res.Counts.Cancelled)
	if err != nil {
		return res, err
	}

	t := tabs[tab]
	rows, err := s.Pool.Query(ctx, `
		SELECT b.id, b.restaurant_id, r.name, b.party_size, b.start_at, b.end_at,
		       b.status, b.cancel_deadline, r.deleted_at IS NOT NULL, b.end_at <= $2
		  FROM bookings b JOIN restaurants r ON r.id = b.restaurant_id
		 WHERE b.user_id = $1 AND `+t.where+`
		 ORDER BY `+t.order+`
		 LIMIT $3 OFFSET $4`,
		userID, now, page.Size, page.Offset())
	if err != nil {
		return res, err
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (MyBooking, error) {
		var m MyBooking
		err := row.Scan(&m.ID, &m.RestaurantID, &m.RestaurantName, &m.PartySize, &m.StartAt, &m.EndAt,
			&m.Status, &m.CancelDeadline, &m.RestaurantDeleted, &m.IsPast)
		m.StartAt, m.EndAt, m.CancelDeadline = m.StartAt.In(Bangkok), m.EndAt.In(Bangkok), m.CancelDeadline.In(Bangkok)
		m.CanModify = m.Status == "active" && CanModify(now, m.CancelDeadline) && !m.RestaurantDeleted
		return m, err
	})
	if err != nil {
		return res, err
	}

	total := map[string]int{"upcoming": res.Counts.Upcoming, "past": res.Counts.Past, "cancelled": res.Counts.Cancelled}[tab]
	res.PageResult = httpx.NewPageResult(items, page, total)
	return res, nil
}
