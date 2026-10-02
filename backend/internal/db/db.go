// Package db รวม pgxpool, transaction helper, การล็อกแถวร้าน และตัวรัน migration
package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect เปิด pool และ ping หนึ่งครั้ง เพื่อให้ต่อฐานข้อมูลไม่ได้แล้วรู้ตั้งแต่เริ่มโปรแกรม
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("เปิด pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ต่อฐานข้อมูลไม่ได้: %w", err)
	}
	return pool, nil
}

// WithTx รัน fn ใน transaction แบบ Read Committed
// fn คืน error → ROLLBACK, คืน nil → COMMIT
//
// ระบุ Read Committed ชัดๆ แทนการพึ่งค่าเริ่มต้นของเซิร์ฟเวอร์ เพราะงานจองล็อกแถวร้านด้วย FOR UPDATE:
// ใน Read Committed แต่ละคำสั่งเห็นข้อมูลที่ commit ล่าสุด หลังรอ lock เสร็จจึงเห็นการจองที่อีกฝ่ายเพิ่ง commit
// ส่วน Repeatable Read จะจับ snapshot ไว้ก่อนรอ lock แล้วมองไม่เห็นการจองนั้น ทำให้ overbook ได้
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) (err error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			// error ของ fn สำคัญกว่า error ตอน rollback จึงคืน error เดิม
			_ = tx.Rollback(ctx)
		}
	}()

	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ErrNotFound ใช้เมื่อหาแถวไม่เจอ (รวมร้านที่ถูกลบแล้ว)
var ErrNotFound = errors.New("not found")
