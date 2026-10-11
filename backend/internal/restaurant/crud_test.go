package restaurant_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"devpool/internal/booking"
	"devpool/internal/httpx"
	"devpool/internal/restaurant"
)

// pngBytes คือรูป PNG จริงขนาดเล็ก
func pngBytes() []byte {
	var buf bytes.Buffer
	png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	return buf.Bytes()
}

// webpBytes คือหัวไฟล์ WebP ที่ http.DetectContentType รู้จัก
func webpBytes() []byte {
	return append([]byte("RIFF\x24\x00\x00\x00WEBPVP8 "), make([]byte, 32)...)
}

type file struct {
	field, name string
	data        []byte
}

// multipartReq สร้างคำขอ multipart (ไม่ตั้ง Content-Type เอง ให้ writer ใส่ boundary)
func multipartReq(method, path string, user int64, data string, files ...file) *http.Request {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if data != "" {
		mw.WriteField("data", data)
	}
	for _, f := range files {
		w, _ := mw.CreateFormFile(f.field, f.name)
		w.Write(f.data)
	}
	mw.Close()
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if user != 0 {
		req.Header.Set("X-Test-User", strconv.FormatInt(user, 10))
	}
	return req
}

func (e *env) send(req *http.Request) (int, string, map[string]any, *httptest.ResponseRecorder) {
	e.t.Helper()
	rec := httptest.NewRecorder()
	e.server().ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	typ := ""
	if rec.Header().Get("Content-Type") == "application/problem+json" {
		typ, _ = out["type"].(string)
	}
	return rec.Code, typ, out, rec
}

func createData(name string, cats ...int) string {
	if len(cats) == 0 {
		cats = []int{1, 9}
	}
	b, _ := json.Marshal(map[string]any{
		"name": name, "description": "อาหารไทยริมคลอง", "capacity": 20,
		"open_time": "10:00", "close_time": "22:00", "category_ids": cats,
	})
	return string(b)
}

func img(name string) file { return file{"images", name, pngBytes()} }

// createViaAPI สร้างร้านผ่าน POST /api/restaurants แล้วคืน id
func (e *env) createViaAPI(owner int64, name string) int64 {
	e.t.Helper()
	status, _, body, _ := e.send(multipartReq("POST", "/api/restaurants", owner, createData(name), img("a.png")))
	if status != 201 {
		e.t.Fatalf("create %q: %d %v", name, status, body)
	}
	return int64(body["id"].(float64))
}

// W1: สร้างร้านครบถ้วน (2 หมวด, 1 รูป) → 201 พร้อมรายละเอียด
func TestCreateW1(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	status, _, body, _ := e.send(multipartReq("POST", "/api/restaurants", owner, createData("  ครัวริมคลอง  "), img("a.png")))
	if status != 201 {
		t.Fatalf("got %d %v", status, body)
	}
	cats := body["categories"].([]any)
	if body["name"] != "ครัวริมคลอง" || body["capacity"] != float64(20) || body["is_owner"] != true ||
		len(cats) != 2 || len(body["image_ids"].([]any)) != 1 ||
		body["cancel_window_minutes"] != float64(30) || body["min_lead_minutes"] != float64(120) ||
		body["accepting_bookings"] != true || body["avg_rating"] != nil || body["my_review"] != nil {
		t.Errorf("body = %v", body)
	}
	// webp และนโยบายที่กำหนดเองก็ใช้ได้
	data := `{"name": "ร้านสอง", "description": "x", "capacity": 5, "open_time": "18:00", "close_time": "02:00",
	          "cancel_window_minutes": 60, "min_lead_minutes": 60, "category_ids": [3]}`
	if status, _, body, _ := e.send(multipartReq("POST", "/api/restaurants", owner, data, file{"images", "a.webp", webpBytes()})); status != 201 || body["close_time"] != "02:00" {
		t.Errorf("webp/overnight: %d %v", status, body)
	}
	if status, typ, _, _ := e.send(multipartReq("POST", "/api/restaurants", 0, createData("x"), img("a.png"))); status != 401 || typ != "/problems/unauthenticated" {
		t.Errorf("anonymous: %d %s", status, typ)
	}
}

