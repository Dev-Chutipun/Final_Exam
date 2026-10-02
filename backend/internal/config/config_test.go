package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "# comment\n\nDEVPOOL_T_A=hello\nDEVPOOL_T_B = with spaces \nDEVPOOL_T_URL=postgres://u:p@h/db?sslmode=disable\nDEVPOOL_T_REAL=from-file\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	// ตัวแปรที่ตั้งใน environment จริงต้องชนะไฟล์
	t.Setenv("DEVPOOL_T_REAL", "from-env")
	for _, k := range []string{"DEVPOOL_T_A", "DEVPOOL_T_B", "DEVPOOL_T_URL"} {
		t.Setenv(k, "") // ให้ t คืนค่าเดิมหลังจบ test
		os.Unsetenv(k)
	}

	if err := loadFile(path); err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"DEVPOOL_T_A":    "hello",
		"DEVPOOL_T_B":    "with spaces",
		"DEVPOOL_T_URL":  "postgres://u:p@h/db?sslmode=disable",
		"DEVPOOL_T_REAL": "from-env",
	}
	for k, v := range want {
		if got := os.Getenv(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestLoadFileBadLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("NOT_A_PAIR\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loadFile(path); err == nil {
		t.Fatal("want error for line without =")
	}
}

func TestFindUp(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(root, ".env")
	if err := os.WriteFile(envPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	got, ok := findUp(deep, ".env")
	if !ok || got != envPath {
		t.Fatalf("findUp = %q, %v; want %q", got, ok, envPath)
	}
	if _, ok := findUp(deep, ".no-such-file"); ok {
		t.Fatal("want not found")
	}
}
