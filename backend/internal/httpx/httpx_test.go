package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ตารางเดียวกับ docs/00-overview.md §5 ทุกแถว
func TestProblemTableMatchesSpec(t *testing.T) {
	spec := map[string]int{
		"validation":                        400,
		"unauthenticated":                   401,
		"own-restaurant-booking":            403,
		"own-restaurant-review":             403,
		"not-restaurant-owner":              403,
		"cross-origin-request":              403,
		"not-found":                         404,
		"image-too-large":                   413,
		"unsupported-image-type":            415,
		"unsupported-media-type":            415,
		"booking-in-past":                   422,
		"lead-time-not-met":                 422,
		"too-far-in-advance":                422,
		"outside-opening-hours":             422,
		"party-exceeds-capacity":            422,
		"cancel-deadline-passed":            422,
		"invalid-restaurant-policy":         422,
		"overlapping-own-booking":           409,
		"seat-capacity-exceeded":            409,
		"booking-not-active":                409,
		"already-cancelled":                 409,
		"already-reviewed":                  409,
		"settings-conflict-with-bookings":   409,
		"restaurant-not-accepting-bookings": 409,
		"restaurant-name-taken":             409,
		"restaurant-has-upcoming-bookings":  409,
		"image-limit-reached":               409,
		"last-image":                        409,
	}
	if len(problems) != len(spec) {
		t.Errorf("problems มี %d type แต่สเปกมี %d", len(problems), len(spec))
	}
	for typ, status := range spec {
		def, ok := problems[typ]
		if !ok {
			t.Errorf("ไม่มี type %q", typ)
			continue
		}
		if def.status != status {
			t.Errorf("%s: status %d, want %d", typ, def.status, status)
		}
		if def.title == "" {
			t.Errorf("%s: title ว่าง", typ)
		}
	}
}

func TestNewUnknownTypePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("want panic")
		}
	}()
	New("no-such-type", "")
}

func TestWriteProblem(t *testing.T) {
	rec := httptest.NewRecorder()
	Write(rec, New("seat-capacity-exceeded", "ช่วง 12:30–13:00 เหลือ 3 ที่ แต่ขอ 5 ที่").
		With("available", 3).
		With("full_from", "2026-10-11T12:30:00+07:00"))

	if rec.Code != 409 {
		t.Errorf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"type":      "/problems/seat-capacity-exceeded",
		"title":     "ที่นั่งไม่พอ",
		"status":    float64(409),
		"detail":    "ช่วง 12:30–13:00 เหลือ 3 ที่ แต่ขอ 5 ที่",
		"available": float64(3),
		"full_from": "2026-10-11T12:30:00+07:00",
	}
	for k, v := range want {
		if body[k] != v {
			t.Errorf("%s = %v, want %v", k, body[k], v)
		}
	}
	if _, ok := body["errors"]; ok {
		t.Error("errors ต้องมีเฉพาะ validation")
	}
}

func TestValidationAlwaysHasErrorsArray(t *testing.T) {
	b, err := json.Marshal(Validation("x"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"errors":[]`) {
		t.Errorf("want errors: [], got %s", b)
	}
}

type sample struct {
	PartySize int    `json:"party_size"`
	StartAt   string `json:"start_at"`
}

func TestReadJSON(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		ok        bool
		wantField string // ว่าง = errors ต้องว่าง
	}{
		{"ok", `{"party_size": 4, "start_at": "2026-10-11T12:00:00+07:00"}`, true, ""},
		{"float เป็น int", `{"party_size": 4.5}`, false, "party_size"},
		{"string เป็น int", `{"party_size": "4"}`, false, "party_size"},
		{"ฟิลด์ไม่รู้จัก", `{"party_size": 4, "owner_id": 9}`, false, "owner_id"},
		{"JSON เสีย", `{"party_size": `, false, ""},
		{"syntax ผิด", `{party_size: 4}`, false, ""},
		{"body ว่าง", ``, false, ""},
		{"มีข้อมูลต่อท้าย", `{"party_size": 4} {"party_size": 5}`, false, ""},
		{"เกิน 1 MB", `{"start_at": "` + strings.Repeat("a", MaxJSONBody) + `"}`, false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/x", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			var dst sample

			got := ReadJSON(rec, req, &dst)
			if got != tc.ok {
				t.Fatalf("ReadJSON = %v, want %v (body %s)", got, tc.ok, rec.Body.String())
			}
			if tc.ok {
				if dst.PartySize != 4 {
					t.Errorf("party_size = %d", dst.PartySize)
				}
				return
			}

			if rec.Code != 400 {
				t.Errorf("status = %d, want 400", rec.Code)
			}
			var p struct {
				Type   string       `json:"type"`
				Detail string       `json:"detail"`
				Errors []FieldError `json:"errors"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
				t.Fatal(err)
			}
			if p.Type != "/problems/validation" {
				t.Errorf("type = %q", p.Type)
			}
			if p.Detail == "" {
				t.Error("detail ว่าง")
			}
			if tc.wantField == "" {
				if len(p.Errors) != 0 {
					t.Errorf("errors = %+v, want empty", p.Errors)
				}
			} else if len(p.Errors) != 1 || p.Errors[0].Field != tc.wantField {
				t.Errorf("errors = %+v, want field %q", p.Errors, tc.wantField)
			}
		})
	}
}