// W2–W5, W8: กติกาข้อมูล
func TestCreateValidation(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	big := append(pngBytes(), make([]byte, 3<<20)...)
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	withData := func(patch string) string {
		var m map[string]any
		json.Unmarshal([]byte(createData("ร้าน")), &m)
		json.Unmarshal([]byte(patch), &m)
		b, _ := json.Marshal(m)
		return string(b)
	}
	tests := []struct {
		name   string
		data   string
		files  []file
		status int
		typ    string
	}{
		{"W2 ไม่มีรูป", createData("ร้าน"), nil, 400, "validation"},
		{"W2 4 รูป", createData("ร้าน"), []file{img("1.png"), img("2.png"), img("3.png"), img("4.png")}, 400, "validation"},
		{"W3 รูป 3 MB", createData("ร้าน"), []file{{"images", "big.png", big}}, 413, "image-too-large"},
		{"W3 ไฟล์ข้อความตั้งชื่อ .png", createData("ร้าน"), []file{{"images", "fake.png", []byte("hello, not an image")}}, 415, "unsupported-image-type"},
		{"W3 SVG", createData("ร้าน"), []file{{"images", "x.svg", svg}}, 415, "unsupported-image-type"},
		{"W4 0 หมวด", withData(`{"category_ids": []}`), []file{img("a.png")}, 400, "validation"},
		{"W4 4 หมวด", withData(`{"category_ids": [1,2,3,4]}`), []file{img("a.png")}, 400, "validation"},
		{"W4 หมวดซ้ำ", withData(`{"category_ids": [1,1]}`), []file{img("a.png")}, 400, "validation"},
		{"W4 หมวดที่ไม่มี", withData(`{"category_ids": [1,99]}`), []file{img("a.png")}, 400, "validation"},
		{"W5 เปิด = ปิด", withData(`{"close_time": "10:00"}`), []file{img("a.png")}, 400, "validation"},
		{"W5 เวลาไม่ลงช่อง", withData(`{"open_time": "10:15"}`), []file{img("a.png")}, 400, "validation"},
		{"W5 capacity 0", withData(`{"capacity": 0}`), []file{img("a.png")}, 400, "validation"},
		{"W5 capacity 501", withData(`{"capacity": 501}`), []file{img("a.png")}, 400, "validation"},
		{"W5 ชื่อว่าง", withData(`{"name": "   "}`), []file{img("a.png")}, 400, "validation"},
		{"W5 cancel 20", withData(`{"cancel_window_minutes": 20}`), []file{img("a.png")}, 422, "invalid-restaurant-policy"},
		{"W5 lead < cancel", withData(`{"cancel_window_minutes": 60, "min_lead_minutes": 30}`), []file{img("a.png")}, 422, "invalid-restaurant-policy"},
		{"W8 ส่ง owner_id", withData(`{"owner_id": 999}`), []file{img("a.png")}, 400, "validation"},
		{"data ไม่ใช่ JSON", "{", []file{img("a.png")}, 400, "validation"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, typ, body, _ := e.send(multipartReq("POST", "/api/restaurants", owner, tc.data, tc.files...))
			if status != tc.status || typ != "/problems/"+tc.typ {
				t.Errorf("got %d %s %v, want %d %s", status, typ, body, tc.status, tc.typ)
			}
		})
	}
	// body ไม่ใช่ multipart → 400 · body เกิน 7 MB → 413
	req := httptest.NewRequest("POST", "/api/restaurants", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User", itoa(owner))
	if status, typ, _, _ := e.send(req); status != 400 || typ != "/problems/validation" {
		t.Errorf("not multipart: %d %s", status, typ)
	}
	huge := make([]byte, 8<<20)
	if status, typ, _, _ := e.send(multipartReq("POST", "/api/restaurants", owner, createData("ร้าน"), file{"images", "huge.png", huge})); status != 413 || typ != "/problems/image-too-large" {
		t.Errorf("body > 7 MB: %d %s", status, typ)
	}
	var n int
	e.pool.QueryRow(context.Background(), `SELECT count(*) FROM restaurants`).Scan(&n)
	if n != 0 {
		t.Errorf("restaurants = %d, ไม่ควรมีร้านถูกสร้าง", n)
	}
}

// W6: ชื่อซ้ำของเจ้าของเดิม (ต่างตัวพิมพ์/มีช่องว่าง) 409 · ของเจ้าของอื่น 201 · D5 ชื่อเดิมหลังลบ 201
func TestCreateW6NameUnique(t *testing.T) {
	e := newEnv(t)
	owner, other := e.user("owner"), e.user("other")
	id := e.createViaAPI(owner, "Sushi Bar")
	if status, typ, _, _ := e.send(multipartReq("POST", "/api/restaurants", owner, createData("  sushi BAR "), img("a.png"))); status != 409 || typ != "/problems/restaurant-name-taken" {
		t.Errorf("same owner: %d %s", status, typ)
	}
	e.createViaAPI(other, "Sushi Bar")

	// D5: ลบแล้วสร้างชื่อเดิมได้ (partial unique index ไม่นับร้านที่ถูกลบ)
	if status, _, _ := e.do("DELETE", "/api/restaurants/"+itoa(id), owner, ""); status != 204 {
		t.Fatalf("delete: %d", status)
	}
	e.createViaAPI(owner, "Sushi Bar")
}

