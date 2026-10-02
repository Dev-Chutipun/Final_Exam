// คำสั่ง seed ใส่ข้อมูลตัวอย่างสำหรับเดโม โดยอิงเวลาปัจจุบัน (การจองจึงอยู่ในอนาคตเสมอ)
//
//	cd backend
//	go run ./cmd/seed          # ใส่ข้อมูลในฐานที่ยังว่าง
//	go run ./cmd/seed -reset   # ล้างข้อมูลทั้งหมด (ยกเว้นหมวดหมู่) แล้วใส่ใหม่
//
// ร้านตัวอย่างครอบคลุมกรณีสำคัญ: มีการจองล่วงหน้า (ลบไม่ได้), ไม่มีการจอง (ลบได้),
// เปิดข้ามเที่ยงคืน และพักรับจอง · รูปเป็นภาพไล่สีที่สร้างด้วยโค้ด (ไม่มีปัญหาลิขสิทธิ์)
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"time"

	_ "time/tzdata"

	"github.com/jackc/pgx/v5"

	"devpool/internal/booking"
	"devpool/internal/config"
	"devpool/internal/db"
)

// บัญชีทดสอบใน keycloak/realm-devpool.json (sub ตายตัว จึงผูกร้านกับเจ้าของได้ก่อน login ครั้งแรก)
var testUsers = []struct{ key, subject, name string }{
	{"alice", "11111111-1111-4111-8111-111111111111", "อลิซ ใจดี"},
	{"bob", "22222222-2222-4222-8222-222222222222", "บ๊อบ มั่นคง"},
	{"carol", "33333333-3333-4333-8333-333333333333", "แครอล สายใจ"},
}

// ผู้รีวิวตัวอย่างอื่น login ไม่ได้ (issuer = seed://local)
var seedReviewers = []struct{ key, name string }{
	{"danai", "ดนัย ว."},
	{"malee", "มาลี ส."},
	{"pim", "พิม ร."},
	{"krit", "กฤต ส."},
}

type restaurant struct {
	key, owner, name, description string
	capacity                      int
	open, close                   string
	cancelWindow, minLead         int
	accepting                     bool
	categories                    []int // id ใน migrations/0002_categories.sql
	images                        int
}

var restaurants = []restaurant{
	{"klong", "alice", "ครัวริมคลอง", "ร้านอาหารไทยริมคลอง บรรยากาศร่มรื่น เมนูเด่นคือปลากะพงทอดน้ำปลาและต้มยำทะเลน้ำข้น มีโซนนั่งริมน้ำและที่จอดรถหน้าร้าน ที่ตั้ง: ซอยริมคลอง 12",
		10, "10:00", "22:00", 30, 120, true, []int{1, 9}, 3},
	{"sushi", "alice", "ซูชิบาร์ ท่าเรือ", "ซูชิและซาชิมิจากปลาสดทุกเช้า นั่งเคาน์เตอร์ได้ 12 ที่ ที่ตั้ง: ท่าเรือเก่า ชั้น 2",
		12, "11:30", "21:30", 60, 180, true, []int{4}, 1},
	{"pho", "bob", "ฟอเฮ้าส์ ซอยสาม", "ก๋วยเตี๋ยวเวียดนามน้ำซุปเคี่ยวกระดูก 12 ชั่วโมง ปอเปี๊ยะสดและขนมปังเวียดนาม ที่ตั้ง: ซอยสาม ใกล้ตลาดเช้า",
		24, "10:30", "20:30", 30, 120, true, []int{3}, 2},
	{"isan", "carol", "เฮือนอีสานบ้านเฮา", "ส้มตำ ลาบ ไก่ย่าง และแจ่วฮ้อนสูตรบ้าน นั่งแคร่ไม้ใต้ร่มมะม่วง ที่ตั้ง: ถนนมิตรภาพ ซอย 4",
		30, "11:00", "21:00", 30, 120, true, []int{2}, 2},
	{"steak", "bob", "สเต็กลุงพล", "สเต็กเนื้อย่างเตาถ่านและเบอร์เกอร์ เปิดถึงตีสอง เหมาะกับมื้อดึก ที่ตั้ง: ถนนคนเดินฝั่งตะวันออก",
		36, "17:00", "02:00", 30, 120, true, []int{8}, 1},
	{"tea", "carol", "ใบชา คาเฟ่", "ชาไทย ชาเขียว และเค้กโฮมเมด มุมนั่งทำงานเงียบสบาย ตอนนี้ปิดปรับปรุงร้าน พักรับจองชั่วคราว",
		16, "08:00", "18:00", 30, 60, false, []int{11, 12}, 1},
}

