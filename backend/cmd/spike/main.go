// คำสั่ง spike ใช้ทดสอบ Keycloak + rewrite ของ Next.js ตาม docs/02-auth.md §8 เท่านั้น
// ไม่ใช่โค้ดจริง: session เก็บในหน่วยความจำ ไม่มี test ลบทิ้งหลังรายงานผล spike
//
//	cd backend
//	go run ./cmd/spike
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"devpool/internal/config"
	"devpool/internal/httpx"
)

type loginState struct {
	State    string `json:"state"`
	Nonce    string `json:"nonce"`
	Verifier string `json:"verifier"`
	ReturnTo string `json:"return_to"`
}

type session struct {
	IDToken           string
	Issuer, Subject   string
	Name, PreferredUN string
}

type server struct {
	baseURL    string
	issuer     string
	oauth      oauth2.Config
	verifier   *oidc.IDTokenVerifier
	endSession string

	mu       sync.Mutex
	sessions map[string]session
}

func main() {
	if err := config.Load(); err != nil {
		log.Fatal(err)
	}
	env := func(k string) string {
		v, err := config.Require(k)
		if err != nil {
			log.Fatal(err)
		}
		return v
	}
	s := &server{
		baseURL:  env("PUBLIC_BASE_URL"),
		issuer:   env("OIDC_ISSUER"),
		sessions: map[string]session{},
	}

	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, s.issuer)
	if err != nil {
		log.Fatalf("ต่อ Keycloak ไม่ได้ (%s): %v", s.issuer, err)
	}
	var meta struct {
		EndSession string `json:"end_session_endpoint"`
	}
	if err := provider.Claims(&meta); err != nil {
		log.Fatal(err)
	}
	s.endSession = meta.EndSession
	s.oauth = oauth2.Config{
		ClientID:     env("OIDC_CLIENT_ID"),
		ClientSecret: env("OIDC_CLIENT_SECRET"),
		Endpoint:     provider.Endpoint(),
		RedirectURL:  s.baseURL + "/api/auth/callback",
		Scopes:       []string{oidc.ScopeOpenID, "profile"},
	}
	s.verifier = provider.Verifier(&oidc.Config{ClientID: s.oauth.ClientID})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/auth/login", s.login)
	mux.HandleFunc("GET /api/auth/callback", s.callback)
	mux.HandleFunc("POST /api/auth/logout", s.logout)
	mux.HandleFunc("GET /api/spike/me", s.me)
	mux.HandleFunc("POST /api/spike/echo", echo)
	mux.HandleFunc("POST /api/spike/upload", upload)

	cop := http.NewCrossOriginProtection()
	if err := cop.AddTrustedOrigin(s.baseURL); err != nil {
		log.Fatal(err)
	}
	cop.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("CSRF ปฏิเสธ %s %s Sec-Fetch-Site=%q Origin=%q Host=%q",
			r.Method, r.URL.Path, r.Header.Get("Sec-Fetch-Site"), r.Header.Get("Origin"), r.Host)
		httpx.Write(w, httpx.New("cross-origin-request", ""))
	}))

	addr := ":" + env("PORT")
	log.Printf("spike ฟังที่ %s · issuer %s · end_session %s", addr, s.issuer, s.endSession)
	log.Fatal(http.ListenAndServe(addr, logRequests(cop.Handler(mux))))
}

func logRequests(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s (Host=%s)", r.Method, r.URL.Path, r.Host)
		h.ServeHTTP(w, r)
	})
}

func randomString() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	st := loginState{
		State:    randomString(),
		Nonce:    randomString(),
		Verifier: oauth2.GenerateVerifier(),
		ReturnTo: "/",
	}
	raw, _ := json.Marshal(st)
	http.SetCookie(w, &http.Cookie{
		Name: "oidc_login", Value: base64.RawURLEncoding.EncodeToString(raw),
		Path: "/api/auth", MaxAge: 600, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, s.oauth.AuthCodeURL(st.State,
		oidc.Nonce(st.Nonce), oauth2.S256ChallengeOption(st.Verifier)), http.StatusFound)
}

