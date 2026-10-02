// Package migrations ฝังไฟล์ .sql ไว้ในโปรแกรม
// ทั้ง cmd/migrate และ internal/testdb ใช้ชุดเดียวกันโดยไม่ต้องหา path บนดิสก์
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