// W7: ส่งสร้างซ้ำพร้อมกัน 2 ครั้ง → ได้ร้านเดียว
func TestConcurrentW7DuplicateCreate(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	var created, taken atomic.Int32
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := e.svc.Create(context.Background(), owner, restaurant.NewRestaurant{
				Info:     restaurant.Info{Name: "ร้านเดียว", Description: "x", CategoryIDs: []int{1}},
				Settings: restaurant.Settings{Capacity: 10, Open: 10 * time.Hour, Close: 22 * time.Hour, CancelWindowMinutes: 30, MinLeadMinutes: 120},
			}, testNow)
			switch {
			case err == nil:
				created.Add(1)
			case strings.Contains(err.Error(), "restaurant-name-taken"):
				taken.Add(1)
			default:
				t.Errorf("unexpected: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if created.Load() != 1 || taken.Load() != 1 {
		t.Errorf("created %d taken %d", created.Load(), taken.Load())
	}
}

// E1: ทุก endpoint เขียนของส่วนนี้ ด้วยผู้ใช้ที่ไม่ใช่เจ้าของ → 403
// E2: ร้านไม่มีอยู่ / ถูกลบแล้ว → 404
func TestE1E2Permissions(t *testing.T) {
	e := newEnv(t)
	owner, other := e.user("owner"), e.user("other")
	r := e.createViaAPI(owner, "ร้าน")
	gone := e.createViaAPI(owner, "ร้านที่ลบ")
	if status, _, _ := e.do("DELETE", "/api/restaurants/"+itoa(gone), owner, ""); status != 204 {
		t.Fatal("delete")
	}
	var imageID int64
	e.pool.QueryRow(context.Background(), `SELECT min(id) FROM restaurant_images WHERE restaurant_id = $1`, r).Scan(&imageID)

	info := `{"name": "ชื่อใหม่", "description": "x", "category_ids": [1]}`
	reqs := func(id int64) []*http.Request {
		p := "/api/restaurants/" + itoa(id)
		mk := func(method, path, body string) *http.Request {
			req := httptest.NewRequest(method, path, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			return req
		}
		return []*http.Request{
			mk("PATCH", p, info),
			mk("PATCH", p+"/settings", settings(10, "10:00", "22:00", 30, 120)),
			multipartReq("POST", p+"/images", 0, "", file{"image", "x.png", pngBytes()}),
			mk("DELETE", p+"/images/"+itoa(imageID), ""),
			mk("DELETE", p, ""),
		}
	}
	for _, req := range reqs(r) {
		req.Header.Set("X-Test-User", itoa(other))
		if status, typ, _, _ := e.send(req); status != 403 || typ != "/problems/not-restaurant-owner" {
			t.Errorf("E1 %s %s: %d %s", req.Method, req.URL.Path, status, typ)
		}
	}
	for _, id := range []int64{gone, 999999} {
		for _, req := range reqs(id) {
			req.Header.Set("X-Test-User", itoa(owner))
			if status, typ, _, _ := e.send(req); status != 404 || typ != "/problems/not-found" {
				t.Errorf("E2 %s %s: %d %s", req.Method, req.URL.Path, status, typ)
			}
		}
	}
}

// R1: รายละเอียดร้านโดยไม่ login / is_owner และ my_review ถูกต้อง · แก้ข้อมูลร้าน
func TestDetailR1AndUpdateInfo(t *testing.T) {
	e := newEnv(t)
	owner, me := e.user("owner"), e.user("me")
	r := e.createViaAPI(owner, "ร้าน")
	e.exec(`INSERT INTO reviews (restaurant_id, user_id, rating_food, rating_ambience, rating_service, rating_value, comment)
	        VALUES ($1, $2, 4, 4, 4, 4, 'ดี')`, r, me)

	_, _, anon := e.do("GET", "/api/restaurants/"+itoa(r), 0, "")
	if anon["is_owner"] != false || anon["my_review"] != nil || anon["avg_rating"] != 4.0 || anon["review_count"] != float64(1) {
		t.Errorf("anonymous: %v", anon)
	}
	_, _, mine := e.do("GET", "/api/restaurants/"+itoa(r), me, "")
	if mr, ok := mine["my_review"].(map[string]any); !ok || mr["rating"] != float64(4) || mr["comment"] != "ดี" || mine["is_owner"] != false {
		t.Errorf("me: %v", mine)
	}
	_, _, own := e.do("GET", "/api/restaurants/"+itoa(r), owner, "")
	if own["is_owner"] != true {
		t.Errorf("owner: %v", own)
	}

	status, _, body := e.do("PATCH", "/api/restaurants/"+itoa(r), owner, `{"name": " ชื่อใหม่ ", "description": "ใหม่", "category_ids": [5]}`)
	cats := body["categories"].([]any)
	if status != 200 || body["name"] != "ชื่อใหม่" || len(cats) != 1 || cats[0].(map[string]any)["slug"] != "chinese" {
		t.Errorf("update: %d %v", status, body)
	}
	if status, typ, _ := e.do("PATCH", "/api/restaurants/"+itoa(r), owner, `{"name": "x", "description": "y", "category_ids": [42]}`); status != 400 || typ != "/problems/validation" {
		t.Errorf("unknown category: %d %s", status, typ)
	}
	e.createViaAPI(owner, "ร้านสอง")
	if status, typ, _ := e.do("PATCH", "/api/restaurants/"+itoa(r), owner, `{"name": "ร้านสอง", "description": "y", "category_ids": [1]}`); status != 409 || typ != "/problems/restaurant-name-taken" {
		t.Errorf("rename to taken: %d %s", status, typ)
	}
	if status, _, body := e.do("GET", "/api/categories", 0, ""); status != 200 || body != nil {
		// categories เป็น array จึง unmarshal ลง map ไม่ได้ (body = nil)
		t.Errorf("categories: %d", status)
	}
}

// R2: ร้านของฉัน: เฉพาะของตัวเอง ไม่รวมที่ลบ และ upcoming_count ถูกต้อง
func TestMineR2(t *testing.T) {
	e := newEnv(t)
	owner, other, cust := e.user("owner"), e.user("other"), e.user("cust")
	a := e.createViaAPI(owner, "A")
	b := e.createViaAPI(owner, "B")
	gone := e.createViaAPI(owner, "C")
	e.createViaAPI(other, "D")
	e.do("DELETE", "/api/restaurants/"+itoa(gone), owner, "")
	e.existing(a, cust, day11("12:00"), day11("13:00"), 2)
	e.existing(a, cust, day11("14:00"), day11("15:00"), 2)
	e.existing(a, cust, at("2026-10-09 12:00"), at("2026-10-09 13:00"), 2) // ผ่านไปแล้ว ไม่นับ

	req := httptest.NewRequest("GET", "/api/me/restaurants", nil)
	req.Header.Set("X-Test-User", itoa(owner))
	rec := httptest.NewRecorder()
	e.server().ServeHTTP(rec, req)
	var list []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &list)
	if rec.Code != 200 || len(list) != 2 || list[0]["id"] != float64(b) || list[1]["id"] != float64(a) ||
		list[1]["upcoming_count"] != float64(2) || list[0]["upcoming_count"] != float64(0) || list[0]["cover_image_id"] == nil {
		t.Errorf("got %d %v", rec.Code, list)
	}
}

// I1–I4: รูป
func TestImagesI1toI4(t *testing.T) {
	e := newEnv(t)
	owner, other := e.user("owner"), e.user("other")
	r := e.createViaAPI(owner, "ร้าน")
	r2 := e.createViaAPI(other, "ร้านอื่น")
	add := func() (int, string, map[string]any) {
		status, typ, body, _ := e.send(multipartReq("POST", "/api/restaurants/"+itoa(r)+"/images", owner, "", file{"image", "x.png", pngBytes()}))
		return status, typ, body
	}
	if status, _, body := add(); status != 201 || body["id"] == nil {
		t.Fatalf("add 2nd: %d %v", status, body)
	}
	add()
	if status, typ, _ := add(); status != 409 || typ != "/problems/image-limit-reached" { // I1
		t.Errorf("I1: %d %s", status, typ)
	}

	var ids []int64
	rows, _ := e.pool.Query(context.Background(), `SELECT id FROM restaurant_images WHERE restaurant_id = $1 ORDER BY id`, r)
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		ids = append(ids, id)
	}
	var otherImage int64
	e.pool.QueryRow(context.Background(), `SELECT min(id) FROM restaurant_images WHERE restaurant_id = $1`, r2).Scan(&otherImage)

	// I3: ลบรูปของร้านอื่นผ่าน path ร้านตัวเอง → 404
	if status, typ, _ := e.do("DELETE", "/api/restaurants/"+itoa(r)+"/images/"+itoa(otherImage), owner, ""); status != 404 || typ != "/problems/not-found" {
		t.Errorf("I3: %d %s", status, typ)
	}
	// I2: ลบจนเหลือรูปสุดท้าย → 409
	for _, id := range ids[:2] {
		if status, _, _ := e.do("DELETE", "/api/restaurants/"+itoa(r)+"/images/"+itoa(id), owner, ""); status != 204 {
			t.Errorf("delete %d: %d", id, status)
		}
	}
	if status, typ, _ := e.do("DELETE", "/api/restaurants/"+itoa(r)+"/images/"+itoa(ids[2]), owner, ""); status != 409 || typ != "/problems/last-image" {
		t.Errorf("I2: %d %s", status, typ)
	}

	// I4: ส่งรูป header ถูกต้อง (สาธารณะ) · รูปของร้านอื่นผ่าน path นี้ → 404
	_, _, _, rec := e.send(httptest.NewRequest("GET", "/api/restaurants/"+itoa(r)+"/images/"+itoa(ids[2]), nil))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" ||
		rec.Header().Get("X-Content-Type-Options") != "nosniff" ||
		rec.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" ||
		!bytes.Equal(rec.Body.Bytes(), pngBytes()) {
		t.Errorf("I4: %d %v", rec.Code, rec.Header())
	}
	if status, _, _, _ := e.send(httptest.NewRequest("GET", "/api/restaurants/"+itoa(r)+"/images/"+itoa(otherImage), nil)); status != 404 {
		t.Errorf("I4 other: %d", status)
	}
	// รูปไม่ถูกชนิด / ไม่มีไฟล์
	if status, typ, _, _ := e.send(multipartReq("POST", "/api/restaurants/"+itoa(r)+"/images", owner, "", file{"image", "x.png", []byte("text")})); status != 415 || typ != "/problems/unsupported-image-type" {
		t.Errorf("bad type: %d %s", status, typ)
	}
	if status, typ, _, _ := e.send(multipartReq("POST", "/api/restaurants/"+itoa(r)+"/images", owner, "")); status != 400 || typ != "/problems/validation" {
		t.Errorf("no file: %d %s", status, typ)
	}
}

