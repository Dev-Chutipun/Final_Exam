package review_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"devpool/internal/auth"
	"devpool/internal/review"
	"devpool/internal/testdb"
)

var testNow = time.Date(2026, 10, 10, 8, 0, 0, 0, time.FixedZone("ICT", 7*3600))

type env struct {
	t           *testing.T
	pool        *pgxpool.Pool
	svc         *review.Service
	restaurants int
	clock       int // เลื่อนเวลา created_at ทีละนาทีเพื่อให้ลำดับรีวิวชัดเจน
}

func newEnv(t *testing.T) *env {
	pool := testdb.New(t)
	return &env{t: t, pool: pool, svc: &review.Service{Pool: pool}}
}

func (e *env) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.pool.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) user(name string) int64 {
	e.t.Helper()
	var id int64
	if err := e.pool.QueryRow(context.Background(),
		`INSERT INTO users (issuer, subject, display_name) VALUES ('test://local', $1, $1) RETURNING id`,
		name).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *env) restaurant(owner int64) int64 {
	e.t.Helper()
	e.restaurants++
	var id int64
	if err := e.pool.QueryRow(context.Background(), `
		INSERT INTO restaurants (owner_id, name, description, capacity, open_time, close_time)
		VALUES ($1, $2, 'test', 10, '10:00', '22:00') RETURNING id`,
		owner, fmt.Sprintf("ร้านทดสอบ %d", e.restaurants)).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	return id
}

// rate ใส่รีวิวลงฐานข้อมูลตรงๆ
func (e *env) rate(restaurantID, userID int64, rating int) {
	e.t.Helper()
	e.clock++
	ts := testNow.Add(time.Duration(e.clock) * time.Minute)
	// ให้ทุกด้านเท่ากัน คะแนนรวมที่ฐานข้อมูลคำนวณจึงเท่ากับ rating พอดี
	e.exec(`INSERT INTO reviews (restaurant_id, user_id, rating_food, rating_ambience, rating_service, rating_value,
	                             comment, created_at, updated_at)
	        VALUES ($1, $2, $3, $3, $3, $3, 'ok', $4, $4)`, restaurantID, userID, rating, ts)
}

// manyRatings ใส่รีวิวจากผู้ใช้ใหม่ n คน คะแนน rating
func (e *env) manyRatings(restaurantID int64, n, rating int, prefix string) {
	e.t.Helper()
	e.exec(`
		WITH u AS (
		  INSERT INTO users (issuer, subject, display_name)
		  SELECT 'test://local', $3 || g, $3 || g FROM generate_series(1, $2::int) g
		  RETURNING id)
		INSERT INTO reviews (restaurant_id, user_id, rating_food, rating_ambience, rating_service, rating_value,
		                     comment, created_at, updated_at)
		SELECT $1, id, $4, $4, $4, $4, 'ok', $5, $5 FROM u`,
		restaurantID, n, prefix, rating, testNow)
}

func fakeAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v := r.Header.Get("X-Test-User"); v != "" {
			id, _ := strconv.ParseInt(v, 10, 64)
			r = r.WithContext(auth.WithUserID(r.Context(), id))
		}
		next.ServeHTTP(w, r)
	})
}

func (e *env) do(method, path string, user int64, body string) (int, string, map[string]any) {
	e.t.Helper()
	mux := http.NewServeMux()
	(&review.Handler{Svc: e.svc, Now: func() time.Time { return testNow }}).Register(mux)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if user != 0 {
		req.Header.Set("X-Test-User", strconv.FormatInt(user, 10))
	}
	rec := httptest.NewRecorder()
	fakeAuth(mux).ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	typ := ""
	if rec.Header().Get("Content-Type") == "application/problem+json" {
		typ, _ = out["type"].(string)
	}
	return rec.Code, typ, out
}

func itoa(id int64) string { return strconv.FormatInt(id, 10) }