func (s *server) callback(w http.ResponseWriter, r *http.Request) {
	fail := func(code string, err error) {
		log.Printf("callback ล้มเหลว %s: %v", code, err)
		http.Error(w, "spike callback failed: "+code, http.StatusBadRequest)
	}
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		fail("cancelled", fmt.Errorf("%s", e))
		return
	}
	c, err := r.Cookie("oidc_login")
	if err != nil {
		fail("login_expired", err)
		return
	}
	var st loginState
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err == nil {
		err = json.Unmarshal(raw, &st)
	}
	if err != nil || q.Get("state") != st.State {
		fail("invalid_state", err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "oidc_login", Path: "/api/auth", MaxAge: -1})

	tok, err := s.oauth.Exchange(r.Context(), q.Get("code"), oauth2.VerifierOption(st.Verifier))
	if err != nil {
		fail("verification_failed", err)
		return
	}
	rawID, _ := tok.Extra("id_token").(string)
	idt, err := s.verifier.Verify(r.Context(), rawID)
	if err != nil {
		fail("verification_failed", err)
		return
	}
	if idt.Nonce != st.Nonce {
		fail("verification_failed", fmt.Errorf("nonce mismatch"))
		return
	}
	var claims struct {
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
	}
	if err := idt.Claims(&claims); err != nil {
		fail("verification_failed", err)
		return
	}

	token := randomString()
	s.mu.Lock()
	s.sessions[token] = session{rawID, idt.Issuer, idt.Subject, claims.Name, claims.PreferredUsername}
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name: "session", Value: token, Path: "/", MaxAge: 604800,
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	log.Printf("login สำเร็จ iss=%s sub=%s name=%s", idt.Issuer, idt.Subject, claims.Name)
	http.Redirect(w, r, st.ReturnTo, http.StatusFound)
}

func (s *server) current(r *http.Request) (string, session, bool) {
	c, err := r.Cookie("session")
	if err != nil {
		return "", session{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[c.Value]
	return c.Value, sess, ok
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	token, sess, ok := s.current(r)
	http.SetCookie(w, &http.Cookie{Name: "session", Path: "/", MaxAge: -1})
	if !ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()

	v := url.Values{}
	v.Set("id_token_hint", sess.IDToken)
	v.Set("post_logout_redirect_uri", s.baseURL+"/")
	http.Redirect(w, r, s.endSession+"?"+v.Encode(), http.StatusSeeOther)
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	_, sess, ok := s.current(r)
	if !ok {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"logged_in": false})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"logged_in":          true,
		"iss":                sess.Issuer,
		"expected_issuer":    s.issuer,
		"iss_matches":        sess.Issuer == s.issuer,
		"sub":                sess.Subject,
		"name":               sess.Name,
		"preferred_username": sess.PreferredUN,
	})
}

// echo ให้เห็นว่า Go ได้ header อะไรมาหลังผ่าน rewrite และผ่าน CrossOriginProtection แล้ว
func echo(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"sec_fetch_site": r.Header.Get("Sec-Fetch-Site"),
		"origin":         r.Header.Get("Origin"),
		"host":           r.Host,
	})
}

// upload รับ multipart ไฟล์ชื่อ images แล้วคืนขนาดและ SHA-256 ของแต่ละไฟล์ (เพดาน 7 MB ตาม 03 §4.1)
func upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 7<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		http.Error(w, "parse multipart: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	type fileInfo struct {
		Name   string `json:"name"`
		Size   int64  `json:"size"`
		SHA256 string `json:"sha256"`
	}
	var files []fileInfo
	for _, fh := range r.MultipartForm.File["images"] {
		f, err := fh.Open()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h := sha256.New()
		n, err := io.Copy(h, f)
		f.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		files = append(files, fileInfo{fh.Filename, n, hex.EncodeToString(h.Sum(nil))})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"count": len(files), "files": files})
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ltime)
}
