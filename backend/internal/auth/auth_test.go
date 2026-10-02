package auth_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"

	"devpool/internal/auth"
	"devpool/internal/httpx"
	"devpool/internal/testdb"
)

const base = "http://localhost:3000"

var testNow = time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)

// fakeExchanger แทน Keycloak: คืน claims ที่กำหนด หรือ error
type fakeExchanger struct {
	claims        auth.Claims
	err           error
	gotCode       string
	gotVerifier   string
	useLoginNonce bool // true = ใส่ nonce ให้ตรงกับที่ login ส่งไป (จำลอง Keycloak ปกติ)
	lastNonce     string
}

func (f *fakeExchanger) Exchange(_ context.Context, code, verifier string) (auth.Claims, string, error) {
	f.gotCode, f.gotVerifier = code, verifier
	if f.err != nil {
		return auth.Claims{}, "", f.err
	}
	c := f.claims
	if f.useLoginNonce {
		c.Nonce = f.lastNonce
	}
	return c, "raw-id-token-" + c.Subject, nil
}

type env struct {
	t    *testing.T
	pool *pgxpool.Pool
	ex   *fakeExchanger
	h    *auth.Handler
	srv  http.Handler
	now  time.Time
}

func newEnv(t *testing.T) *env {
	pool := testdb.New(t)
	e := &env{t: t, pool: pool, now: testNow, ex: &fakeExchanger{
		claims:        auth.Claims{Issuer: "http://localhost:8081/realms/devpool", Subject: "sub-alice", Name: "อลิซ ใจดี"},
		useLoginNonce: true,
	}}
	e.h = &auth.Handler{
		Pool: pool,
		OAuth: oauth2.Config{
			ClientID:    "devpool-web",
			Endpoint:    oauth2.Endpoint{AuthURL: "http://kc.test/auth", TokenURL: "http://kc.test/token"},
			RedirectURL: base + "/api/auth/callback",
			Scopes:      []string{"openid", "profile"},
		},
		Exchanger:     e.ex,
		EndSessionURL: "http://kc.test/logout",
		PublicBaseURL: base,
		Now:           func() time.Time { return e.now },
	}
	mux := http.NewServeMux()
	e.h.Register(mux)
	// endpoint ทดสอบที่อ่าน JSON เพื่อทดสอบ CSRF ชั้นที่ 3
	mux.HandleFunc("POST /api/test/json", func(w http.ResponseWriter, r *http.Request) {
		var v map[string]any
		if httpx.ReadJSON(w, r, &v) {
			w.WriteHeader(http.StatusNoContent)
		}
	})
	srv, err := auth.CSRF(base, e.h.Middleware(mux))
	if err != nil {
		t.Fatal(err)
	}
	e.srv = srv
	return e
}

func (e *env) request(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.srv.ServeHTTP(rec, req)
	return rec
}

