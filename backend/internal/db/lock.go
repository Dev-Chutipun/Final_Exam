package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Restaurant คือค่าของร้านที่ใช้ตัดสินใจในงานจองและงานจัดการร้าน
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

// Querier คือสิ่งที่รัน QueryRow ได้ ทั้ง *pgxpool.Pool และ pgx.Tx
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

const restaurantColumns = `
	SELECT id, owner_id, capacity, open_time, close_time,
	       cancel_window_minutes, min_lead_minutes, accepting_bookings
	  FROM restaurants
	 WHERE id = $1 AND deleted_at IS NULL`

// LockRestaurant ล็อกแถวร้านด้วย FOR UPDATE แล้วคืนค่าของร้าน ณ หลังล็อก
//
//   - กรอง deleted_at IS NULL ร้านที่ถูกลบจึงคืน ErrNotFound เหมือนร้านที่ไม่มีอยู่
//   - ทุก pipeline ที่อ่านค่าร้านเพื่อตัดสินใจต้องเรียกฟังก์ชันนี้ก่อนอ่านข้อมูลอื่น
//   - ลำดับการล็อกคือ ร้าน → การจอง เสมอ เพื่อกัน deadlock
//
// แถวร้านเป็นตัวแทนของ "ที่นั่งทั้งร้าน": การจองใหม่ยังไม่มีแถวให้ล็อก (phantom)
// ถ้าทุกคำขอจองต้องล็อกแถวร้านก่อน คำขอของร้านเดียวกันจะเข้าแถวทีละคำขอ
func LockRestaurant(ctx context.Context, tx pgx.Tx, id int64) (Restaurant, error) {
	return scanRestaurant(tx.QueryRow(ctx, restaurantColumns+` FOR UPDATE`, id))
}

// GetRestaurant อ่านค่าร้านโดยไม่ล็อก ใช้กับงานที่ผลเป็นแค่คำแนะนำ (เช่น availability)
// หรืองานที่ไม่ได้ตัดสินจากค่าร้าน ห้ามใช้แทน LockRestaurant ในงานจอง
func GetRestaurant(ctx context.Context, q Querier, id int64) (Restaurant, error) {
	return scanRestaurant(q.QueryRow(ctx, restaurantColumns, id))
}

func scanRestaurant(row pgx.Row) (Restaurant, error) {
	var r Restaurant
	var openT, closeT pgtype.Time
	err := row.Scan(&r.ID, &r.OwnerID, &r.Capacity, &openT, &closeT,
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