// K6: เพิ่มรูปพร้อมกันตอนมี 2 รูป → สำเร็จ 1 · K7: ลบ 2 รูปที่เหลือพร้อมกัน → เหลืออย่างน้อย 1
func TestConcurrentK6K7Images(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	r := e.createViaAPI(owner, "ร้าน")
	if _, err := e.svc.AddImage(context.Background(), owner, r, imgOf(), testNow); err != nil {
		t.Fatal(err)
	}

	var ok atomic.Int32
	run := func(n int, fn func() error) {
		start := make(chan struct{})
		var wg sync.WaitGroup
		for range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				if fn() == nil {
					ok.Add(1)
				}
			}()
		}
		close(start)
		wg.Wait()
	}
	run(5, func() error { _, err := e.svc.AddImage(context.Background(), owner, r, imgOf(), testNow); return err })
	if ok.Load() != 1 || e.imageCount(r) != 3 {
		t.Errorf("K6: success %d, images %d", ok.Load(), e.imageCount(r))
	}

	// เหลือ 2 รูปแล้วลบทั้งสองพร้อมกัน
	var first int64
	e.pool.QueryRow(context.Background(), `SELECT min(id) FROM restaurant_images WHERE restaurant_id = $1`, r).Scan(&first)
	e.svc.DeleteImage(context.Background(), owner, r, first)
	var ids []int64
	rows, _ := e.pool.Query(context.Background(), `SELECT id FROM restaurant_images WHERE restaurant_id = $1`, r)
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		ids = append(ids, id)
	}
	var i atomic.Int32
	run(2, func() error { return e.svc.DeleteImage(context.Background(), owner, r, ids[i.Add(1)-1]) })
	if n := e.imageCount(r); n < 1 {
		t.Errorf("K7: images %d", n)
	}
}