func cookieOf(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// login เริ่ม login แล้วคืน cookie oidc_login และ query ของ URL ที่ไป Keycloak
func (e *env) login(returnTo string) (*http.Cookie, url.Values) {
	e.t.Helper()
	rec := e.request(httptest.NewRequest(http.MethodGet, "/api/auth/login?return_to="+url.QueryEscape(returnTo), nil))
	if rec.Code != http.StatusFound {
		e.t.Fatalf("login status = %d", rec.Code)
	}
	loc, _ := url.Parse(rec.Header().Get("Location"))
	q := loc.Query()
	e.ex.lastNonce = q.Get("nonce")
	return cookieOf(rec, "oidc_login"), q
}

// callback เรียก callback พร้อม cookie (ถ้ามี)
func (e *env) callback(query string, c *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?"+query, nil)
	if c != nil {
		req.AddCookie(c)
	}
	return e.request(req)
}

// fullLogin login สำเร็จแล้วคืน cookie session
func (e *env) fullLogin() *http.Cookie {
	e.t.Helper()
	c, q := e.login("/bookings")
	rec := e.callback("code=abc&state="+q.Get("state"), c)
	s := cookieOf(rec, "session")
	if rec.Code != http.StatusFound || s == nil {
		e.t.Fatalf("login failed: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	return s
}

func TestLoginRedirectAndCookie(t *testing.T) {
	e := newEnv(t)
	c, q := e.login("/bookings")

	if c == nil || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/api/auth" || c.MaxAge != 600 || c.Secure {
		t.Errorf("oidc_login cookie = %+v", c)
	}
	if q.Get("state") == "" || q.Get("nonce") == "" || q.Get("code_challenge") == "" ||
		q.Get("code_challenge_method") != "S256" || q.Get("response_type") != "code" ||
		q.Get("redirect_uri") != base+"/api/auth/callback" || q.Get("scope") != "openid profile" {
		t.Errorf("auth URL query = %v", q)
	}
	// state และ nonce ต้องไม่ซ้ำกันทุกครั้ง
	if _, q2 := e.login("/"); q2.Get("state") == q.Get("state") || q2.Get("nonce") == q.Get("nonce") {
		t.Error("state/nonce ซ้ำกับครั้งก่อน")
	}
}

func TestSafeReturnTo(t *testing.T) {
	for in, want := range map[string]string{
		"/bookings":          "/bookings",
		"/restaurants/1?x=1": "/restaurants/1?x=1",
		"":                   "/",
		"https://evil.com":   "/",
		"//evil.com":         "/",
		`/\evil.com`:         "/",
		"evil.com":           "/",
	} {
		if got := auth.SafeReturnTo(in); got != want {
			t.Errorf("SafeReturnTo(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCallbackFailures(t *testing.T) {
	tests := []struct {
		name  string
		setup func(e *env) *httptest.ResponseRecorder
		code  string
	}{
		{"Keycloak ส่ง error", func(e *env) *httptest.ResponseRecorder {
			c, q := e.login("/")
			return e.callback("error=access_denied&state="+q.Get("state"), c)
		}, "cancelled"},
		{"ไม่มี cookie", func(e *env) *httptest.ResponseRecorder {
			_, q := e.login("/")
			return e.callback("code=abc&state="+q.Get("state"), nil)
		}, "login_expired"},
		{"state ไม่ตรง", func(e *env) *httptest.ResponseRecorder {
			c, _ := e.login("/")
			return e.callback("code=abc&state=forged", c)
		}, "invalid_state"},
		{"แลก code ล้มเหลว", func(e *env) *httptest.ResponseRecorder {
			e.ex.err = errors.New("invalid_grant")
			c, q := e.login("/")
			return e.callback("code=abc&state="+q.Get("state"), c)
		}, "verification_failed"},
		{"nonce ไม่ตรง", func(e *env) *httptest.ResponseRecorder {
			e.ex.useLoginNonce = false
			e.ex.claims.Nonce = "replayed-nonce"
			c, q := e.login("/")
			return e.callback("code=abc&state="+q.Get("state"), c)
		}, "verification_failed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			rec := tc.setup(e)
			if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/auth/error?code="+tc.code {
				t.Errorf("got %d %s, want /auth/error?code=%s", rec.Code, rec.Header().Get("Location"), tc.code)
			}
			if cookieOf(rec, "session") != nil {
				t.Error("ไม่ควรตั้ง cookie session")
			}
			var n int
			e.pool.QueryRow(context.Background(), `SELECT count(*) FROM sessions`).Scan(&n)
			if n != 0 {
				t.Errorf("sessions = %d", n)
			}
		})
	}
}

func TestCallbackSuccess(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	c, q := e.login("/bookings")
	rec := e.callback("code=the-code&state="+q.Get("state"), c)

	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/bookings" {
		t.Fatalf("got %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if e.ex.gotCode != "the-code" || e.ex.gotVerifier == "" {
		t.Errorf("exchange got code %q verifier %q", e.ex.gotCode, e.ex.gotVerifier)
	}
	s := cookieOf(rec, "session")
	if s == nil || !s.HttpOnly || s.SameSite != http.SameSiteLaxMode || s.Path != "/" || s.MaxAge != 604800 {
		t.Fatalf("session cookie = %+v", s)
	}
	if lc := cookieOf(rec, "oidc_login"); lc == nil || lc.MaxAge >= 0 {
		t.Errorf("oidc_login ต้องถูกลบ: %+v", lc)
	}

	// ผู้ใช้ถูกสร้างด้วย (iss, sub) และ session เก็บเป็น hash ไม่ใช่ค่าจริง
	var userID int64
	var name string
	if err := e.pool.QueryRow(ctx, `SELECT id, display_name FROM users WHERE issuer = $1 AND subject = 'sub-alice'`,
		e.ex.claims.Issuer).Scan(&userID, &name); err != nil || name != "อลิซ ใจดี" {
		t.Fatalf("user = %d %q %v", userID, name, err)
	}
	sum := sha256.Sum256([]byte(s.Value))
	var idToken string
	var expires time.Time
	if err := e.pool.QueryRow(ctx, `SELECT id_token, expires_at FROM sessions WHERE token_hash = $1 AND user_id = $2`,
		sum[:], userID).Scan(&idToken, &expires); err != nil {
		t.Fatalf("session by hash: %v", err)
	}
	var raw int
	e.pool.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE token_hash = $1`, []byte(s.Value)).Scan(&raw)
	if raw != 0 || idToken != "raw-id-token-sub-alice" || !expires.Equal(testNow.Add(7*24*time.Hour)) {
		t.Errorf("raw stored %d, id_token %q, expires %v", raw, idToken, expires)
	}

	// login ซ้ำด้วยชื่อใหม่: ผู้ใช้เดิมถูกอัปเดตชื่อ ไม่สร้างใหม่
	e.ex.claims.Name = "อลิซ เปลี่ยนชื่อ"
	e.fullLogin()
	var users int
	e.pool.QueryRow(ctx, `SELECT count(*), max(display_name) FROM users`).Scan(&users, &name)
	if users != 1 || name != "อลิซ เปลี่ยนชื่อ" {
		t.Errorf("users = %d name %q", users, name)
	}
}

// session ที่หมดอายุของผู้ใช้ถูกลบตอน login ครั้งถัดไป
func TestLoginCleansExpiredSessions(t *testing.T) {
	e := newEnv(t)
	e.fullLogin()
	e.now = testNow.Add(8 * 24 * time.Hour) // session แรกหมดอายุแล้ว
	e.fullLogin()
	var n int
	e.pool.QueryRow(context.Background(), `SELECT count(*) FROM sessions`).Scan(&n)
	if n != 1 {
		t.Errorf("sessions = %d, want 1", n)
	}
}

func TestDisplayName(t *testing.T) {
	long := strings.Repeat("ก", 70)
	tests := []struct {
		name string
		c    auth.Claims
		want string
	}{
		{"name", auth.Claims{Name: " อลิซ ใจดี ", PreferredUsername: "alice"}, "อลิซ ใจดี"},
		{"fallback preferred_username", auth.Claims{PreferredUsername: "alice", GivenName: "อลิซ"}, "alice"},
		{"preferred_username เป็นอีเมล", auth.Claims{PreferredUsername: "alice@example.com", GivenName: "อลิซ"}, "อลิซ"},
		{"name เป็นอีเมล", auth.Claims{Name: "a@b.c", PreferredUsername: "alice"}, "alice"},
		{"ไม่มีอะไรเลย", auth.Claims{PreferredUsername: "x@y.z"}, "ผู้ใช้"},
		{"ยาวเกิน 60", auth.Claims{Name: long}, strings.Repeat("ก", 60)},
	}
	for _, tc := range tests {
		if got := auth.DisplayName(tc.c); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

// me เรียก /api/me ด้วย cookie ที่กำหนด
func (e *env) me(c *http.Cookie) (int, map[string]any) {
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	if c != nil {
		req.AddCookie(c)
	}
	rec := e.request(req)
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}

// Middleware: session ถูกต้อง / หมดอายุ / ไม่มีในระบบ / ค่าถูกแก้ไข · /api/me 200 / 401
func TestMiddlewareAndMe(t *testing.T) {
	e := newEnv(t)
	s := e.fullLogin()

	if status, body := e.me(s); status != 200 || body["display_name"] != "อลิซ ใจดี" || body["id"] == nil {
		t.Errorf("valid: %d %v", status, body)
	}
	if status, body := e.me(nil); status != 401 || body["type"] != "/problems/unauthenticated" {
		t.Errorf("no cookie: %d %v", status, body)
	}
	if status, _ := e.me(&http.Cookie{Name: "session", Value: "not-in-db"}); status != 401 {
		t.Errorf("unknown: %d", status)
	}
	tampered := *s
	tampered.Value = s.Value[:len(s.Value)-1] + "x"
	if status, _ := e.me(&tampered); status != 401 {
		t.Errorf("tampered: %d", status)
	}
	e.now = testNow.Add(7 * 24 * time.Hour) // หมดอายุพอดี (expires_at > now ไม่จริงแล้ว)
	if status, _ := e.me(s); status != 401 {
		t.Errorf("expired: %d", status)
	}
}

func TestLogout(t *testing.T) {
	e := newEnv(t)
	s := e.fullLogin()

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.AddCookie(s)
	rec := e.request(req)

	loc, _ := url.Parse(rec.Header().Get("Location"))
	if rec.Code != http.StatusSeeOther || loc.Host != "kc.test" || loc.Path != "/logout" ||
		loc.Query().Get("id_token_hint") != "raw-id-token-sub-alice" ||
		loc.Query().Get("post_logout_redirect_uri") != base+"/" {
		t.Errorf("got %d %s", rec.Code, loc)
	}
	if c := cookieOf(rec, "session"); c == nil || c.MaxAge >= 0 {
		t.Errorf("session cookie ต้องถูกล้าง: %+v", c)
	}
	if status, _ := e.me(s); status != 401 {
		t.Errorf("session ยังใช้ได้หลัง logout: %d", status)
	}

	// ไม่มี session → 303 /
	req = httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	if rec := e.request(req); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Errorf("no session: %d %s", rec.Code, rec.Header().Get("Location"))
	}
}

// CSRF 3 ชั้น
func TestCSRF(t *testing.T) {
	e := newEnv(t)
	post := func(headers map[string]string, ct string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/test/json", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", ct)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		return e.request(req)
	}
	tests := []struct {
		name    string
		headers map[string]string
		ct      string
		status  int
	}{
		{"Sec-Fetch-Site cross-site", map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"}, "application/json", 403},
		{"Origin ต่างเว็บ", map[string]string{"Origin": "https://evil.example"}, "application/json", 403},
		{"origin เดียวกัน (ผ่าน rewrite)", map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": base}, "application/json", 204},
		{"Origin = PUBLIC_BASE_URL ไม่มี Sec-Fetch-Site", map[string]string{"Origin": base}, "application/json", 204},
		{"ไม่มีทั้งสอง header (server ต่อ server)", nil, "application/json", 204},
		{"JSON endpoint ได้ text/plain", map[string]string{"Sec-Fetch-Site": "same-origin"}, "text/plain", 415},
	}
	for _, tc := range tests {
		rec := post(tc.headers, tc.ct)
		if rec.Code != tc.status {
			t.Errorf("%s: got %d %s, want %d", tc.name, rec.Code, rec.Body.String(), tc.status)
		}
		if tc.status == 403 && !strings.Contains(rec.Body.String(), "/problems/cross-origin-request") {
			t.Errorf("%s: body = %s", tc.name, rec.Body.String())
		}
	}
}