// reviewBody ให้ทุกด้านเท่ากัน คะแนนรวมที่ฐานข้อมูลคำนวณจึงเท่ากับค่าที่ส่งมาพอดี
func reviewBody(rating any, comment string) string {
	return fmt.Sprintf(`{"aspects": {"food": %[1]v, "ambience": %[1]v, "service": %[1]v, "value": %[1]v}, "comment": %[2]q}`, rating, comment)
}

// list คืน id ของร้านตามลำดับจาก GET /api/restaurants
func (e *env) list(sort string) ([]int64, []map[string]any) {
	e.t.Helper()
	status, _, body := e.do(http.MethodGet, "/api/restaurants?sort="+sort, 0, "")
	if status != 200 {
		e.t.Fatalf("list: %d %v", status, body)
	}
	var ids []int64
	var items []map[string]any
	for _, it := range body["items"].([]any) {
		m := it.(map[string]any)
		ids = append(ids, int64(m["id"].(float64)))
		items = append(items, m)
	}
	return ids, items
}

// V1–V2: รูปแบบผิด → 400
func TestReviewV1V2Validation(t *testing.T) {
	e := newEnv(t)
	me := e.user("me")
	r := e.restaurant(e.user("owner"))
	path := "/api/restaurants/" + itoa(r) + "/reviews"
	for _, body := range []string{
		reviewBody(0, "ดี"), reviewBody(6, "ดี"), reviewBody(4.5, "ดี"), `{"comment": "ดี"}`,
		reviewBody(5, ""), reviewBody(5, "   \n\t "), reviewBody(5, strings.Repeat("ก", 1001)),
	} {
		if status, typ, _ := e.do(http.MethodPost, path, me, body); status != 400 || typ != "/problems/validation" {
			t.Errorf("%.40s: got %d %s", body, status, typ)
		}
	}
	// 1,000 ตัวอักษรไทยพอดีผ่าน (นับเป็น rune ไม่ใช่ byte) และ comment ถูกตัดช่องว่างหัวท้าย
	status, _, body := e.do(http.MethodPost, path, me, reviewBody(5, "  "+strings.Repeat("ก", 1000)+"  "))
	if status != 201 || body["comment"] != strings.Repeat("ก", 1000) || body["display_name"] != "me" {
		t.Errorf("1000 runes: %d", status)
	}
}

// V3 รีวิวร้านตัวเอง 403 · V4 รีวิวซ้ำ 409 · V5 แก้หรือลบของคนอื่น 404
func TestReviewV3V4V5(t *testing.T) {
	e := newEnv(t)
	owner, me, other := e.user("owner"), e.user("me"), e.user("other")
	r := e.restaurant(owner)
	path := "/api/restaurants/" + itoa(r) + "/reviews"

	if status, typ, _ := e.do(http.MethodPost, path, owner, reviewBody(5, "ร้านฉันดีสุด")); status != 403 || typ != "/problems/own-restaurant-review" {
		t.Errorf("V3: %d %s", status, typ)
	}
	status, _, body := e.do(http.MethodPost, path, me, reviewBody(4, "อร่อย"))
	if status != 201 {
		t.Fatalf("create: %d %v", status, body)
	}
	id := itoa(int64(body["id"].(float64)))
	if status, typ, _ := e.do(http.MethodPost, path, me, reviewBody(5, "อีกครั้ง")); status != 409 || typ != "/problems/already-reviewed" {
		t.Errorf("V4: %d %s", status, typ)
	}
	if status, typ, _ := e.do(http.MethodPut, "/api/reviews/"+id, other, reviewBody(1, "แก้ของคนอื่น")); status != 404 || typ != "/problems/not-found" {
		t.Errorf("V5 update: %d %s", status, typ)
	}
	if status, typ, _ := e.do(http.MethodDelete, "/api/reviews/"+id, other, ""); status != 404 || typ != "/problems/not-found" {
		t.Errorf("V5 delete: %d %s", status, typ)
	}
	if status, typ, _ := e.do(http.MethodPost, path, 0, reviewBody(5, "x")); status != 401 || typ != "/problems/unauthenticated" {
		t.Errorf("anonymous: %d %s", status, typ)
	}

	// ผู้เขียนแก้ได้ (ผ่าน handler) และเมื่อแก้หลังเวลาที่สร้าง edited = true
	if status, _, body := e.do(http.MethodPut, "/api/reviews/"+id, me, reviewBody(5, "แก้แล้ว")); status != 200 || body["rating"] != float64(5) {
		t.Errorf("update: %d %v", status, body)
	}
	reviewID, _ := strconv.ParseInt(id, 10, 64)
	rv, err := e.svc.Update(context.Background(), me, reviewID, review.Input{Aspects: review.Aspects{Food: 4, Ambience: 4, Service: 4, Value: 4}, Comment: "แก้อีกรอบ"}, testNow.Add(time.Hour))
	if err != nil || !rv.Edited {
		t.Errorf("edited = %v, err %v", rv.Edited, err)
	}
	// ลบได้ (204) แล้วรีวิวใหม่ได้
	if status, _, _ := e.do(http.MethodDelete, "/api/reviews/"+id, me, ""); status != 204 {
		t.Errorf("delete: %d", status)
	}
	if status, _, _ := e.do(http.MethodPost, path, me, reviewBody(3, "รีวิวใหม่")); status != 201 {
		t.Errorf("re-create: %d", status)
	}
}