func imgOf() httpx.Image { return httpx.Image{ContentType: "image/png", Data: pngBytes()} }

func (e *env) imageCount(r int64) int {
	var n int
	e.pool.QueryRow(context.Background(), `SELECT count(*) FROM restaurant_images WHERE restaurant_id = $1`, r).Scan(&n)
	return n
}

// D1–D4: ลบร้าน
func TestDeleteD1toD4(t *testing.T) {
	e := newEnv(t)
	owner, cust := e.user("owner"), e.user("cust")
	r := e.createViaAPI(owner, "ร้าน")
	future := e.existing(r, cust, day11("12:00"), day11("13:00"), 2)
	e.existing(r, cust, at("2026-10-12 18:00"), at("2026-10-12 19:00"), 2)

	// D1: มีการจองในอนาคต → 409 พร้อม upcoming_count, latest_booking_end
	status, typ, body := e.do("DELETE", "/api/restaurants/"+itoa(r), owner, "")
	if status != 409 || typ != "/problems/restaurant-has-upcoming-bookings" ||
		body["upcoming_count"] != float64(2) || body["latest_booking_end"] != "2026-10-12T19:00:00+07:00" || body["accepting_bookings"] != true {
		t.Fatalf("D1: %d %s %v", status, typ, body)
	}

	// D2: เหลือแต่การจองที่ยกเลิกแล้วหรือจบไปแล้ว → 204
	e.exec(`UPDATE bookings SET status = 'cancelled', cancelled_at = now() WHERE restaurant_id = $1 AND id <> $2`, r, future)
	e.exec(`UPDATE bookings SET start_at = '2026-10-09 12:00+07', end_at = '2026-10-09 13:00+07', cancel_deadline = '2026-10-09 11:30+07' WHERE id = $1`, future)
	if status, _, body := e.do("DELETE", "/api/restaurants/"+itoa(r), owner, ""); status != 204 {
		t.Fatalf("D2: %d %v", status, body)
	}

	// D3: หลังลบ รายละเอียด รูป ตั้งค่า → 404 (รายการร้าน จอง รีวิว availability ทดสอบแล้วใน package อื่น)
	for _, c := range []struct{ method, path, body string }{
		{"GET", "/api/restaurants/" + itoa(r), ""},
		{"GET", "/api/restaurants/" + itoa(r) + "/images/1", ""},
		{"PATCH", "/api/restaurants/" + itoa(r) + "/settings", settings(10, "10:00", "22:00", 30, 120)},
		{"DELETE", "/api/restaurants/" + itoa(r), ""},
	} {
		if status, _, _ := e.do(c.method, c.path, owner, c.body); status != 404 {
			t.Errorf("D3 %s %s: %d", c.method, c.path, status)
		}
	}

	// D4: การจองในอดีตยังอยู่ในการจองของฉันพร้อม restaurant_deleted
	res, err := e.bookings.MyBookings(context.Background(), cust, "past", pageOf(), testNow)
	if err != nil || len(res.Items) != 1 || !res.Items[0].RestaurantDeleted {
		t.Errorf("D4: %+v %v", res.Items, err)
	}
}

