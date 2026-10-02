package booking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"devpool/internal/db"
	"devpool/internal/httpx"
)

// Booking คือการจองที่ส่งกลับใน API เวลาเป็นเวลา Bangkok (+07:00)
type Booking struct {
	ID             int64     `json:"id"`
	RestaurantID   int64     `json:"restaurant_id"`
	PartySize      int       `json:"party_size"`
	StartAt        time.Time `json:"start_at"`
	EndAt          time.Time `json:"end_at"`
	Status         string    `json:"status"`
	CancelDeadline time.Time `json:"cancel_deadline"`
}

// Request คือค่าที่ผู้ใช้ส่งมาเมื่อจองหรือแก้ไข (parse แล้ว)
type Request struct {
	PartySize      int
	StartAt, EndAt time.Time
}

// Validate ตรวจรูปแบบก่อนเข้า transaction (ขั้น 2 ของ 01 §7.1) → 400 validation
func (r Request) Validate() error {
	var errs []httpx.FieldError
	if r.PartySize < 1 {
		errs = append(errs, httpx.FieldError{Field: "party_size", Message: "ต้องเป็นจำนวนเต็มตั้งแต่ 1"})
	}
	if err := ValidateShape(r.StartAt, r.EndAt); err != nil {
		var p *httpx.Problem
		errors.As(err, &p)
		errs = append(errs, p.Errors...)
	}
	if len(errs) > 0 {
		return httpx.Validation("ข้อมูลการจองไม่ถูกต้อง", errs...)
	}
	return nil
}

type Service struct {
	Pool *pgxpool.Pool
}

// Create จองใหม่ตาม 01 §7.1 (ขั้น 3–11)
// now มาจาก handler เสมอ service ไม่เรียก time.Now()
func (s *Service) Create(ctx context.Context, userID, restaurantID int64, req Request, now time.Time) (Booking, error) {
	if err := req.Validate(); err != nil {
		return Booking{}, err
	}

	var b Booking
	err := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		// ขั้น 3: ล็อกแถวร้านก่อนอ่านอะไรทั้งหมด คำขอจองร้านเดียวกันจะต่อคิวกันตรงนี้
		r, err := db.LockRestaurant(ctx, tx, restaurantID)
		if errors.Is(err, db.ErrNotFound) {
			return httpx.New("not-found", "ไม่พบร้านนี้")
		}
		if err != nil {
			return err
		}

		// ขั้น 4–5: ใช้ค่าร้านที่อ่านหลังล็อกเท่านั้น
		if r.OwnerID == userID {
			return httpx.New("own-restaurant-booking", "จองร้านของตัวเองไม่ได้")
		}
		if !r.AcceptingBookings {
			return httpx.New("restaurant-not-accepting-bookings", "ร้านนี้พักรับจองชั่วคราว")
		}

		// ขั้น 6–10: กติกาที่ใช้ร่วมกับการแก้ไข (excludeID = 0 คือไม่ตัดการจองใด)
		if err := checkSeats(ctx, tx, r, userID, req, now, 0); err != nil {
			return err
		}

		// ขั้น 11: บันทึก deadline ตามนโยบายของร้าน ณ ตอนนี้
		b, err = insertBooking(ctx, tx, r, userID, req, now)
		return err
	})
	return b, err
}

