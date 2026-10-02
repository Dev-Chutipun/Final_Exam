package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"

	"devpool/internal/httpx"
)

const (
	sessionCookie  = "session"
	loginCookie    = "oidc_login"
	SessionMaxAge  = 7 * 24 * time.Hour // ไม่ต่ออายุอัตโนมัติ
	loginCookieAge = 600                // วินาที
)

// Handler คือ login, callback, logout, /api/me และ middleware ของ session (02 §2)
type Handler struct {
	Pool          *pgxpool.Pool
	OAuth         oauth2.Config // ใช้สร้าง URL ไป Keycloak (RedirectURL มาจาก PUBLIC_BASE_URL)
	Exchanger     Exchanger
	EndSessionURL string
	PublicBaseURL string
	CookieSecure  bool
	Now           func() time.Time
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/login", h.login)
	mux.HandleFunc("GET /api/auth/callback", h.callback)
	mux.HandleFunc("POST /api/auth/logout", h.logout)
	mux.HandleFunc("GET /api/me", h.me)
}

// loginState คือสิ่งที่เก็บใน cookie oidc_login ระหว่างไป Keycloak
type loginState struct {
	State    string `json:"state"`
	Nonce    string `json:"nonce"`
	Verifier string `json:"verifier"`
	ReturnTo string `json:"return_to"`
}

// randomToken สุ่ม 32 bytes ด้วย crypto/rand แล้วเข้ารหัส base64url
func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand ล้มเหลวไม่ได้ในทางปฏิบัติ
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// hashToken คือ SHA-256 ของ token ที่เก็บในฐานข้อมูล (ฐานข้อมูลหลุดก็เอาไปใช้เป็น session ไม่ได้)
func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// SafeReturnTo กัน open redirect: ต้องขึ้นต้นด้วย / และตัวที่สองต้องไม่ใช่ / หรือ \ ไม่อย่างนั้นใช้ /
// ("//evil.com" และ "/\evil.com" เบราว์เซอร์ตีความเป็นโดเมนอื่น)
func SafeReturnTo(s string) string {
	if !strings.HasPrefix(s, "/") || (len(s) > 1 && (s[1] == '/' || s[1] == '\\')) {
		return "/"
	}
	return s
}

func (h *Handler) setCookie(w http.ResponseWriter, name, value, path string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: path, MaxAge: maxAge,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: h.CookieSecure,
	})
}

// GET /api/auth/login?return_to=/bookings (02 §2.1)
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	st := loginState{
		State:    randomToken(),
		Nonce:    randomToken(),
		Verifier: oauth2.GenerateVerifier(),
		ReturnTo: SafeReturnTo(r.URL.Query().Get("return_to")),
	}
	raw, _ := json.Marshal(st)
	h.setCookie(w, loginCookie, base64.RawURLEncoding.EncodeToString(raw), "/api/auth", loginCookieAge)

	http.Redirect(w, r, h.OAuth.AuthCodeURL(st.State,
		oidc.Nonce(st.Nonce), oauth2.S256ChallengeOption(st.Verifier)), http.StatusFound)
}

// GET /api/auth/callback (02 §2.2) ผิดพลาด → 302 /auth/error?code=...
func (h *Handler) callback(w http.ResponseWriter, r *http.Request) {
	fail := func(code string, err error) {
		if err != nil {
			log.Printf("login callback %s: %v", code, err)
		}
		http.Redirect(w, r, "/auth/error?code="+code, http.StatusFound)
	}
	q := r.URL.Query()

	// 1: ผู้ใช้กดยกเลิกหรือ Keycloak ส่ง error มา
	if e := q.Get("error"); e != "" {
		fail("cancelled", nil)
		return
	}
	// 2: ต้องมี cookie จากตอนเริ่ม login (หมดอายุใน 10 นาที)
	c, err := r.Cookie(loginCookie)
	if err != nil {
		fail("login_expired", nil)
		return
	}
	// 3: ลบ cookie ทันทีให้ใช้ได้ครั้งเดียว แล้วเทียบ state (กัน login CSRF)
	h.setCookie(w, loginCookie, "", "/api/auth", -1)
	var st loginState
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err == nil {
		err = json.Unmarshal(raw, &st)
	}
	if err != nil || st.State == "" || q.Get("state") != st.State {
		fail("invalid_state", err)
		return
	}
	// 4–5: แลก code ด้วย secret + PKCE verifier แล้วตรวจ ID token และเทียบ nonce เอง
	claims, idToken, err := h.Exchanger.Exchange(r.Context(), q.Get("code"), st.Verifier)
	if err != nil {
		fail("verification_failed", err)
		return
	}
	if claims.Nonce != st.Nonce {
		fail("verification_failed", errors.New("nonce ไม่ตรง"))
		return
	}

	// 6–7: upsert ผู้ใช้ สร้าง session แล้วกลับไปหน้าที่มา
	now := h.Now()
	userID, err := upsertUser(r, h.Pool, claims)
	if err != nil {
		fail("verification_failed", err)
		return
	}
	token, err := createSession(r, h.Pool, userID, idToken, now)
	if err != nil {
		fail("verification_failed", err)
		return
	}
	h.setCookie(w, sessionCookie, token, "/", int(SessionMaxAge.Seconds()))
	http.Redirect(w, r, st.ReturnTo, http.StatusFound)
}