// สีไล่ของภาพปกตาม 05 §5 (ตามหมวดแรกของร้าน)
var coverStops = map[int][3]string{
	1: {"#FFE5D8", "#FFC7B4", "#FAACA6"}, 9: {"#FFE5D8", "#FFC7B4", "#FAACA6"}, 5: {"#FFE5D8", "#FFC7B4", "#FAACA6"},
	3: {"#D9F5E8", "#AEE6D2", "#92D6CB"}, 10: {"#D9F5E8", "#AEE6D2", "#92D6CB"},
	2: {"#FFF5D1", "#FFE0A3", "#FAC88A"}, 7: {"#FFF5D1", "#FFE0A3", "#FAC88A"},
	11: {"#F8EBE3", "#EBD3C6", "#DCBBAE"}, 12: {"#F8EBE3", "#EBD3C6", "#DCBBAE"},
	8: {"#EEE4FA", "#D8C5EF", "#C1AAE0"}, 6: {"#EEE4FA", "#D8C5EF", "#C1AAE0"},
	4: {"#E2EFFF", "#C4DBFA", "#ADC6F2"},
}

func main() {
	reset := flag.Bool("reset", false, "ล้างข้อมูลทั้งหมด (ยกเว้นหมวดหมู่) ก่อนใส่ข้อมูลตัวอย่าง")
	flag.Parse()
	if err := run(*reset); err != nil {
		log.Fatal("seed: ", err)
	}
}

