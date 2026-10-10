package db_test

import (
	"context"
	"errors"
	"io/fs"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"devpool/internal/db"
	"devpool/internal/testdb"
	"devpool/migrations"
)

func TestMigrateIsIdempotent(t *testing.T) {
	pool := testdb.New(t) // รัน migration ไปแล้วหนึ่งรอบ
	ctx := context.Background()

	applied, err := db.Migrate(ctx, pool, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != 0 {
		t.Errorf("รอบสองไม่ควรรันไฟล์ใด แต่รัน %v", applied)
	}

	// นับไฟล์จาก embed.FS ไม่ใส่ตัวเลขตายตัว test จะได้ไม่พังทุกครั้งที่เพิ่ม migration ใหม่
	want, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	var files, categories int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&files); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM categories`).Scan(&categories); err != nil {
		t.Fatal(err)
	}
	if files != len(want) || categories != 12 {
		t.Errorf("schema_migrations = %d (want %d), categories = %d (want 12)", files, len(want), categories)
	}
}

func TestWithTxCommitAndRollback(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()

	err := db.WithTx(ctx, pool, func(tx pgx.Tx) error {
		var level string
		if err := tx.QueryRow(ctx, `SHOW transaction_isolation`).Scan(&level); err != nil {
			return err
		}
		if level != "read committed" {
			t.Errorf("isolation = %q, want read committed", level)
		}
		_, err := tx.Exec(ctx, `INSERT INTO users (issuer, subject, display_name) VALUES ('t', 'commit', 'A')`)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	boom := errors.New("boom")
	err = db.WithTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO users (issuer, subject, display_name) VALUES ('t', 'rollback', 'B')`); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}

	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("users = %d, want 1 (แถวที่ rollback ต้องไม่อยู่)", n)
	}
}

func TestLockRestaurant(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	owner := insertUser(t, pool, "owner")
	open := insertRestaurant(t, pool, owner, "ร้านเปิด", "18:00", "02:00")
	deleted := insertRestaurant(t, pool, owner, "ร้านที่ลบ", "10:00", "22:00")
	if _, err := pool.Exec(ctx, `UPDATE restaurants SET deleted_at = now() WHERE id = $1`, deleted); err != nil {
		t.Fatal(err)
	}

	err := db.WithTx(ctx, pool, func(tx pgx.Tx) error {
		r, err := db.LockRestaurant(ctx, tx, open)
		if err != nil {
			return err
		}
		want := db.Restaurant{
			ID: open, OwnerID: owner, Capacity: 10,
			OpenTime: 18 * time.Hour, CloseTime: 2 * time.Hour,
			CancelWindowMinutes: 30, MinLeadMinutes: 120, AcceptingBookings: true,
		}
		if r != want {
			t.Errorf("got %+v\nwant %+v", r, want)
		}

		for _, id := range []int64{deleted, 999999} {
			if _, err := db.LockRestaurant(ctx, tx, id); !errors.Is(err, db.ErrNotFound) {
				t.Errorf("id %d: err = %v, want ErrNotFound", id, err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// ขณะที่ transaction แรกถือ lock อยู่ อีก transaction ต้องล็อกร้านเดียวกันไม่ได้
func TestLockRestaurantBlocksOthers(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	id := insertRestaurant(t, pool, insertUser(t, pool, "owner"), "ร้าน", "10:00", "22:00")

	err := db.WithTx(ctx, pool, func(tx1 pgx.Tx) error {
		if _, err := db.LockRestaurant(ctx, tx1, id); err != nil {
			return err
		}
		err := db.WithTx(ctx, pool, func(tx2 pgx.Tx) error {
			if _, err := tx2.Exec(ctx, `SET LOCAL lock_timeout = '200ms'`); err != nil {
				return err
			}
			_, err := db.LockRestaurant(ctx, tx2, id)
			return err
		})
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "55P03" { // lock_not_available
			t.Errorf("tx2 err = %v, want lock timeout (55P03)", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// หัวใจของ Read Committed + ล็อกแถวร้าน:
// tx2 รอ lock อยู่ระหว่างที่ tx1 เพิ่มการจอง พอ tx1 commit แล้ว tx2 ต้องเห็นการจองนั้น
func TestReadAfterLockSeesCommittedBooking(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	owner := insertUser(t, pool, "owner")
	customer := insertUser(t, pool, "customer")
	id := insertRestaurant(t, pool, owner, "ร้าน", "10:00", "22:00")

	tx1, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	defer tx1.Rollback(ctx)
	if _, err := db.LockRestaurant(ctx, tx1, id); err != nil {
		t.Fatal(err)
	}
	if _, err := tx1.Exec(ctx, `
		INSERT INTO bookings (restaurant_id, user_id, party_size, start_at, end_at, cancel_deadline)
		VALUES ($1, $2, 4, '2026-10-11 12:00+07', '2026-10-11 13:00+07', '2026-10-11 11:30+07')`,
		id, customer); err != nil {
		t.Fatal(err)
	}

	seen := make(chan int, 1)
	errs := make(chan error, 1)
	go func() {
		errs <- db.WithTx(ctx, pool, func(tx2 pgx.Tx) error {
			if _, err := db.LockRestaurant(ctx, tx2, id); err != nil { // รอ tx1 ตรงนี้
				return err
			}
			var n int
			err := tx2.QueryRow(ctx, `SELECT count(*) FROM bookings WHERE restaurant_id = $1`, id).Scan(&n)
			seen <- n
			return err
		})
	}()

	select {
	case <-seen:
		t.Fatal("tx2 ล็อกร้านได้ทั้งที่ tx1 ยังถือ lock")
	case <-time.After(300 * time.Millisecond):
	}
	if err := tx1.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	if n := <-seen; n != 1 {
		t.Errorf("tx2 เห็นการจอง %d แถว, want 1", n)
	}
}

func insertUser(t *testing.T, pool *pgxpool.Pool, subject string) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (issuer, subject, display_name) VALUES ('test://local', $1, $1) RETURNING id`,
		subject).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func insertRestaurant(t *testing.T, pool *pgxpool.Pool, owner int64, name, open, close string) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(), `
		INSERT INTO restaurants (owner_id, name, description, capacity, open_time, close_time)
		VALUES ($1, $2, 'test', 10, $3, $4) RETURNING id`,
		owner, name, open, close).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
