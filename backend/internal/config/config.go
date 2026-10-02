// Package config อ่านไฟล์ .env แบบง่าย (KEY=VALUE) โดยไม่เพิ่ม dependency
package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Load หาไฟล์ .env จากโฟลเดอร์ปัจจุบันขึ้นไปจนเจอ แล้วตั้งค่าเป็น environment variable
// ตัวแปรที่ตั้งไว้ใน environment จริงอยู่แล้วมีสิทธิ์เหนือกว่าไฟล์ จึงไม่ถูกเขียนทับ
// ไม่เจอไฟล์ไม่ถือว่าผิด (เช่นตอนรัน test ใน container ที่ตั้ง env มาให้แล้ว)
func Load() error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	path, ok := findUp(dir, ".env")
	if !ok {
		return nil
	}
	return loadFile(path)
}

// findUp เดินจาก dir ขึ้นไปทีละชั้นจนถึง root ของไดรฟ์
func findUp(dir, name string) (string, bool) {
	for {
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func loadFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return fmt.Errorf("%s:%d: ต้องเป็นรูปแบบ KEY=VALUE", path, lineNo)
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, strings.TrimSpace(value)); err != nil {
			return err
		}
	}
	return sc.Err()
}

// Require คืนค่าของตัวแปร หรือ error ถ้าไม่ได้ตั้งไว้
func Require(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", errors.New("ไม่ได้ตั้งค่า " + key + " (ดู .env.example)")
	}
	return v, nil
}