// Update แก้การจองตาม 01 §7.2 (ส่งครบทุกค่า แทนค่าเดิมทั้งหมด ห้ามเปลี่ยนร้าน)
func (s *Service) Update(ctx context.Context, userID, bookingID int64, req Request, now time.Time) (Booking, error) {
	if err := req.Validate(); err != nil {
		return Booking{}, err
	}

	var b Booking
	err := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		// 2: หาร้านของการจอง (เฉพาะของผู้ใช้คนนี้ ไม่เปิดเผยว่าเป็นการจองของคนอื่น)
		var restaurantID int64
		err := tx.QueryRow(ctx,
			`SELECT restaurant_id FROM bookings WHERE id = $1 AND user_id = $2`,
			bookingID, userID).Scan(&restaurantID)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.New("not-found", "ไม่พบการจองนี้")
		}
		if err != nil {
			return err
		}

		// 3: ล็อกตามลำดับ ร้าน → การจอง
		r, err := db.LockRestaurant(ctx, tx, restaurantID)
		if errors.Is(err, db.ErrNotFound) {
			return httpx.New("not-found", "ไม่พบร้านนี้")
		}
		if err != nil {
			return err
		}
		var status string
		var deadline time.Time
		err = tx.QueryRow(ctx,
			`SELECT status, cancel_deadline FROM bookings WHERE id = $1 FOR UPDATE`,
			bookingID).Scan(&status, &deadline)
		if err != nil {
			return err
		}

		// 4–6
		if status != "active" {
			return httpx.New("booking-not-active", "การจองนี้ถูกยกเลิกแล้ว แก้ไขไม่ได้")
		}
		if !CanModify(now, deadline) {
			return httpx.New("cancel-deadline-passed",
				"แก้ไขได้ถึง "+deadline.In(Bangkok).Format("02/01/2006 15:04")+" เท่านั้น")
		}
		if !r.AcceptingBookings {
			return httpx.New("restaurant-not-accepting-bookings", "ร้านนี้พักรับจองชั่วคราว แก้ไขการจองไม่ได้")
		}

		// 7: กติกาเดียวกับจองใหม่ โดยตัดการจองนี้ออก (ไม่นับที่นั่งของตัวเองซ้ำ — สไลด์ 8)
		if err := checkSeats(ctx, tx, r, userID, req, now, bookingID); err != nil {
			return err
		}

		// 8: deadline ใหม่ตามนโยบายปัจจุบันของร้าน
		b = Booking{
			ID:             bookingID,
			RestaurantID:   r.ID,
			PartySize:      req.PartySize,
			StartAt:        req.StartAt.In(Bangkok),
			EndAt:          req.EndAt.In(Bangkok),
			Status:         "active",
			CancelDeadline: CancelDeadline(req.StartAt, r.CancelWindowMinutes).In(Bangkok),
		}
		_, err = tx.Exec(ctx, `
			UPDATE bookings
			   SET party_size = $2, start_at = $3, end_at = $4, cancel_deadline = $5, updated_at = $6
			 WHERE id = $1`,
			b.ID, b.PartySize, b.StartAt, b.EndAt, b.CancelDeadline, now)
		return err
	})
	return b, err
}

// Cancel ยกเลิกการจองตาม 01 §7.3
// ไม่เช็คที่นั่งและไม่ล็อกแถวร้าน: การยกเลิกมีแต่ทำให้ที่นั่งว่างขึ้น จึงไม่มีทางทำให้จองเกิน
// ใช้ UPDATE แบบมีเงื่อนไขคำสั่งเดียว เงื่อนไขทั้งหมดถูกตรวจแบบ atomic ในคำสั่งนั้น
func (s *Service) Cancel(ctx context.Context, userID, bookingID int64, now time.Time) (Booking, error) {
	var b Booking
	err := s.Pool.QueryRow(ctx, `
		UPDATE bookings
		   SET status = 'cancelled', cancelled_at = $3, updated_at = $3
		 WHERE id = $1 AND user_id = $2 AND status = 'active' AND $3 <= cancel_deadline
		RETURNING id, restaurant_id, party_size, start_at, end_at, status, cancel_deadline`,
		bookingID, userID, now).
		Scan(&b.ID, &b.RestaurantID, &b.PartySize, &b.StartAt, &b.EndAt, &b.Status, &b.CancelDeadline)
	if err == nil {
		return b.inBangkok(), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, err
	}

	// ไม่มีแถวถูกอัปเดต: อ่านแถวนั้นเพื่อบอกสาเหตุ
	var status string
	var deadline time.Time
	err = s.Pool.QueryRow(ctx,
		`SELECT status, cancel_deadline FROM bookings WHERE id = $1 AND user_id = $2`,
		bookingID, userID).Scan(&status, &deadline)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Booking{}, httpx.New("not-found", "ไม่พบการจองนี้")
	case err != nil:
		return Booking{}, err
	case status == "cancelled":
		return Booking{}, httpx.New("already-cancelled", "การจองนี้ถูกยกเลิกไปแล้ว")
	default:
		return Booking{}, httpx.New("cancel-deadline-passed",
			"ยกเลิกได้ถึง "+deadline.In(Bangkok).Format("02/01/2006 15:04")+" เท่านั้น")
	}
}

