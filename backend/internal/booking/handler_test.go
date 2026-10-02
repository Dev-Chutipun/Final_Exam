package booking_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"devpool/internal/auth"
	"devpool/internal/booking"
)

// fakeAuth อ่าน userID จาก header X-Test-User แล้วใส่ใน context
// มีเฉพาะในไฟล์ _test.go: production code ไม่มีช่องทางสลับผู้ใช้แบบนี้
func fakeAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v := r.Header.Get("X-Test-User"); v != "" {
			id, _ := strconv.ParseInt(v, 10, 64)
			r = r.WithContext(auth.WithUserID(r.Context(), id))
		}
		next.ServeHTTP(w, r)
	})
}

func (e *env) server() http.Handler {
	mux := http.NewServeMux()
	h := &booking.Handler{Svc: e.svc, Now: func() time.Time { return testNow }}
	h.Register(mux)
	return fakeAuth(mux)
}

// do ส่งคำขอแล้วคืน status และ type ของ problem ("" ถ้าไม่ใช่ problem)
func (e *env) do(method, path string, user int64, body string) (int, string, map[string]any) {
	e.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if user != 0 {
		req.Header.Set("X-Test-User", strconv.FormatInt(user, 10))
	}
	rec := httptest.NewRecorder()
	e.server().ServeHTTP(rec, req)

	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	typ := ""
	if rec.Header().Get("Content-Type") == "application/problem+json" {
		typ, _ = out["type"].(string)
	}
	return rec.Code, typ, out
}

func TestHandlerCreate(t *testing.T) {
	e := newEnv(t)
	me := e.user("me")
	r := e.restaurant(e.user("owner"))
	path := "/api/restaurants/" + strconv.FormatInt(r, 10) + "/bookings"
	ok := `{"party_size": 4, "start_at": "2026-10-11T12:00:00+07:00", "end_at": "2026-10-11T13:00:00+07:00"}`

	tests := []struct {
		name       string
		path       string
		user       int64
		body       string
		wantStatus int
		wantType   string
	}{
		{"ไม่ได้ login", path, 0, ok, 401, "/problems/unauthenticated"},
		{"id ไม่ใช่ตัวเลข", "/api/restaurants/abc/bookings", me, ok, 404, "/problems/not-found"},
		{"party_size ทศนิยม", path, me, `{"party_size": 4.5, "start_at": "2026-10-11T12:00:00+07:00", "end_at": "2026-10-11T13:00:00+07:00"}`, 400, "/problems/validation"},
		{"เวลาไม่มี offset", path, me, `{"party_size": 4, "start_at": "2026-10-11T12:00:00", "end_at": "2026-10-11T13:00:00+07:00"}`, 400, "/problems/validation"},
		{"ฟิลด์ไม่รู้จัก", path, me, `{"party_size": 4, "user_id": 1, "start_at": "2026-10-11T12:00:00+07:00", "end_at": "2026-10-11T13:00:00+07:00"}`, 400, "/problems/validation"},
		{"สำเร็จ", path, me, ok, 201, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, typ, body := e.do(http.MethodPost, tc.path, tc.user, tc.body)
			if status != tc.wantStatus || typ != tc.wantType {
				t.Fatalf("got %d %q %v, want %d %q", status, typ, body, tc.wantStatus, tc.wantType)
			}
			if status == 201 {
				if body["start_at"] != "2026-10-11T12:00:00+07:00" || body["cancel_deadline"] != "2026-10-11T11:30:00+07:00" {
					t.Errorf("body = %v", body)
				}
			}
		})
	}
}

// เวลาที่ส่งมาเป็น UTC ก็ใช้ได้ และตอบกลับเป็น +07:00 เสมอ
func TestHandlerCreateUTCInput(t *testing.T) {
	e := newEnv(t)
	r := e.restaurant(e.user("owner"))
	status, _, body := e.do(http.MethodPost, "/api/restaurants/"+strconv.FormatInt(r, 10)+"/bookings", e.user("me"),
		`{"party_size": 2, "start_at": "2026-10-11T05:00:00Z", "end_at": "2026-10-11T06:00:00Z"}`)
	if status != 201 || body["start_at"] != "2026-10-11T12:00:00+07:00" {
		t.Fatalf("got %d %v", status, body)
	}
}