// V6: คะแนน 4, 5, 4 → avg_rating 4.3, review_count 3
func TestReviewV6Average(t *testing.T) {
	e := newEnv(t)
	r := e.restaurant(e.user("owner"))
	for i, rating := range []int{4, 5, 4} {
		e.rate(r, e.user(fmt.Sprint("u", i)), rating)
	}
	_, items := e.list("highest_rated")
	if items[0]["avg_rating"] != 4.3 || items[0]["review_count"] != float64(3) {
		t.Errorf("item = %v", items[0])
	}
}

// V7: 5.0 จาก 1 รีวิว เทียบ 4.8 จาก 300 รีวิว → ร้าน 300 รีวิวอยู่บน
// V8: ร้านไม่มีรีวิวอยู่ท้ายสุด avg_rating = null
//
// หมายเหตุ: ผลนี้ขึ้นกับ m (ค่าเฉลี่ยทั้งระบบ) ด้วย ถ้ามีแค่สองร้านนี้ m ≈ 4.80 จะทำให้
// ร้าน 1 รีวิวได้ (5×4.80+5)/6 = 4.83 ชนะ 4.80 (ร้าน 300 รีวิวชนะเมื่อ m < 4.76)
// จึงใส่ร้านอื่นที่มีคะแนนกลางๆ ให้ m ใกล้ความจริง (≈ 4.3)
func TestReviewV7V8Bayesian(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	none := e.restaurant(owner) // id น้อยสุด แต่ต้องอยู่ท้าย
	one := e.restaurant(owner)
	many := e.restaurant(owner)
	average := e.restaurant(owner)
	e.rate(one, e.user("solo"), 5)
	e.manyRatings(many, 240, 5, "five")
	e.manyRatings(many, 60, 4, "four") // (240×5 + 60×4) / 300 = 4.8
	e.manyRatings(average, 100, 3, "three")

	ids, items := e.list("highest_rated")
	if fmt.Sprint(ids) != fmt.Sprint([]int64{many, one, average, none}) {
		t.Errorf("order = %v, want %v", ids, []int64{many, one, average, none})
	}
	if items[0]["avg_rating"] != 4.8 || items[1]["avg_rating"] != 5.0 || items[3]["avg_rating"] != nil || items[3]["review_count"] != float64(0) {
		t.Errorf("items = %v", items)
	}
}

