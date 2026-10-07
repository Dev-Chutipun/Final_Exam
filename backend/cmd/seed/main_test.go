package main

import (
	"os"
	"path/filepath"
	"testing"
)

// pngHeader คือ 8 bytes แรกของไฟล์ PNG พอให้ http.DetectContentType ตอบ image/png
var pngHeader = []byte("\x89PNG\r\n\x1a\n")

func TestReadImagesMissingDir(t *testing.T) {
	got, err := readImages(filepath.Join(t.TempDir(), "ไม่มี"), 2)
	if err != nil || len(got) != 0 {
		t.Fatalf("โฟลเดอร์ไม่มีต้องได้ 0 รูปไม่มี error ได้ %d รูป err=%v", len(got), err)
	}
}

func TestReadImagesSkipsDotFilesAndLimits(t *testing.T) {
	dir := t.TempDir()
	for name, data := range map[string][]byte{
		".gitkeep": nil,
		"1.png":    pngHeader,
		"2.png":    pngHeader,
		"3.png":    pngHeader,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := readImages(dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ContentType != "image/png" {
		t.Fatalf("ต้องได้ 2 รูป PNG ได้ %+v", got)
	}
}

func TestReadImagesRejectsNonImage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "menu.txt"), []byte("ไม่ใช่รูป"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readImages(dir, 2); err == nil {
		t.Fatal("ไฟล์ที่ไม่ใช่รูปต้องได้ error")
	}
}

// TestSeedData ตรวจข้อมูลร้านตัวอย่างให้ผ่านกติกาเดียวกับตอนสร้างร้านผ่าน API
func TestSeedData(t *testing.T) {
	if len(restaurants) != 20 {
		t.Errorf("ต้องมี 20 ร้าน มี %d", len(restaurants))
	}
	owners := map[string]bool{}
	for _, u := range testUsers {
		owners[u.key] = true
	}
	keys := map[string]bool{}
	for _, r := range restaurants {
		if keys[r.key] {
			t.Errorf("key ซ้ำ %s", r.key)
		}
		keys[r.key] = true
		if !owners[r.owner] {
			t.Errorf("%s: เจ้าของ %s ไม่ใช่บัญชีที่ login ได้", r.key, r.owner)
		}
		if n := len([]rune(r.description())); n > 2000 {
			t.Errorf("%s: รายละเอียดยาว %d เกิน 2,000", r.key, n)
		}
		if len(r.menu) < 2 {
			t.Errorf("%s: ต้องมีเมนูอย่างน้อย 2 รายการ (ใช้กับรูปเมนู)", r.key)
		}
		if n := len(r.categories); n < 1 || n > 3 {
			t.Errorf("%s: ต้องมี 1–3 หมวด มี %d", r.key, n)
		}
		if r.minLead < r.cancelWindow || r.cancelWindow < 30 {
			t.Errorf("%s: นโยบายยกเลิก/จองล่วงหน้าไม่ผ่านกติกา", r.key)
		}
	}
}