// DisplayName เลือกชื่อแสดงผลจาก claims (02 §3) ชื่อนี้แสดงต่อสาธารณะในรีวิว
// name → preferred_username → given_name → "ผู้ใช้" ค่าที่มี @ ข้ามเสมอ (กันอีเมลหลุด)
func DisplayName(c Claims) string {
	for _, v := range []string{c.Name, c.PreferredUsername, c.GivenName} {
		v = strings.TrimSpace(v)
		if v == "" || strings.Contains(v, "@") {
			continue
		}
		if utf8.RuneCountInString(v) > 60 {
			v = strings.TrimSpace(string([]rune(v)[:60]))
		}
		return v
	}
	return "ผู้ใช้"
}

// upsertUser ผูกผู้ใช้ด้วย (iss, sub) ไม่ใช้อีเมล และอัปเดตชื่อทุกครั้งที่ login
func upsertUser(r *http.Request, pool *pgxpool.Pool, c Claims) (int64, error) {
	var id int64
	err := pool.QueryRow(r.Context(), `
		INSERT INTO users (issuer, subject, display_name) VALUES ($1, $2, $3)
		ON CONFLICT (issuer, subject) DO UPDATE SET display_name = EXCLUDED.display_name
		RETURNING id`, c.Issuer, c.Subject, DisplayName(c)).Scan(&id)
	return id, err
}

// createSession ลบ session ที่หมดอายุของผู้ใช้คนนี้ (ไม่ต้องมี job ลบ) แล้วสร้างใหม่
// คืน token ดิบสำหรับ cookie ฐานข้อมูลเก็บแค่ hash
func createSession(r *http.Request, pool *pgxpool.Pool, userID int64, idToken string, now time.Time) (string, error) {
	ctx := r.Context()
	if _, err := pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1 AND expires_at <= $2`, userID, now); err != nil {
		return "", err
	}
	token := randomToken()
	_, err := pool.Exec(ctx, `
		INSERT INTO sessions (token_hash, user_id, id_token, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		hashToken(token), userID, idToken, now, now.Add(SessionMaxAge))
	return token, err
}

// Middleware อ่าน cookie session ทุกคำขอ พบ session ที่ยังไม่หมดอายุ → ใส่ userID ลง context
// ไม่พบ → ไม่ตอบ error ปล่อยให้ handler ตัดสินเองผ่าน UserID(ctx) (มี endpoint สาธารณะ)
func (h *Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil || c.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		var userID int64
		err = h.Pool.QueryRow(r.Context(),
			`SELECT user_id FROM sessions WHERE token_hash = $1 AND expires_at > $2`,
			hashToken(c.Value), h.Now()).Scan(&userID)
		switch {
		case err == nil:
			r = r.WithContext(WithUserID(r.Context(), userID))
		case !errors.Is(err, pgx.ErrNoRows):
			httpx.WriteError(w, r, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// POST /api/auth/logout (02 §2.4) เป็นฟอร์ม HTML ธรรมดา
// ลบ session แล้วพาไป logout ที่ Keycloak ด้วย ไม่อย่างนั้น Keycloak จะ login บัญชีเดิมให้อัตโนมัติ
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	h.setCookie(w, sessionCookie, "", "/", -1)
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	var idToken string
	err = h.Pool.QueryRow(r.Context(),
		`DELETE FROM sessions WHERE token_hash = $1 RETURNING id_token`, hashToken(c.Value)).Scan(&idToken)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	v := url.Values{}
	v.Set("id_token_hint", idToken)
	v.Set("post_logout_redirect_uri", h.PublicBaseURL+"/")
	http.Redirect(w, r, h.EndSessionURL+"?"+v.Encode(), http.StatusSeeOther)
}

// GET /api/me (02 §2.5)
func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserID(r.Context())
	if !ok {
		httpx.Write(w, httpx.New("unauthenticated", "ยังไม่ได้เข้าสู่ระบบ"))
		return
	}
	var out struct {
		ID          int64  `json:"id"`
		DisplayName string `json:"display_name"`
	}
	err := h.Pool.QueryRow(r.Context(), `SELECT id, display_name FROM users WHERE id = $1`, userID).
		Scan(&out.ID, &out.DisplayName)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}