func (b Booking) inBangkok() Booking {
	b.StartAt = b.StartAt.In(Bangkok)
	b.EndAt = b.EndAt.In(Bangkok)
	b.CancelDeadline = b.CancelDeadline.In(Bangkok)
	return b
}

// checkSeats คือขั้น 6–10 ของ 01 §7.1 ต้องเรียกหลัง LockRestaurant
// excludeID คือการจองที่กำลังแก้ไข ซึ่งต้องตัดออกไม่ให้นับที่นั่งของตัวเองซ้ำ (สไลด์ 8)
func checkSeats(ctx context.Context, tx pgx.Tx, r db.Restaurant, userID int64, req Request, now time.Time, excludeID int64) error {
	// 6: เทียบกับ now
	if err := CheckTiming(req.StartAt, now, r.MinLeadMinutes); err != nil {
		return err
	}
	// 7: อยู่ในหน้าต่างเปิดร้าน
	if err := CheckOpeningHours(req.StartAt, req.EndAt, Hours{Open: r.OpenTime, Close: r.CloseTime}); err != nil {
		return err
	}
	// 8: จำนวนคนไม่เกินที่นั่งทั้งร้าน
	if req.PartySize > r.Capacity {
		return httpx.New("party-exceeds-capacity",
			fmt.Sprintf("ร้านนี้มีทั้งหมด %d ที่ แต่ขอ %d ที่", r.Capacity, req.PartySize))
	}

	// 9: ผู้ใช้ไม่มีการจอง active อื่นที่ทับช่วงนี้ในร้านเดียวกัน (ทับ = start_at < e AND end_at > s)
	var own bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM bookings
		   WHERE restaurant_id = $1 AND user_id = $2 AND status = 'active'
		     AND start_at < $4 AND end_at > $3 AND id <> $5)`,
		r.ID, userID, req.StartAt, req.EndAt, excludeID).Scan(&own)
	if err != nil {
		return err
	}
	if own {
		return httpx.New("overlapping-own-booking", "คุณมีการจองที่ร้านนี้ในช่วงเวลาที่ทับกันอยู่แล้ว")
	}

	// 10: ที่นั่งพอที่ทุกจุดของช่วง (นับเฉพาะ active)
	existing, err := activeOverlapping(ctx, tx, r.ID, req.StartAt, req.EndAt, excludeID)
	if err != nil {
		return err
	}
	load, at := MaxLoad(existing, req.StartAt, req.EndAt)
	if req.PartySize+load > r.Capacity {
		available := max(r.Capacity-load, 0)
		return httpx.New("seat-capacity-exceeded",
			fmt.Sprintf("ตั้งแต่ %s เหลือ %d ที่ แต่ขอ %d ที่", at.In(Bangkok).Format("15:04"), available, req.PartySize)).
			With("available", available).
			With("full_from", at.In(Bangkok).Format(time.RFC3339))
	}
	return nil
}

// activeOverlapping คืนการจอง active ของร้านที่ทับช่วง [s, e)
func activeOverlapping(ctx context.Context, tx pgx.Tx, restaurantID int64, s, e time.Time, excludeID int64) ([]Interval, error) {
	rows, err := tx.Query(ctx, `
		SELECT start_at, end_at, party_size FROM bookings
		 WHERE restaurant_id = $1 AND status = 'active'
		   AND start_at < $3 AND end_at > $2 AND id <> $4`,
		restaurantID, s, e, excludeID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Interval, error) {
		var iv Interval
		err := row.Scan(&iv.Start, &iv.End, &iv.Size)
		return iv, err
	})
}

func insertBooking(ctx context.Context, tx pgx.Tx, r db.Restaurant, userID int64, req Request, now time.Time) (Booking, error) {
	b := Booking{
		RestaurantID:   r.ID,
		PartySize:      req.PartySize,
		StartAt:        req.StartAt.In(Bangkok),
		EndAt:          req.EndAt.In(Bangkok),
		Status:         "active",
		CancelDeadline: CancelDeadline(req.StartAt, r.CancelWindowMinutes).In(Bangkok),
	}
	err := tx.QueryRow(ctx, `
		INSERT INTO bookings (restaurant_id, user_id, party_size, start_at, end_at, cancel_deadline, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		RETURNING id`,
		b.RestaurantID, userID, b.PartySize, b.StartAt, b.EndAt, b.CancelDeadline, now).Scan(&b.ID)
	return b, err
}