// V9: ร้านที่ค่าทุกอย่างเท่ากัน ลำดับคงที่ตาม id · V10: most_reviewed จำนวนเท่ากัน ค่าเฉลี่ยสูงกว่าอยู่บน
func TestReviewV9V10TieBreak(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	a, b, c := e.restaurant(owner), e.restaurant(owner), e.restaurant(owner)
	u1, u2 := e.user("u1"), e.user("u2")
	e.rate(a, u1, 4)
	e.rate(b, u1, 4)
	e.rate(c, u1, 4)
	for range 3 {
		if ids, _ := e.list("highest_rated"); fmt.Sprint(ids) != fmt.Sprint([]int64{a, b, c}) {
			t.Fatalf("V9 order = %v", ids)
		}
	}

	e.rate(c, u2, 5) // c: 2 รีวิว เฉลี่ย 4.5
	e.rate(b, u2, 3) // b: 2 รีวิว เฉลี่ย 3.5
	if ids, _ := e.list("most_reviewed"); fmt.Sprint(ids) != fmt.Sprint([]int64{c, b, a}) {
		t.Errorf("V10 order = %v, want %v", ids, []int64{c, b, a})
	}
}

// V11: รายการรีวิว is_mine ทั้งตอน login และไม่ login, ลำดับ, แบ่งหน้าและ total
func TestReviewV11List(t *testing.T) {
	e := newEnv(t)
	r := e.restaurant(e.user("owner"))
	me := e.user("me")
	e.rate(r, e.user("a"), 3)
	e.rate(r, me, 4)
	e.rate(r, e.user("b"), 5) // ใหม่สุด

	path := "/api/restaurants/" + itoa(r) + "/reviews"
	_, _, body := e.do(http.MethodGet, path+"?page_size=2", me, "")
	items := body["items"].([]any)
	if body["total"] != float64(3) || len(items) != 2 || body["page_size"] != float64(2) {
		t.Fatalf("page 1 = %v", body)
	}
	first, second := items[0].(map[string]any), items[1].(map[string]any)
	if first["rating"] != float64(5) || first["is_mine"] != false || second["is_mine"] != true || second["edited"] != false {
		t.Errorf("items = %v", items)
	}
	_, _, body = e.do(http.MethodGet, path+"?page=2&page_size=2", me, "")
	if items := body["items"].([]any); len(items) != 1 || items[0].(map[string]any)["rating"] != float64(3) {
		t.Errorf("page 2 = %v", body)
	}
	_, _, body = e.do(http.MethodGet, path, 0, "")
	for _, it := range body["items"].([]any) {
		if it.(map[string]any)["is_mine"] != false {
			t.Errorf("ไม่ได้ login แต่ is_mine = true")
		}
	}
	if status, typ, _ := e.do(http.MethodGet, path+"?page_size=51", 0, ""); status != 400 || typ != "/problems/validation" {
		t.Errorf("page_size 51: %d %s", status, typ)
	}
}

// V12: ร้านที่ถูกลบไม่อยู่ในรายการร้าน รีวิวใหม่ แก้ ลบ และดูรีวิวได้ 404 · m ไม่นับรีวิวของร้านที่ถูกลบ
func TestReviewV12DeletedRestaurant(t *testing.T) {
	e := newEnv(t)
	owner, me := e.user("owner"), e.user("me")
	keep := e.restaurant(owner)
	gone := e.restaurant(owner)
	e.rate(gone, me, 1)
	var reviewID int64
	if err := e.pool.QueryRow(context.Background(), `SELECT id FROM reviews WHERE restaurant_id = $1`, gone).Scan(&reviewID); err != nil {
		t.Fatal(err)
	}
	e.exec(`UPDATE restaurants SET deleted_at = now() WHERE id = $1`, gone)

	ids, _ := e.list("highest_rated")
	if fmt.Sprint(ids) != fmt.Sprint([]int64{keep}) {
		t.Errorf("list = %v", ids)
	}
	_, _, body := e.do(http.MethodGet, "/api/restaurants", 0, "")
	if body["total"] != float64(1) {
		t.Errorf("total = %v", body["total"])
	}
	for _, c := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/restaurants/" + itoa(gone) + "/reviews", reviewBody(5, "x")},
		{http.MethodGet, "/api/restaurants/" + itoa(gone) + "/reviews", ""},
		{http.MethodPut, "/api/reviews/" + itoa(reviewID), reviewBody(5, "x")},
		{http.MethodDelete, "/api/reviews/" + itoa(reviewID), ""},
	} {
		if status, typ, _ := e.do(c.method, c.path, me, c.body); status != 404 || typ != "/problems/not-found" {
			t.Errorf("%s %s: %d %s", c.method, c.path, status, typ)
		}
	}
}