// K5: ลบร้านพร้อมกับ 10 คนจอง → ห้ามจบที่ร้านถูกลบทั้งที่มีการจอง active ในอนาคต
func TestConcurrentK5DeleteWhileBooking(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	r := e.createViaAPI(owner, "ร้าน")
	users := make([]int64, 10)
	for i := range users {
		users[i] = e.user(fmt.Sprint("k5-", i))
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		_ = e.svc.Delete(context.Background(), owner, r, testNow)
	}()
	for _, u := range users {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _ = e.bookings.Create(context.Background(), u, r,
				booking.Request{PartySize: 1, StartAt: day11("12:00"), EndAt: day11("13:00")}, testNow)
		}()
	}
	close(start)
	wg.Wait()

	var deleted bool
	var active int
	e.pool.QueryRow(context.Background(), `
		SELECT r.deleted_at IS NOT NULL,
		       (SELECT count(*) FROM bookings WHERE restaurant_id = r.id AND status = 'active' AND end_at > $2)
		  FROM restaurants r WHERE r.id = $1`, r, testNow).Scan(&deleted, &active)
	if deleted && active > 0 {
		t.Errorf("ร้านถูกลบทั้งที่มีการจอง active ในอนาคต %d รายการ", active)
	}
}

func pageOf() httpx.Page { return httpx.Page{Page: 1, Size: 20} }

// L1–L4 ที่ตั้งร้าน: ไม่ระบุได้ · ระบุแล้วคืนค่ากลับ · ต้องมาคู่กัน · นอกช่วง 400 · แก้ไขและล้างได้
func TestRestaurantLocationL1toL4(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")

	withLocation := func(name string, extra map[string]any) string {
		m := map[string]any{
			"name": name, "description": "ร้านทดสอบที่ตั้ง", "capacity": 20,
			"open_time": "10:00", "close_time": "22:00", "category_ids": []int{1},
		}
		for k, v := range extra {
			m[k] = v
		}
		b, _ := json.Marshal(m)
		return string(b)
	}
	create := func(name string, extra map[string]any) (int, map[string]any) {
		status, _, body, _ := e.send(multipartReq("POST", "/api/restaurants", owner, withLocation(name, extra), img("a.png")))
		return status, body
	}

	// L1 ไม่ระบุที่ตั้ง → สร้างได้ ได้ null ทั้งคู่ (ร้านเดิมที่ยังไม่มีพิกัดต้องใช้งานได้ต่อ)
	status, body := create("ร้านไม่มีพิกัด", nil)
	if status != 201 || body["latitude"] != nil || body["longitude"] != nil {
		t.Fatalf("L1: %d lat=%v lng=%v", status, body["latitude"], body["longitude"])
	}
	if body["address"] != nil {
		t.Errorf("L1: ไม่ส่ง address ต้องได้ null ได้ %v", body["address"])
	}

	// L2 ระบุพิกัด → คืนค่าเดิมกลับมา
	status, body = create("ร้านมีพิกัด", map[string]any{"latitude": 13.7466, "longitude": 100.5347, "address": "  ซอยริมคลอง 12  "})
	if status != 201 || body["latitude"] != 13.7466 || body["longitude"] != 100.5347 {
		t.Fatalf("L2: %d %v", status, body)
	}
	if body["address"] != "ซอยริมคลอง 12" {
		t.Errorf("L2: address ต้องถูกตัดช่องว่างหัวท้าย ได้ %q", body["address"])
	}
	id := int64(body["id"].(float64))

	// L3 ส่งมาตัวเดียว หรือค่านอกช่วง → 400
	for _, c := range []struct {
		name  string
		extra map[string]any
	}{
		{"เฉพาะ lat", map[string]any{"latitude": 13.74}},
		{"เฉพาะ lng", map[string]any{"longitude": 100.53}},
		{"lat เกิน", map[string]any{"latitude": 91.0, "longitude": 100.53}},
		{"lng เกิน", map[string]any{"latitude": 13.74, "longitude": -181.0}},
	} {
		if status, body := create("ร้าน "+c.name, c.extra); status != 400 {
			t.Errorf("L3 %s: %d %v", c.name, status, body)
		}
	}

	// L4 แก้ไขพิกัด แล้วล้างทิ้งด้วยการส่ง null
	patch := `{"name":"ร้านมีพิกัด","description":"ย้ายที่","category_ids":[1],"latitude":13.7563,"longitude":100.5018}`
	if status, _, body := e.do("PATCH", "/api/restaurants/"+itoa(id), owner, patch); status != 200 {
		t.Fatalf("L4 patch: %d %v", status, body)
	}
	_, _, d := e.do("GET", "/api/restaurants/"+itoa(id), owner, "")
	if d["latitude"] != 13.7563 || d["longitude"] != 100.5018 {
		t.Errorf("L4 moved = %v %v", d["latitude"], d["longitude"])
	}
	clear := `{"name":"ร้านมีพิกัด","description":"ไม่ระบุที่ตั้งแล้ว","category_ids":[1]}`
	if status, _, body := e.do("PATCH", "/api/restaurants/"+itoa(id), owner, clear); status != 200 {
		t.Fatalf("L4 clear: %d %v", status, body)
	}
	_, _, d = e.do("GET", "/api/restaurants/"+itoa(id), owner, "")
	if d["latitude"] != nil || d["longitude"] != nil {
		t.Errorf("L4 cleared = %v %v", d["latitude"], d["longitude"])
	}
}

