// คำสั่ง migrate รัน migrations/*.sql ตามลำดับชื่อไฟล์บน DATABASE_URL
//
//	cd backend
//	go run ./cmd/migrate
package main

import (
	"context"
	"fmt"
	"os"

	"devpool/internal/config"
	"devpool/internal/db"
	"devpool/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run() error {
	if err := config.Load(); err != nil {
		return err
	}
	url, err := config.Require("DATABASE_URL")
	if err != nil {
		return err
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		return err
	}
	defer pool.Close()

	applied, err := db.Migrate(ctx, pool, migrations.FS)
	for _, name := range applied {
		fmt.Println("applied", name)
	}
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		fmt.Println("ฐานข้อมูลเป็นปัจจุบันแล้ว")
	}
	return nil
}
