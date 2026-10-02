package db

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrate รันไฟล์ .sql ใน fsys ตามลำดับชื่อไฟล์ ไฟล์ละหนึ่ง transaction
// ไฟล์ที่รันแล้วบันทึกใน schema_migrations จึงรันซ้ำได้โดยไม่เกิดผลซ้ำ
// คืนรายชื่อไฟล์ที่เพิ่งรันในครั้งนี้
func Migrate(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) ([]string, error) {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
		  filename   TEXT PRIMARY KEY,
		  applied_at TIMESTAMPTZ NOT NULL
		)`)
	if err != nil {
		return nil, fmt.Errorf("สร้าง schema_migrations: %w", err)
	}

	files, err := fs.Glob(fsys, "*.sql") // fs.Glob คืนชื่อเรียงตามตัวอักษรแล้ว
	if err != nil {
		return nil, err
	}

	var applied []string
	for _, name := range files {
		ran, err := migrateFile(ctx, pool, fsys, name)
		if err != nil {
			return applied, fmt.Errorf("%s: %w", name, err)
		}
		if ran {
			applied = append(applied, name)
		}
	}
	return applied, nil
}

func migrateFile(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS, name string) (ran bool, err error) {
	sql, err := fs.ReadFile(fsys, name)
	if err != nil {
		return false, err
	}

	err = WithTx(ctx, pool, func(tx pgx.Tx) error {
		// INSERT ก่อนแล้วดูว่าได้แถวไหม: ถ้าเคยรันแล้วจะชน PRIMARY KEY และไม่ได้แถวกลับมา
		// ใช้ unique constraint ตัดสิน ไม่เช็คก่อนแล้วค่อย INSERT
		tag, err := tx.Exec(ctx, `
			INSERT INTO schema_migrations (filename, applied_at) VALUES ($1, now())
			ON CONFLICT (filename) DO NOTHING`, name)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		// Exec ที่ไม่มีพารามิเตอร์ใช้ simple protocol จึงรันหลายคำสั่งในไฟล์เดียวได้
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			return err
		}
		ran = true
		return nil
	})
	return ran, err
}