// L5 ที่อยู่: ยาวเกิน 200 → 400 · ช่องว่างล้วน = ไม่ระบุ · ระบุที่อยู่อย่างเดียวโดยไม่ปักหมุดได้
func TestRestaurantAddressL5(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	data := func(name string, address any) string {
		b, _ := json.Marshal(map[string]any{
			"name": name, "description": "ร้านทดสอบที่อยู่", "capacity": 20,
			"open_time": "10:00", "close_time": "22:00", "category_ids": []int{1}, "address": address,
		})
		return string(b)
	}

	long := strings.Repeat("ก", 201)
	if status, _, body, _ := e.send(multipartReq("POST", "/api/restaurants", owner, data("ร้านยาว", long), img("a.png"))); status != 400 {
		t.Errorf("ที่อยู่ 201 ตัวอักษรต้องได้ 400 ได้ %d %v", status, body)
	}

	// ช่องว่างล้วนถือว่าไม่ระบุ ไม่ใช่ error และไม่เก็บสตริงว่างลงฐาน (CHECK ห้ามไว้)
	status, _, body, _ := e.send(multipartReq("POST", "/api/restaurants", owner, data("ร้านช่องว่าง", "   "), img("a.png")))
	if status != 201 || body["address"] != nil {
		t.Errorf("ช่องว่างล้วนต้องได้ null: %d %v", status, body["address"])
	}

	// ที่อยู่กับพิกัดเป็นอิสระต่อกัน ระบุอย่างใดอย่างหนึ่งได้
	status, _, body, _ = e.send(multipartReq("POST", "/api/restaurants", owner, data("ร้านมีแต่ที่อยู่", "ถนนเยาวราช"), img("a.png")))
	if status != 201 || body["address"] != "ถนนเยาวราช" || body["latitude"] != nil {
		t.Errorf("ที่อยู่อย่างเดียว: %d %v", status, body)
	}
}

// V14 ค่าเฉลี่ยรายด้านและรีวิวของฉันในรายละเอียดร้าน (คู่กับ V13 ของแพ็กเกจ review)
func TestDetailAspectAveragesV14(t *testing.T) {
	e := newEnv(t)
	owner, a, b := e.user("owner"), e.user("แอน"), e.user("บี")
	r := e.createViaAPI(owner, "ร้านมีรีวิว")

	// ร้านยังไม่มีรีวิว → null ไม่ใช่ศูนย์ (หน้าเว็บจะได้ไม่โชว์ "0.0 ดาว")
	_, _, empty := e.do("GET", "/api/restaurants/"+itoa(r), a, "")
	if empty["aspect_averages"] != nil || empty["avg_rating"] != nil || empty["my_review"] != nil {
		t.Fatalf("ร้านไม่มีรีวิว: %v", empty)
	}

	rate := func(user int64, food, ambience, service, value int) {
		e.exec(`INSERT INTO reviews (restaurant_id, user_id, rating_food, rating_ambience, rating_service, rating_value, comment)
		        VALUES ($1, $2, $3, $4, $5, $6, 'ok')`, r, user, food, ambience, service, value)
	}
	rate(a, 5, 4, 3, 4) // รวม 4.00
	rate(b, 3, 4, 5, 4) // รวม 4.00

	_, _, d := e.do("GET", "/api/restaurants/"+itoa(r), a, "")
	avg := d["aspect_averages"].(map[string]any)
	// อาหารกับบริการเฉลี่ยคนละทาง แต่คะแนนรวมเท่ากัน จึงพิสูจน์ว่าแยกรายด้านมีประโยชน์จริง
	if avg["food"] != float64(4) || avg["ambience"] != float64(4) || avg["service"] != float64(4) || avg["value"] != float64(4) {
		t.Errorf("aspect_averages = %v", avg)
	}
	if d["avg_rating"] != 4.0 || d["review_count"] != float64(2) {
		t.Errorf("avg = %v count = %v", d["avg_rating"], d["review_count"])
	}
	mine := d["my_review"].(map[string]any)
	if mine["rating"] != 4.0 || mine["aspects"].(map[string]any)["service"] != float64(3) {
		t.Errorf("my_review = %v", mine)
	}
}