// รายการร้าน: sort ผิด 400, หมวด รูปปก is_mine ครบ
func TestListRestaurantsFields(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	r := e.restaurant(owner)
	e.exec(`INSERT INTO restaurant_categories (restaurant_id, category_id) VALUES ($1, 9), ($1, 1)`, r)
	e.exec(`INSERT INTO restaurant_images (restaurant_id, content_type, data) VALUES ($1, 'image/png', '\x01'), ($1, 'image/png', '\x02')`, r)

	if status, typ, _ := e.do(http.MethodGet, "/api/restaurants?sort=newest", 0, ""); status != 400 || typ != "/problems/validation" {
		t.Errorf("bad sort: %d %s", status, typ)
	}
	_, _, body := e.do(http.MethodGet, "/api/restaurants", owner, "")
	it := body["items"].([]any)[0].(map[string]any)
	cats := it["categories"].([]any)
	if len(cats) != 2 || cats[0].(map[string]any)["slug"] != "thai" || cats[1].(map[string]any)["name_th"] != "อาหารทะเล" {
		t.Errorf("categories = %v", cats)
	}
	if it["cover_image_id"] != float64(1) || it["is_mine"] != true || it["open_time"] != "10:00" || it["accepting_bookings"] != true {
		t.Errorf("item = %v", it)
	}
	if body["page"] != float64(1) || body["page_size"] != float64(20) {
		t.Errorf("paging = %v %v", body["page"], body["page_size"])
	}
}

