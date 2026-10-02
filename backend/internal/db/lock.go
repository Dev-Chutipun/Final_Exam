package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Restaurant คือค่าของร้านที่ใช้ตัดสินใจในงานจองและงานจัดการร้าน
// ได้มาจาก LockRestaurant เท่านั้น จึงเป็นค่าที่อ่านหลังล็อกแล้วเสมอ
type Restaurant struct {
	ID                  int64
	OwnerID             int64
	Capacity            int
	OpenTime            time.Duration // เวลาเปิด นับจากเที่ยงคืน เช่น 10:00 = 10h
	CloseTime           time.Duration // close < open แปลว่าข้ามเที่ยงคืน
	CancelWindowMinutes int
	MinLeadMinutes      int
	AcceptingBookings   bool
}

// LockRestaurant ล็อกแถวร้านด้วย FOR UPDATE แล้วคืนค่าของร้าน ณ หลังล็อก
//
//   - กรอง deleted_at IS NULL ร้านที่ถูกลบจึงคืน ErrNotFound เหมือนร้านที่ไม่มีอยู่
//   - ทุก pipeline ที่อ่านค่าร้านเพื่อตัดสินใจต้องเรียกฟังก์ชันนี้ก่อนอ่านข้อมูลอื่น
//   - ลำดับการล็อกคือ ร้าน → การจอง เสมอ เพื่อกัน deadlock
//
// แถวร้านเป็นตัวแทนของ "ที่นั่งทั้งร้าน": การจองใหม่ยังไม่มีแถวให้ล็อก (phantom)
// ถ้าทุกคำขอจองต้องล็อกแถวร้านก่อน คำขอของร้านเดียวกันจะเข้าแถวทีละคำขอ
func LockRestaurant(ctx context.Context, tx pgx.Tx, id int64) (Restaurant, error) {
	var r Restaurant
	var openT, closeT pgtype.Time
	err := tx.QueryRow(ctx, `
		SELECT id, owner_id, capacity, open_time, close_time,
		       cancel_window_minutes, min_lead_minutes, accepting_bookings
		  FROM restaurants
		 WHERE id = $1 AND deleted_at IS NULL
		   FOR UPDATE`, id,
	).Scan(&r.ID, &r.OwnerID, &r.Capacity, &openT, &closeT,
		&r.CancelWindowMinutes, &r.MinLeadMinutes, &r.AcceptingBookings)
	if errors.Is(err, pgx.ErrNoRows) {
		return Restaurant{}, ErrNotFound
	}
	if err != nil {
		return Restaurant{}, err
	}
	r.OpenTime = time.Duration(openT.Microseconds) * time.Microsecond
	r.CloseTime = time.Duration(closeT.Microseconds) * time.Microsecond
	return r, nil
}