// F1–F5 ร้านที่ถูกใจ: ต้อง login · กดซ้ำได้ · เอาออกซ้ำได้ · เจ้าของกดร้านตัวเองได้ · ร้านที่ถูกลบไม่อยู่ในรายการ
func TestFavoritesF1toF5(t *testing.T) {
	e := newEnv(t)
	owner, me := e.user("owner"), e.user("me")
	a := e.createViaAPI(owner, "ร้านหนึ่ง")
	b := e.createViaAPI(owner, "ร้านสอง")
	fav := func(id int64) string { return "/api/restaurants/" + itoa(id) + "/favorite" }

	// F1 ยังไม่ login → 401
	if status, typ, _ := e.do("PUT", fav(a), 0, ""); status != 401 || typ != "/problems/unauthenticated" {
		t.Errorf("F1: %d %s", status, typ)
	}
	// ร้านไม่มีอยู่ → 404
	if status, typ, _ := e.do("PUT", fav(999999), me, ""); status != 404 || typ != "/problems/not-found" {
		t.Errorf("ร้านไม่มี: %d %s", status, typ)
	}

	// F2 กดถูกใจซ้ำได้ ไม่ใช่ 409 (primary key ตัดสินความซ้ำ)
	for range 2 {
		if status, _, body := e.do("PUT", fav(a), me, ""); status != 200 || body["is_favorite"] != true {
			t.Errorf("F2: %d %v", status, body)
		}
	}
	if _, _, d := e.do("GET", "/api/restaurants/"+itoa(a), me, ""); d["is_favorite"] != true {
		t.Errorf("F2 detail: %v", d["is_favorite"])
	}
	// คนอื่นยังไม่ได้กด ต้องเห็นเป็น false (ไม่ใช่ของใครก็ได้)
	if _, _, d := e.do("GET", "/api/restaurants/"+itoa(a), owner, ""); d["is_favorite"] != false {
		t.Errorf("F2 คนอื่น: %v", d["is_favorite"])
	}

	// F3 เอาออกซ้ำได้
	for range 2 {
		if status, _, body := e.do("DELETE", fav(a), me, ""); status != 200 || body["is_favorite"] != false {
			t.Errorf("F3: %d %v", status, body)
		}
	}

	// F4 เจ้าของกดถูกใจร้านตัวเองได้ (ต่างจากจองและรีวิวที่ห้าม)
	if status, _, _ := e.do("PUT", fav(a), owner, ""); status != 200 {
		t.Errorf("F4: %d", status)
	}

	// F5 รายการของฉันเรียงใหม่สุดก่อน และไม่รวมร้านที่ถูกลบ
	e.do("PUT", fav(a), me, "")
	e.do("PUT", fav(b), me, "")
	_, _, list := e.do("GET", "/api/me/favorites", me, "")
	if list != nil {
		t.Fatalf("รายการต้องเป็น array ไม่ใช่ object: %v", list)
	}
	names := favoriteNames(t, e, me)
	if len(names) != 2 || names[0] != "ร้านสอง" || names[1] != "ร้านหนึ่ง" {
		t.Errorf("F5 ลำดับ = %v", names)
	}
	if status, _, body := e.do("DELETE", "/api/restaurants/"+itoa(b), owner, ""); status != 204 {
		t.Fatalf("ลบร้าน: %d %v", status, body)
	}
	if names := favoriteNames(t, e, me); len(names) != 1 || names[0] != "ร้านหนึ่ง" {
		t.Errorf("F5 หลังลบร้าน = %v", names)
	}
}

// favoriteNames อ่าน GET /api/me/favorites ซึ่งตอบเป็น array (e.do แปลงได้เฉพาะ object)
func favoriteNames(t *testing.T, e *env, user int64) []string {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/me/favorites", nil)
	req.Header.Set("X-Test-User", itoa(user))
	rec := httptest.NewRecorder()
	e.server().ServeHTTP(rec, req)
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("อ่านรายการถูกใจ: %v (%s)", err, rec.Body.String())
	}
	out := make([]string, len(list))
	for i, f := range list {
		out[i] = f["name"].(string)
	}
	return out
}

// V16 ราคาเฉลี่ยของร้าน: ใช้มัธยฐาน ไม่ใช่ค่าเฉลี่ย · ต้องมีคนระบุอย่างน้อย 3 คน
func TestRestaurantPriceLevelV16(t *testing.T) {
	e := newEnv(t)
	owner := e.user("owner")
	r := e.createViaAPI(owner, "ร้านทดสอบราคา")
	n := 0
	rate := func(price any) {
		n++
		e.exec(`INSERT INTO reviews (restaurant_id, user_id, rating_food, rating_ambience, rating_service, rating_value,
		                             price_level, comment)
		        VALUES ($1, $2, 4, 4, 4, 4, $3, 'ok')`, r, e.user(fmt.Sprint("p", n)), price)
	}
	level := func() (any, any) {
		_, _, d := e.do("GET", "/api/restaurants/"+itoa(r), 0, "")
		return d["price_level"], d["price_level_count"]
	}

	// ยังไม่มีใครระบุ → null
	if lv, c := level(); lv != nil || c != float64(0) {
		t.Fatalf("ไม่มีใครระบุ: %v %v", lv, c)
	}

	// 2 คนยังไม่พอที่จะประกาศเป็นราคาของร้าน
	rate(2)
	rate(2)
	if lv, c := level(); lv != nil || c != float64(2) {
		t.Errorf("2 คน: %v %v", lv, c)
	}

	// ครบ 3 คนแล้วจึงแสดง · [2,2,3] มัธยฐาน = 2
	rate(3)
	if lv, c := level(); lv != float64(2) || c != float64(3) {
		t.Errorf("3 คน: %v %v", lv, c)
	}

	// คนที่ 4 จัดเลี้ยงจ่ายหนัก ค่าเฉลี่ยจะถูกลากขึ้นแต่มัธยฐานต้องไม่ขยับ
	// [2,2,3,5] ค่าเฉลี่ย = 3.0 แต่มัธยฐาน (PERCENTILE_DISC) = 2
	rate(5)
	if lv, c := level(); lv != float64(2) || c != float64(4) {
		t.Errorf("มีค่าสุดโต่ง: %v %v (มัธยฐานต้องไม่ขยับ)", lv, c)
	}

	// รีวิวที่ไม่ระบุราคาต้องไม่ถูกนับ
	rate(nil)
	if _, c := level(); c != float64(4) {
		t.Errorf("นับเฉพาะคนที่ระบุ: %v", c)
	}
}