// Q1 [B]: ค้น "ครัว" เจอ "ครัวริมคลอง" · ค้น % ได้อักขระ % จริง · หมวด OR · q AND หมวด · slug ไม่มี 400
func TestSearchQ1(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	mk := func(name, desc string, cats ...int) int64 {
		id := e.restaurant(owner)
		e.exec(`UPDATE restaurants SET name = $2, description = $3 WHERE id = $1`, id, name, desc)
		for _, c := range cats {
			e.exec(`INSERT INTO restaurant_categories (restaurant_id, category_id) VALUES ($1, $2)`, id, c)
		}
		return id
	}
	klong := mk("ครัวริมคลอง", "อาหารไทยบ้านๆ", 1)               // thai
	sushi := mk("ซูชิบาร์ ท่าเรือ", "ปลาสด ลด 50% ทุกวันพุธ", 4) // japanese
	sea := mk("ทะเลเผา", "กุ้ง_หอย ครัวทะเล", 9)                 // seafood
	mk("ร้านอื่น", "ไม่มีอะไร", 2)                               // isan

	search := func(q string) string {
		t.Helper()
		status, _, body := e.do("GET", "/api/restaurants?"+q, 0, "")
		if status != 200 {
			t.Fatalf("%s: %d %v", q, status, body)
		}
		var ids []int64
		for _, it := range body["items"].([]any) {
			ids = append(ids, int64(it.(map[string]any)["id"].(float64)))
		}
		if int(body["total"].(float64)) != len(ids) {
			t.Errorf("%s: total %v ไม่ตรงกับจำนวนรายการ %d", q, body["total"], len(ids))
		}
		return fmt.Sprint(ids)
	}
	esc := url.QueryEscape

	tests := []struct{ query, want string }{
		{"q=" + esc("ครัว"), fmt.Sprint([]int64{klong, sea})}, // ชื่อหรือรายละเอียด
		{"q=" + esc(" ครัวริม "), fmt.Sprint([]int64{klong})},
		{"q=" + esc("%"), fmt.Sprint([]int64{sushi})}, // % เป็นอักขระจริง ไม่ใช่ wildcard
		{"q=_", fmt.Sprint([]int64{sea})},             // _ เป็นอักขระจริง
		{"q=" + esc(`\`), "[]"},
		{"category=thai&category=seafood", fmt.Sprint([]int64{klong, sea})}, // หมวดใช้ OR
		{"q=" + esc("ครัว") + "&category=thai", fmt.Sprint([]int64{klong})}, // q AND หมวด
		{"q=", fmt.Sprint([]int64{klong, sushi, sea, 4})},                   // ว่าง = ไม่ค้น
	}
	for _, tc := range tests {
		if got := search(tc.query); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.query, got, tc.want)
		}
	}
	for _, q := range []string{"category=thai&category=nope", "q=" + strings.Repeat("ก", 101)} {
		if status, typ, _ := e.do("GET", "/api/restaurants?"+q, 0, ""); status != 400 || typ != "/problems/validation" {
			t.Errorf("%.40s: %d %s", q, status, typ)
		}
	}
}

// K3: ผู้ใช้เดียวส่งรีวิวร้านเดียวกันพร้อมกัน 2 ครั้ง → มี 1 แถว อีกคำขอได้ 409
func TestConcurrentK3DuplicateReview(t *testing.T) {
	e := newEnv(t)
	me := e.user("me")
	r := e.restaurant(e.user("owner"))

	var created, dup atomic.Int32
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := e.svc.Create(context.Background(), me, r, review.Input{Aspects: review.Aspects{Food: 5, Ambience: 5, Service: 5, Value: 5}, Comment: "พร้อมกัน"}, testNow)
			switch {
			case err == nil:
				created.Add(1)
			case strings.Contains(err.Error(), "already-reviewed"):
				dup.Add(1)
			default:
				t.Errorf("unexpected: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	var rows int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM reviews WHERE restaurant_id = $1`, r).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if created.Load() != 1 || dup.Load() != 1 || rows != 1 {
		t.Errorf("created %d dup %d rows %d", created.Load(), dup.Load(), rows)
	}
}

// V13 คะแนน 4 ด้าน: คะแนนรวมเป็นค่าเฉลี่ยที่ฐานข้อมูลคำนวณให้ และด้านที่ขาดหรือนอกช่วง → 400
func TestReviewAspectsV13(t *testing.T) {
	e := newEnv(t)
	owner, me := e.user("owner"), e.user("me")
	r := e.restaurant(owner)
	path := "/api/restaurants/" + itoa(r) + "/reviews"

	// ส่งไม่ครบ 4 ด้าน หรือค่านอกช่วง → 400 พร้อมบอกชื่อด้านที่ผิด
	for _, body := range []string{
		`{"aspects": {"food": 5, "ambience": 5, "service": 5}, "comment": "ขาดด้านความคุ้มค่า"}`,
		`{"aspects": {"food": 0, "ambience": 5, "service": 5, "value": 5}, "comment": "ศูนย์"}`,
		`{"aspects": {"food": 6, "ambience": 5, "service": 5, "value": 5}, "comment": "เกินห้า"}`,
		`{"comment": "ไม่ส่ง aspects เลย"}`,
	} {
		if status, typ, _ := e.do(http.MethodPost, path, me, body); status != 400 || typ != "/problems/validation" {
			t.Errorf("%s: %d %s", body, status, typ)
		}
	}

	// 5 + 4 + 3 + 4 = 16 / 4 = 4.00 ฐานข้อมูลคำนวณให้ ไม่ได้รับจาก body
	status, _, body := e.do(http.MethodPost, path,
		me, `{"aspects": {"food": 5, "ambience": 4, "service": 3, "value": 4}, "comment": "อาหารดี บริการช้า"}`)
	if status != 201 || body["rating"] != 4.0 {
		t.Fatalf("สร้าง: %d rating=%v", status, body["rating"])
	}
	asp := body["aspects"].(map[string]any)
	if asp["food"] != float64(5) || asp["service"] != float64(3) {
		t.Errorf("aspects = %v", asp)
	}
}