func run(reset bool) error {
	if err := config.Load(); err != nil {
		return err
	}
	dbURL, err := config.Require("DATABASE_URL")
	if err != nil {
		return err
	}
	issuer, err := config.Require("OIDC_ISSUER")
	if err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, dbURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	now := time.Now().In(booking.Bangkok)
	return db.WithTx(ctx, pool, func(tx pgx.Tx) error {
		if reset {
			if _, err := tx.Exec(ctx, `TRUNCATE users, sessions, restaurants, restaurant_categories,
				restaurant_images, bookings, reviews RESTART IDENTITY CASCADE`); err != nil {
				return err
			}
		} else {
			var n int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM restaurants`).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				return fmt.Errorf("มีร้านในฐานข้อมูลอยู่แล้ว %d ร้าน ใช้ -reset ถ้าต้องการล้างแล้วใส่ใหม่", n)
			}
		}
		s := &seeder{ctx: ctx, tx: tx, now: now, users: map[string]int64{}, rest: map[string]int64{}}
		return s.all(issuer)
	})
}

type seeder struct {
	ctx   context.Context
	tx    pgx.Tx
	now   time.Time
	users map[string]int64
	rest  map[string]int64
}

func (s *seeder) all(issuer string) error {
	for _, u := range testUsers {
		if err := s.user(u.key, issuer, u.subject, u.name); err != nil {
			return err
		}
	}
	for _, u := range seedReviewers {
		if err := s.user(u.key, "seed://local", u.key, u.name); err != nil {
			return err
		}
	}
	for _, r := range restaurants {
		if err := s.restaurant(r); err != nil {
			return err
		}
	}
	if err := s.bookings(); err != nil {
		return err
	}
	if err := s.reviews(); err != nil {
		return err
	}
	fmt.Printf("seed เสร็จ: ผู้ใช้ %d คน, ร้าน %d ร้าน (อิงเวลา %s)\n",
		len(s.users), len(s.rest), s.now.Format("2006-01-02 15:04"))
	return nil
}

func (s *seeder) user(key, issuer, subject, name string) error {
	var id int64
	err := s.tx.QueryRow(s.ctx, `
		INSERT INTO users (issuer, subject, display_name) VALUES ($1, $2, $3)
		ON CONFLICT (issuer, subject) DO UPDATE SET display_name = EXCLUDED.display_name
		RETURNING id`, issuer, subject, name).Scan(&id)
	s.users[key] = id
	return err
}

func (s *seeder) restaurant(r restaurant) error {
	var id int64
	err := s.tx.QueryRow(s.ctx, `
		INSERT INTO restaurants (owner_id, name, description, capacity, open_time, close_time,
		                         cancel_window_minutes, min_lead_minutes, accepting_bookings)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		s.users[r.owner], r.name, r.description, r.capacity, r.open, r.close,
		r.cancelWindow, r.minLead, r.accepting).Scan(&id)
	if err != nil {
		return fmt.Errorf("%s: %w", r.name, err)
	}
	s.rest[r.key] = id
	for _, c := range r.categories {
		if _, err := s.tx.Exec(s.ctx, `INSERT INTO restaurant_categories (restaurant_id, category_id) VALUES ($1, $2)`, id, c); err != nil {
			return err
		}
	}
	stops := coverStops[r.categories[0]]
	for i := range r.images {
		// รูปถัดไปสลับทิศการไล่สี เพื่อให้แต่ละรูปต่างกันเล็กน้อย
		data, err := gradientPNG(stops, i%2 == 1)
		if err != nil {
			return err
		}
		if _, err := s.tx.Exec(s.ctx, `INSERT INTO restaurant_images (restaurant_id, content_type, data) VALUES ($1, 'image/png', $2)`, id, data); err != nil {
			return err
		}
	}
	return nil
}

// at คือเวลา hh:mm ของวันที่ (วันนี้ + days) ตามเวลา Bangkok
func (s *seeder) at(days, hh, mm int) time.Time {
	y, m, d := s.now.Date()
	return time.Date(y, m, d+days, hh, mm, 0, 0, booking.Bangkok)
}

// book ใส่การจองตรงลงฐานข้อมูล deadline คำนวณจากนโยบายของร้านแบบเดียวกับตอนจองจริง
func (s *seeder) book(restKey, userKey string, size int, start time.Time, minutes int, cancelled bool) error {
	cancelWindow := 30
	for _, r := range restaurants {
		if r.key == restKey {
			cancelWindow = r.cancelWindow
		}
	}
	var cancelledAt any
	status := "active"
	if cancelled {
		status, cancelledAt = "cancelled", start.Add(-48*time.Hour)
	}
	_, err := s.tx.Exec(s.ctx, `
		INSERT INTO bookings (restaurant_id, user_id, party_size, start_at, end_at, cancel_deadline, status, cancelled_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		s.rest[restKey], s.users[userKey], size, start, start.Add(time.Duration(minutes)*time.Minute),
		booking.CancelDeadline(start, cancelWindow), status, cancelledAt)
	return err
}

func (s *seeder) bookings() error {
	steps := []error{
		// ครัวริมคลอง: มีการจองล่วงหน้า (เดโมว่าลบร้านไม่ได้) และช่วง 12:00–13:00 ของพรุ่งนี้เหลือน้อย
		s.book("klong", "bob", 7, s.at(1, 12, 0), 60, false),
		s.book("klong", "carol", 2, s.at(1, 12, 0), 60, false),
		s.book("klong", "danai", 4, s.at(1, 18, 0), 90, false),
		s.book("klong", "bob", 2, s.at(5, 19, 0), 60, false),
		s.book("klong", "malee", 2, s.at(1, 19, 0), 60, true), // ยกเลิกแล้ว
		// ประวัติของลูกค้า (แท็บ "ผ่านไปแล้ว" และ "ยกเลิกแล้ว")
		s.book("klong", "bob", 3, s.at(-3, 12, 0), 60, false),
		s.book("isan", "bob", 4, s.at(-6, 18, 0), 90, false),
		s.book("pho", "carol", 2, s.at(-2, 12, 0), 60, true),
		// เฮือนอีสาน: carol เป็นเจ้าของ bob และ alice เป็นลูกค้า
		s.book("isan", "bob", 4, s.at(3, 18, 0), 90, false),
		s.book("isan", "alice", 6, s.at(2, 12, 0), 60, false),
		// สเต็กลุงพล ร้านข้ามเที่ยงคืน: จองช่วงหลังเที่ยงคืน (วันทำการก่อนหน้า)
		s.book("steak", "carol", 4, s.at(3, 0, 30), 90, false),
		s.book("steak", "alice", 2, s.at(2, 22, 30), 120, false),
		// ใบชา คาเฟ่ พักรับจอง แต่ยังมีการจองเดิมค้างอยู่ (ลูกค้ายังยกเลิกได้)
		s.book("tea", "bob", 2, s.at(4, 14, 0), 60, false),
		// ซูชิบาร์ ท่าเรือ: ไม่มีการจองเลย (เดโมว่าลบร้านได้)
	}
	for _, err := range steps {
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *seeder) reviews() error {
	type rv struct {
		rest, user string
		rating     int
		daysAgo    int
		comment    string
	}
	list := []rv{
		{"klong", "carol", 5, 29, "ปลากะพงทอดน้ำปลากรอบมาก นั่งริมคลองสบาย จองล่วงหน้าแล้วได้โต๊ะตามเวลาพอดี"},
		{"klong", "danai", 4, 5, "ต้มยำทะเลรสจัดจ้านกำลังดี ช่วงเย็นคนค่อนข้างเยอะ แนะนำให้จองก่อน"},
		{"klong", "malee", 5, 12, "บริการเร็ว พนักงานสุภาพ มีเก้าอี้เด็กให้"},
		{"klong", "pim", 4, 20, "อาหารอร่อย ราคาเหมาะสม ที่จอดรถมีจำกัด"},
		// รีวิวเดียว 5 ดาว: ใช้เดโมว่า Bayesian ไม่ให้ร้านนี้ขึ้นอันดับหนึ่งเหนือร้านที่มีหลายรีวิว
		{"sushi", "carol", 5, 3, "ปลาสดมาก ข้าวซูชิกำลังดี นั่งเคาน์เตอร์ได้คุยกับเชฟ"},
		{"pho", "alice", 5, 8, "น้ำซุปหอมกลมกล่อม เส้นเหนียวนุ่ม"},
		{"pho", "danai", 5, 15, "ปอเปี๊ยะสดผักกรอบ น้ำจิ้มอร่อย"},
		{"pho", "malee", 4, 30, "ร้านเล็กแต่สะอาด รอไม่นาน"},
		{"pho", "krit", 5, 2, "มากินซ้ำทุกอาทิตย์"},
		{"isan", "alice", 4, 6, "ส้มตำปูปลาร้ารสเด็ด ไก่ย่างหนังกรอบ"},
		{"isan", "danai", 5, 10, "ลาบเป็ดอร่อยที่สุดในย่านนี้"},
		{"isan", "krit", 3, 18, "อร่อยแต่ช่วงเที่ยงคนเยอะ รอนานหน่อย"},
		{"steak", "carol", 4, 4, "สเต็กนุ่ม ย่างได้ระดับที่สั่ง เปิดดึกสะดวกมาก"},
		{"steak", "pim", 4, 9, "เบอร์เกอร์ชิ้นใหญ่ คุ้มราคา"},
		{"tea", "alice", 5, 25, "ชาไทยหอมเข้ม เค้กมะพร้าวอร่อย"},
	}
	for _, r := range list {
		created := s.now.Add(-time.Duration(r.daysAgo) * 24 * time.Hour)
		if _, err := s.tx.Exec(s.ctx, `
			INSERT INTO reviews (restaurant_id, user_id, rating, comment, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $5)`,
			s.rest[r.rest], s.users[r.user], r.rating, r.comment, created); err != nil {
			return fmt.Errorf("review %s/%s: %w", r.rest, r.user, err)
		}
	}
	return nil
}

// gradientPNG สร้างภาพไล่สี 135° สามจุดขนาด 640×400
func gradientPNG(stops [3]string, flip bool) ([]byte, error) {
	const w, h = 640, 400
	c := [3]color.RGBA{hexColor(stops[0]), hexColor(stops[1]), hexColor(stops[2])}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			t := (float64(x)/w + float64(y)/h) / 2 // 0 ที่มุมซ้ายบน → 1 ที่มุมขวาล่าง
			if flip {
				t = 1 - t
			}
			img.Set(x, y, mix(c, t))
		}
	}
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	return buf.Bytes(), err
}

func mix(c [3]color.RGBA, t float64) color.RGBA {
	a, b, u := c[0], c[1], t*2
	if t > 0.5 {
		a, b, u = c[1], c[2], (t-0.5)*2
	}
	lerp := func(p, q uint8) uint8 { return uint8(float64(p) + (float64(q)-float64(p))*u) }
	return color.RGBA{lerp(a.R, b.R), lerp(a.G, b.G), lerp(a.B, b.B), 255}
}

func hexColor(s string) color.RGBA {
	var r, g, b uint8
	fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b)
	return color.RGBA{r, g, b, 255}
}
