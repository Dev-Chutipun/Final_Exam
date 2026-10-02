// Package testdb เตรียมฐานข้อมูลสำหรับ integration test
//
// ใช้ฐาน TEST_DATABASE_URL (ไม่ได้ตั้ง → t.Skip)
// รัน migration ครั้งเดียวต่อ package และล้างข้อมูลก่อนทุก test
// ต้องรัน go test -p 1 เพื่อไม่ให้หลาย package ล้างฐานข้อมูลชนกัน
package testdb

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"devpool/internal/config"
	"devpool/internal/db"
	"devpool/migrations"
)

var (
	once    sync.Once
	pool    *pgxpool.Pool
	initErr error
)

// New คืน pool ที่ชี้ฐานข้อมูล test ซึ่งถูกล้างข้อมูลแล้ว
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()

	// อ่าน .env เพื่อให้ go test บนเครื่องใช้ค่าเดียวกับ .env ได้ (env จริงยังชนะไฟล์)
	if err := config.Load(); err != nil {
		t.Fatalf("อ่าน .env: %v", err)
	}
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ไม่ได้ตั้ง TEST_DATABASE_URL จึงข้าม integration test")
	}

	once.Do(func() {
		ctx := context.Background()
		pool, initErr = db.Connect(ctx, url)
		if initErr != nil {
			return
		}
		_, initErr = db.Migrate(ctx, pool, migrations.FS)
	})
	if initErr != nil {
		t.Fatalf("เตรียมฐานข้อมูล test: %v", initErr)
	}

	if err := truncateAll(context.Background(), pool); err != nil {
		t.Fatalf("ล้างฐานข้อมูล test: %v", err)
	}
	return pool
}

// truncateAll ล้างทุกตารางยกเว้น categories (ข้อมูลอ้างอิง) และ schema_migrations
// RESTART IDENTITY ทำให้ id เริ่มที่ 1 ใหม่ทุก test
func truncateAll(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, `
		SELECT quote_ident(tablename) FROM pg_tables
		 WHERE schemaname = 'public'
		   AND tablename NOT IN ('categories', 'schema_migrations')
		 ORDER BY tablename`)
	if err != nil {
		return err
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(tables) == 0 {
		return nil
	}
	_, err = pool.Exec(ctx, "TRUNCATE "+strings.Join(tables, ", ")+" RESTART IDENTITY CASCADE")
	return err
}
