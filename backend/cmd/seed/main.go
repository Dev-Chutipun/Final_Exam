// คำสั่ง seed ใส่ข้อมูลตัวอย่างสำหรับเดโม โดยอิงเวลาปัจจุบัน (การจองจึงอยู่ในอนาคตเสมอ)
//
//	cd backend
//	go run ./cmd/seed          # ใส่ข้อมูลในฐานที่ยังว่าง
//	go run ./cmd/seed -reset   # ล้างข้อมูลทั้งหมด (ยกเว้นหมวดหมู่) แล้วใส่ใหม่
//
// ร้านตัวอย่าง 20 ร้านครอบคลุมกรณีสำคัญ: มีการจองล่วงหน้า (ลบไม่ได้), ไม่มีการจอง (ลบได้),
// เปิดข้ามเที่ยงคืน, พักรับจอง และร้านใหม่ที่ยังไม่มีรีวิว
//
// รูปของแต่ละร้านอ่านจากโฟลเดอร์ seed-images/<ลำดับ-key>/cover (รูปปก 1 รูป) และ /menu (รูปเมนู 2 รูป)
// ช่องที่ไม่มีไฟล์ใช้ภาพไล่สีที่สร้างด้วยโค้ดแทน · ไม่มีตารางเมนู รายการเมนูจึงเขียนไว้ในรายละเอียดร้าน
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "time/tzdata"

	"github.com/jackc/pgx/v5"

	"devpool/internal/booking"
	"devpool/internal/config"
	"devpool/internal/db"
	"devpool/internal/httpx"
)

// บัญชีทดสอบใน keycloak/realm-devpool.json (sub ตายตัว จึงผูกร้านกับเจ้าของได้ก่อน login ครั้งแรก)
// ชื่อต้องตรงกับ firstName + lastName ใน realm เพราะ login แล้วชื่อจะถูกอัปเดตตาม Keycloak
var testUsers = []struct{ key, subject, name string }{
	{"chutipun", "11111111-1111-4111-8111-111111111111", "Chutipun Homsaard"}, // ลูกค้าอย่างเดียว
	{"ownertest", "22222222-2222-4222-8222-222222222222", "Owner Test"},       // เจ้าของร้านอย่างเดียว
	{"usertest", "33333333-3333-4333-8333-333333333333", "User Test"},         // ทั้งเจ้าของร้านและลูกค้า
	{"somchai", "44444444-4444-4444-8444-444444444444", "สมชาย ใจกว้าง"},
	{"nida", "55555555-5555-4555-8555-555555555555", "นิดา รักดี"},
	{"arthit", "66666666-6666-4666-8666-666666666666", "อาทิตย์ แสงทอง"},
	{"somsri", "77777777-7777-4777-8777-777777777777", "สมศรี มีสุข"},
	{"kanya", "88888888-8888-4888-8888-888888888888", "กันยา พากเพียร"},
}

// ผู้รีวิวตัวอย่างอื่น login ไม่ได้ (issuer = seed://local)
var seedReviewers = []struct{ key, name string }{
	{"danai", "ดนัย ว."},
	{"malee", "มาลี ส."},
	{"pim", "พิม ร."},
	{"krit", "กฤต ส."},
	{"ploy", "พลอย ท."},
	{"ton", "ต้น ก."},
	{"june", "จูน ม."},
}

type restaurant struct {
	key, owner, name      string
	about, place          string
	menu                  []string // สองรายการแรกคือรูปในโฟลเดอร์ menu/
	capacity              int
	open, close           string
	cancelWindow, minLead int
	accepting             bool
	categories            []int   // id ใน migrations/0002_categories.sql
	lat, lng              float64 // พิกัดสมมติแถวกรุงเทพฯ สำหรับเดโมแผนที่ (ไม่ใช่ที่ตั้งจริง)
}

var restaurants = []restaurant{
	// ── ownertest ──
	{"klong", "ownertest", "ครัวริมคลอง",
		"ร้านอาหารไทยริมคลอง บรรยากาศร่มรื่น มีโซนนั่งริมน้ำและที่จอดรถหน้าร้าน", "ซอยริมคลอง 12",
		[]string{"ปลากะพงทอดน้ำปลา", "ต้มยำทะเลน้ำข้น", "ผัดผักบุ้งไฟแดง", "แกงเขียวหวานไก่"},
		10, "10:00", "22:00", 30, 120, true, []int{1, 9}, 13.7730, 100.5102},
	{"sushi", "ownertest", "ซูชิบาร์ ท่าเรือ",
		"ซูชิและซาชิมิจากปลาสดทุกเช้า นั่งเคาน์เตอร์ได้ 12 ที่ คุยกับเชฟได้ตลอดมื้อ", "ท่าเรือเก่า ชั้น 2",
		[]string{"ซาชิมิแซลมอน", "ซูชิรวม 12 คำ", "ข้าวหน้าปลาไหล", "ซุปมิโซะ"},
		12, "11:30", "21:30", 60, 180, true, []int{4}, 13.7257, 100.5140},
	{"steak", "ownertest", "สเต็กลุงพล",
		"สเต็กเนื้อย่างเตาถ่านและเบอร์เกอร์ เปิดถึงตีสอง เหมาะกับมื้อดึก", "ถนนคนเดินฝั่งตะวันออก",
		[]string{"ริบอายสเต็กซอสพริกไทยดำ", "เบอร์เกอร์เนื้อชีส", "สลัดซีซาร์", "เฟรนช์ฟรายส์ทรัฟเฟิล"},
		36, "17:00", "02:00", 30, 120, true, []int{8}, 13.7465, 100.5341},
	{"tea", "ownertest", "ใบชา คาเฟ่",
		"ชาไทย ชาเขียว และเค้กโฮมเมด มุมนั่งทำงานเงียบสบาย ตอนนี้ปิดปรับปรุงร้าน พักรับจองชั่วคราว", "ถนนนิมมาน ซอย 7",
		[]string{"ชาไทยเย็น", "เค้กมะพร้าวอ่อน", "มัทฉะลาเต้", "ครัมเบิลแอปเปิล"},
		16, "08:00", "18:00", 30, 60, false, []int{11, 12}, 13.7301, 100.5695},
	{"dimsum", "ownertest", "ติ่มซำเยาวราช",
		"ติ่มซำนึ่งร้อนจากเข่งทุกเช้า ชาจีนเติมไม่อั้น เหมาะกับมื้อเช้าวันหยุดกับครอบครัว", "ถนนเยาวราช ใกล้วงเวียนโอเดียน",
		[]string{"ฮะเก๋ากุ้ง", "ขนมจีบหมูปู", "ซาลาเปาลาวา", "โจ๊กหมูสับ"},
		40, "07:00", "14:00", 30, 120, true, []int{5}, 13.7398, 100.5100},
	{"bbqk", "ownertest", "โซลบาร์บีคิว",
		"ปิ้งย่างเกาหลีบนเตาถ่าน มีพนักงานย่างให้ทุกโต๊ะ เครื่องเคียงเติมได้", "สยามสแควร์ ซอย 3",
		[]string{"หมูสามชั้นหมักซอสโคชูจัง", "เนื้อวัวคาลบี", "ต๊อกบกกี", "กิมจิจิเก"},
		28, "16:00", "23:00", 30, 120, true, []int{6, 8}, 13.7447, 100.5340},
	// ── usertest (เป็นทั้งเจ้าของและลูกค้า) ──
	{"pho", "usertest", "ฟอเฮ้าส์ ซอยสาม",
		"ก๋วยเตี๋ยวเวียดนามน้ำซุปเคี่ยวกระดูก 12 ชั่วโมง ร้านเล็กแต่สะอาด รอไม่นาน", "ซอยสาม ใกล้ตลาดเช้า",
		[]string{"เฝอเนื้อสด", "ปอเปี๊ยะสด", "ขนมปังเวียดนาม (บั๋นหมี่)", "กาแฟเวียดนามนมข้น"},
		24, "10:30", "20:30", 30, 120, true, []int{3}, 13.7652, 100.5380},
	{"pasta", "usertest", "พาสต้าบ้านนา",
		"พาสต้าเส้นสดทำเองทุกวัน ใช้ผักจากฟาร์มของร้าน บรรยากาศบ้านไม้กลางทุ่ง", "ถนนเลียบทุ่งนา กม. 4",
		[]string{"คาโบนาราไข่แดง", "สปาเกตตีกุ้งซอสมะเขือเทศ", "ลาซานญาเนื้อ", "ทีรามิสุ"},
		20, "11:00", "22:00", 30, 120, true, []int{7}, 13.8204, 100.5601},
	// ── somchai ──
	{"isan", "somchai", "เฮือนอีสานบ้านเฮา",
		"อาหารอีสานสูตรบ้าน นั่งแคร่ไม้ใต้ร่มมะม่วง", "ถนนมิตรภาพ ซอย 4",
		[]string{"ส้มตำปูปลาร้า", "ลาบเป็ด", "ไก่ย่างหนังกรอบ", "แจ่วฮ้อน"},
		30, "11:00", "21:00", 30, 120, true, []int{2}, 13.8051, 100.5533},
	{"tomyum", "somchai", "ต้มยำนายหนวด",
		"ร้านต้มยำเจ้าเก่ากว่า 30 ปี น้ำซุปเคี่ยวสดทุกหม้อ เปิดแค่ช่วงกลางวัน", "ตลาดพลู ซอย 2",
		[]string{"ต้มยำกุ้งน้ำใส", "ต้มยำกุ้งน้ำข้น", "ไข่เจียวปู", "ข้าวผัดกุ้ง"},
		18, "09:00", "16:00", 30, 120, true, []int{1}, 13.7192, 100.4761},
	{"seafood", "somchai", "ทะเลเผาริมหาด",
		"บุฟเฟต์ทะเลเผาริมหาด กุ้ง หอย ปู ปลาหมึก สดจากเรือประมงทุกวัน ยกเลิกได้ถึง 2 ชั่วโมงก่อนเวลาจอง", "หาดบางแสน ทางเข้าซอย 9",
		[]string{"กุ้งแม่น้ำเผา", "หอยแมลงภู่อบหม้อดิน", "ปูม้านึ่ง", "ปลาหมึกย่าง"},
		60, "16:00", "23:00", 120, 240, true, []int{9, 10}, 13.5402, 100.4203},
	{"khaosoi", "somchai", "ข้าวซอยเชียงใหม่",
		"อาหารเหนือรสดั้งเดิม น้ำพริกตำสดทุกเช้า", "ถนนเจริญราษฎร์ ใกล้สะพานนวรัฐ",
		[]string{"ข้าวซอยไก่", "ไส้อั่วย่าง", "น้ำพริกหนุ่มผักลวก", "แกงฮังเล"},
		22, "08:00", "15:00", 30, 120, true, []int{1}, 13.7822, 100.5441},
	// ── nida ──
	{"ramen", "nida", "ราเมงมิโซะ",
		"ราเมงน้ำซุปมิโซะสไตล์ซัปโปโร เส้นสดทำเองในร้าน", "ซอยทองหล่อ 13",
		[]string{"มิโซะราเมงหมูชาชู", "เกี๊ยวซ่า", "ทงคตสึราเมง", "ไข่ยางมะตูมดองโชยุ"},
		14, "11:00", "21:00", 30, 120, true, []int{4}, 13.7343, 100.5822},
	{"bingsu", "nida", "บิงซูหิมะ",
		"บิงซูน้ำแข็งนมเนื้อเนียนแบบเกาหลี ท็อปปิงผลไม้ตามฤดูกาล", "ถนนบรรทัดทอง",
		[]string{"บิงซูสตรอว์เบอร์รี", "บิงซูถั่วแดงต๊อก", "บิงซูมะม่วง", "โทสต์เนยน้ำผึ้ง"},
		20, "12:00", "22:00", 30, 60, true, []int{12, 6}, 13.7421, 100.5232},
	{"hotpot", "nida", "ชาบูหม้อไฟ",
		"บุฟเฟต์ชาบูและหม่าล่า น้ำซุปให้เลือก 4 แบบ จำกัดเวลา 2 ชั่วโมง ยกเลิกได้ถึง 1 ชั่วโมงก่อนเวลาจอง", "ศูนย์การค้าเซ็นทรัล ชั้น 5",
		[]string{"น้ำซุปหม่าล่า", "เนื้อวัวสไลซ์", "ลูกชิ้นกุ้งทำเอง", "ไอศกรีมชาเขียว"},
		48, "11:00", "22:00", 60, 120, true, []int{10, 5}, 13.7472, 100.5392},
	{"bakery", "nida", "ขนมปังอบเช้า",
		"เบเกอรี่อบสดทุกเช้า เนยแท้จากฝรั่งเศส กาแฟคั่วเอง", "ถนนพระอาทิตย์",
		[]string{"ครัวซองต์เนยสด", "ขนมปังซาวร์โดว์", "พายไก่", "ลาเต้ร้อน"},
		12, "07:00", "15:00", 30, 60, true, []int{12, 11}, 13.7623, 100.4954},
	// ── arthit ──
	{"pizza", "arthit", "พิซซ่าเตาฟืน",
		"พิซซ่าแป้งหมัก 48 ชั่วโมง อบในเตาฟืนอิฐ 400 องศา", "ถนนสุขุมวิท ซอย 31",
		[]string{"มาร์เกริตา", "พิซซ่าแฮมเห็ดทรัฟเฟิล", "สลัดร็อกเก็ต", "พานาคอตตา"},
		26, "11:30", "22:30", 30, 120, true, []int{7}, 13.7371, 100.5693},
	{"gaiyang", "arthit", "ไก่ย่างวิเชียร",
		"ไก่ย่างเตาถ่านสูตรวิเชียรบุรี ส้มตำครบทุกแบบ", "ถนนพหลโยธิน ใกล้ตลาดนัด",
		[]string{"ไก่ย่างวิเชียร", "ส้มตำไทย", "คอหมูย่าง", "ข้าวเหนียวมะม่วง"},
		34, "10:00", "20:00", 30, 120, true, []int{2, 1}, 13.8103, 100.5624},
	{"jok", "arthit", "ข้าวต้มโต้รุ่ง",
		"ข้าวต้มกุ๊ยและกับข้าวจีน เปิดหกโมงเย็นถึงตีสาม สำหรับคนเลิกงานดึก", "ถนนเยาวราช ปากซอยผดุงด้าว",
		[]string{"ข้าวต้มปลา", "หมูแดดเดียว", "ผัดคะน้าปลาเค็ม", "ไข่เจียวหมูสับ"},
		40, "18:00", "03:00", 30, 120, true, []int{1, 5}, 13.7405, 100.5085},
	{"salad", "arthit", "สลัดผักสวนครัว",
		"ร้านใหม่ เปิดได้สัปดาห์เดียว สลัดและโบวล์สุขภาพจากผักออร์แกนิก", "ถนนอารีย์ ซอย 1",
		[]string{"สลัดอกไก่ย่าง", "โบวล์แซลมอนควินัว", "สมูทตี้ผักเขียว", "โยเกิร์ตกราโนลา"},
		18, "09:00", "19:00", 30, 120, true, []int{11}, 13.7791, 100.5443},
}

// description รวมคำอธิบายกับรายการเมนูเป็นข้อความเดียว (หน้าร้านแสดงแบบรักษาการขึ้นบรรทัด)
// ที่อยู่ไม่อยู่ในนี้แล้ว ไปอยู่ในคอลัมน์ address เพื่อให้แสดงคู่กับแผนที่ได้
func (r restaurant) description() string {
	var b strings.Builder
	b.WriteString(r.about)
	b.WriteString("\n\nเมนูแนะนำ")
	for _, m := range r.menu {
		b.WriteString("\n• " + m)
	}
	return b.String()
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
	images := flag.String("images", filepath.Join("..", "seed-images"), "โฟลเดอร์รูปของร้าน (ไม่มีก็ได้ จะใช้ภาพไล่สีแทน)")
	flag.Parse()
	if err := run(*reset, *images); err != nil {
		log.Fatal("seed: ", err)
	}
}

func run(reset bool, imageDir string) error {
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
			// ไม่ใช้ RESTART IDENTITY: /api/restaurants/{id}/images/{imageId} ส่ง Cache-Control immutable
			// ถ้า id เริ่มที่ 1 ใหม่ทุกครั้ง URL เดิมจะชี้รูปใหม่ เบราว์เซอร์ที่เคยเข้าจะเห็นรูปเก่าค้างอยู่
			if _, err := tx.Exec(ctx, `TRUNCATE users, sessions, restaurants, restaurant_categories,
				restaurant_images, bookings, reviews CASCADE`); err != nil {
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
		s := &seeder{ctx: ctx, tx: tx, now: now, imageDir: imageDir, users: map[string]int64{}, rest: map[string]int64{}}
		return s.all(issuer)
	})
}

type seeder struct {
	ctx      context.Context
	tx       pgx.Tx
	now      time.Time
	imageDir string
	users    map[string]int64
	rest     map[string]int64
	fromDisk int // จำนวนรูปที่อ่านจากโฟลเดอร์ (ที่เหลือเป็นภาพไล่สี)
	total    int
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
	for i, r := range restaurants {
		if err := s.restaurant(i, r); err != nil {
			return err
		}
	}
	if err := s.bookings(); err != nil {
		return err
	}
	if err := s.reviews(); err != nil {
		return err
	}
	fmt.Printf("seed เสร็จ: ผู้ใช้ %d คน, ร้าน %d ร้าน, รูปจากโฟลเดอร์ %d/%d รูป (อิงเวลา %s)\n",
		len(s.users), len(s.rest), s.fromDisk, s.total, s.now.Format("2006-01-02 15:04"))
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

// folder คือชื่อโฟลเดอร์รูปของร้านลำดับที่ i เช่น 01-klong
func folder(i int, key string) string {
	return fmt.Sprintf("%02d-%s", i+1, key)
}

func (s *seeder) restaurant(i int, r restaurant) error {
	var id int64
	err := s.tx.QueryRow(s.ctx, `
		INSERT INTO restaurants (owner_id, name, description, capacity, open_time, close_time,
		                         cancel_window_minutes, min_lead_minutes, accepting_bookings, address, latitude, longitude)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING id`,
		s.users[r.owner], r.name, r.description(), r.capacity, r.open, r.close,
		r.cancelWindow, r.minLead, r.accepting, r.place, r.lat, r.lng).Scan(&id)
	if err != nil {
		return fmt.Errorf("%s: %w", r.name, err)
	}
	s.rest[r.key] = id
	for _, c := range r.categories {
		if _, err := s.tx.Exec(s.ctx, `INSERT INTO restaurant_categories (restaurant_id, category_id) VALUES ($1, $2)`, id, c); err != nil {
			return err
		}
	}
	imgs, err := s.images(filepath.Join(s.imageDir, folder(i, r.key)), coverStops[r.categories[0]])
	if err != nil {
		return err
	}
	// ใส่รูปปกก่อน รูปปกคือรูปที่ id น้อยที่สุดของร้าน (03 §4)
	for _, img := range imgs {
		if _, err := s.tx.Exec(s.ctx, `INSERT INTO restaurant_images (restaurant_id, content_type, data) VALUES ($1, $2, $3)`,
			id, img.ContentType, img.Data); err != nil {
			return err
		}
	}
	return nil
}

// images คืนรูปของร้าน 3 รูป: รูปปก 1 รูปจาก cover/ แล้วรูปเมนู 2 รูปจาก menu/
// ช่องที่ไม่มีไฟล์ใช้ภาพไล่สีแทน (สลับทิศการไล่สีให้แต่ละรูปต่างกันเล็กน้อย)
func (s *seeder) images(dir string, stops [3]string) ([]httpx.Image, error) {
	var out []httpx.Image
	for _, part := range []struct {
		sub string
		n   int
	}{{"cover", 1}, {"menu", 2}} {
		files, err := readImages(filepath.Join(dir, part.sub), part.n)
		if err != nil {
			return nil, err
		}
		s.fromDisk += len(files)
		out = append(out, files...)
		for range part.n - len(files) {
			data, err := gradientPNG(stops, len(out)%2 == 1)
			if err != nil {
				return nil, err
			}
			out = append(out, httpx.Image{ContentType: "image/png", Data: data})
		}
	}
	s.total += len(out)
	return out, nil
}

// readImages อ่านรูปไม่เกิน max รูปจาก dir เรียงตามชื่อไฟล์ ข้ามไฟล์ที่ขึ้นต้นด้วยจุด (เช่น .gitkeep)
// ไม่มีโฟลเดอร์ → ไม่มีรูป · ไฟล์ที่ไม่ผ่านกฎเดียวกับการอัปโหลด (≤ 2 MB, JPEG/PNG/WebP) → error
func readImages(dir string, max int) ([]httpx.Image, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []httpx.Image
	for _, e := range entries {
		if len(out) == max {
			break
		}
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		if info.Size() > httpx.MaxImageSize {
			return nil, fmt.Errorf("%s ใหญ่เกิน 2 MB", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		img, err := httpx.CheckImage(path, data)
		if err != nil {
			return nil, err
		}
		out = append(out, img)
	}
	return out, nil
}

// at คือเวลา hh:mm ของวันที่ (วันนี้ + days) ตามเวลา Bangkok
func (s *seeder) at(days, hh, mm int) time.Time {
	y, m, d := s.now.Date()
	return time.Date(y, m, d+days, hh, mm, 0, 0, booking.Bangkok)
}

// book ใส่การจองตรงลงฐานข้อมูล deadline คำนวณจากนโยบายของร้านแบบเดียวกับตอนจองจริง
// ข้อมูลในรายการด้านล่างจัดให้ผ่านกติกาเองแล้ว: อยู่ในเวลาเปิด ที่นั่งพอ และเจ้าของไม่จองร้านตัวเอง
func (s *seeder) book(restKey, userKey string, size int, start time.Time, minutes int, cancelled bool) error {
	cancelWindow := 30
	for _, r := range restaurants {
		if r.key == restKey {
			cancelWindow = r.cancelWindow
			if r.owner == userKey {
				return fmt.Errorf("seed ผิด: %s เป็นเจ้าของ %s จองร้านตัวเองไม่ได้", userKey, restKey)
			}
		}
	}
	var cancelledAt any
	status := "active"
	if cancelled {
		// ยกเลิกไปแล้ว 2 วันก่อนเวลาจอง แต่ต้องไม่อยู่ในอนาคต
		at := start.Add(-48 * time.Hour)
		if at.After(s.now) {
			at = s.now.Add(-time.Hour)
		}
		status, cancelledAt = "cancelled", at
	}
	_, err := s.tx.Exec(s.ctx, `
		INSERT INTO bookings (restaurant_id, user_id, party_size, start_at, end_at, cancel_deadline, status, cancelled_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		s.rest[restKey], s.users[userKey], size, start, start.Add(time.Duration(minutes)*time.Minute),
		booking.CancelDeadline(start, cancelWindow), status, cancelledAt)
	if err != nil {
		return fmt.Errorf("booking %s/%s: %w", restKey, userKey, err)
	}
	return nil
}

func (s *seeder) bookings() error {
	steps := []error{
		// ครัวริมคลอง (ownertest): มีการจองล่วงหน้า (เดโมว่าลบร้านไม่ได้)
		// ช่วง 12:00–13:00 ของพรุ่งนี้ใช้ไป 9 จาก 10 ที่ (เดโม Limited Seats Left)
		s.book("klong", "usertest", 7, s.at(1, 12, 0), 60, false),
		s.book("klong", "chutipun", 2, s.at(1, 12, 0), 60, false),
		s.book("klong", "danai", 4, s.at(1, 18, 0), 90, false),
		s.book("klong", "usertest", 2, s.at(5, 19, 0), 60, false),
		s.book("klong", "malee", 2, s.at(1, 19, 0), 60, true),
		// สเต็กลุงพล ร้านข้ามเที่ยงคืน: จองคร่อมเที่ยงคืน และจองหลังเที่ยงคืน (วันทำการก่อนหน้า)
		s.book("steak", "chutipun", 2, s.at(2, 22, 30), 120, false),
		s.book("steak", "somsri", 4, s.at(3, 0, 30), 90, false),
		// ใบชา คาเฟ่ พักรับจอง แต่ยังมีการจองเดิมค้างอยู่ (ลูกค้ายังยกเลิกได้)
		s.book("tea", "kanya", 2, s.at(4, 14, 0), 60, false),
		s.book("dimsum", "chutipun", 5, s.at(4, 9, 0), 90, false),
		s.book("dimsum", "krit", 3, s.at(4, 9, 30), 60, false),
		s.book("bbqk", "usertest", 4, s.at(3, 19, 0), 120, false),
		s.book("bbqk", "somsri", 2, s.at(1, 20, 0), 90, false),
		// ซูชิบาร์ ท่าเรือ: ไม่มีการจองเลย (เดโมว่าลบร้านได้)

		// ร้านของ usertest: usertest เป็นเจ้าของที่ได้รับการจอง
		s.book("pho", "chutipun", 2, s.at(1, 18, 0), 60, false),
		s.book("pho", "somsri", 3, s.at(2, 18, 0), 60, false),
		s.book("pasta", "kanya", 2, s.at(2, 19, 0), 90, false),
		s.book("pasta", "nida", 4, s.at(6, 12, 0), 60, false),

		s.book("isan", "chutipun", 4, s.at(3, 18, 0), 90, false),
		s.book("isan", "usertest", 6, s.at(2, 12, 0), 60, false),
		s.book("tomyum", "kanya", 2, s.at(1, 11, 0), 60, false),
		s.book("seafood", "nida", 8, s.at(5, 18, 0), 120, false),
		s.book("seafood", "chutipun", 6, s.at(7, 19, 0), 120, false),
		s.book("khaosoi", "somsri", 2, s.at(2, 9, 0), 60, false),

		s.book("ramen", "chutipun", 2, s.at(2, 12, 0), 60, false),
		s.book("ramen", "usertest", 3, s.at(5, 12, 30), 60, false),
		s.book("bingsu", "kanya", 4, s.at(3, 15, 0), 60, false),
		s.book("hotpot", "usertest", 6, s.at(4, 18, 0), 120, false),
		s.book("hotpot", "somsri", 10, s.at(4, 18, 30), 90, false),
		s.book("hotpot", "danai", 20, s.at(4, 19, 0), 60, false),
		s.book("bakery", "kanya", 2, s.at(1, 8, 0), 60, false),

		s.book("pizza", "chutipun", 3, s.at(1, 19, 0), 90, false),
		s.book("pizza", "nida", 4, s.at(3, 18, 30), 90, false),
		s.book("gaiyang", "usertest", 5, s.at(6, 12, 0), 60, false),
		// ข้าวต้มโต้รุ่ง เปิด 18:00–03:00 ข้ามเที่ยงคืน
		s.book("jok", "kanya", 2, s.at(1, 23, 30), 90, false),
		s.book("jok", "chutipun", 4, s.at(2, 1, 0), 60, false),
		// สลัดผักสวนครัว: ร้านใหม่ ไม่มีการจองและไม่มีรีวิว

		// ประวัติของลูกค้า (แท็บ "ผ่านไปแล้ว" และ "ยกเลิกแล้ว")
		s.book("pho", "chutipun", 2, s.at(-3, 12, 0), 60, false),
		s.book("ramen", "chutipun", 2, s.at(-10, 12, 0), 60, false),
		s.book("isan", "chutipun", 4, s.at(-6, 18, 0), 90, false),
		s.book("pizza", "chutipun", 2, s.at(4, 19, 0), 60, true),
		s.book("hotpot", "chutipun", 4, s.at(-2, 18, 0), 120, true),
		s.book("klong", "usertest", 3, s.at(-3, 12, 0), 60, false),
		s.book("seafood", "usertest", 4, s.at(-8, 19, 0), 120, false),
		s.book("bingsu", "somsri", 2, s.at(-4, 15, 0), 60, false),
		s.book("dimsum", "kanya", 3, s.at(-5, 9, 0), 60, false),
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
		{"klong", "usertest", 5, 29, "ปลากะพงทอดน้ำปลากรอบมาก นั่งริมคลองสบาย จองล่วงหน้าแล้วได้โต๊ะตามเวลาพอดี"},
		{"klong", "danai", 4, 5, "ต้มยำทะเลรสจัดจ้านกำลังดี ช่วงเย็นคนค่อนข้างเยอะ แนะนำให้จองก่อน"},
		{"klong", "malee", 5, 12, "บริการเร็ว พนักงานสุภาพ มีเก้าอี้เด็กให้"},
		{"klong", "pim", 4, 20, "อาหารอร่อย ราคาเหมาะสม ที่จอดรถมีจำกัด"},
		{"klong", "somsri", 5, 7, "ต้มยำทะเลน้ำข้นเข้มข้น กุ้งตัวใหญ่สด"},
		// รีวิวเดียว 5 ดาว: ใช้เดโมว่า Bayesian ไม่ให้ร้านนี้ขึ้นอันดับหนึ่งเหนือร้านที่มีหลายรีวิว
		{"sushi", "kanya", 5, 3, "ปลาสดมาก ข้าวซูชิกำลังดี นั่งเคาน์เตอร์ได้คุยกับเชฟ"},
		{"steak", "somsri", 4, 4, "สเต็กนุ่ม ย่างได้ระดับที่สั่ง เปิดดึกสะดวกมาก"},
		{"steak", "pim", 4, 9, "เบอร์เกอร์ชิ้นใหญ่ คุ้มราคา"},
		{"steak", "krit", 5, 14, "ริบอายเนื้อนุ่ม ซอสพริกไทยดำเข้ากันดี"},
		{"steak", "nida", 3, 21, "รสชาติดีแต่เสิร์ฟช้าช่วงสามทุ่ม"},
		{"tea", "kanya", 5, 25, "ชาไทยหอมเข้ม เค้กมะพร้าวอร่อย"},
		{"dimsum", "kanya", 4, 5, "ฮะเก๋ากุ้งเต็มคำ ขนมจีบร้อน ๆ เสิร์ฟไว"},
		{"dimsum", "malee", 5, 11, "ซาลาเปาลาวาไส้ไหลเยิ้ม ชาจีนหอม"},
		{"dimsum", "ton", 4, 16, "วันหยุดคนเยอะมาก ดีที่จองไว้ก่อน"},
		{"bbqk", "somsri", 5, 6, "หมูสามชั้นหมักซอสเกาหลีหอมมาก กิมจิทำเอง"},
		{"bbqk", "june", 4, 13, "พนักงานย่างให้ตลอด ไม่ต้องลุกเลย"},
		{"bbqk", "danai", 4, 19, "เนื้อคาลบีนุ่ม เครื่องเคียงหลากหลาย"},
		{"pho", "chutipun", 5, 2, "น้ำซุปหอมกลมกล่อม เส้นเหนียวนุ่ม ได้โต๊ะตามที่จองไว้"},
		{"pho", "danai", 5, 15, "ปอเปี๊ยะสดผักกรอบ น้ำจิ้มอร่อย"},
		{"pho", "malee", 4, 30, "ร้านเล็กแต่สะอาด รอไม่นาน"},
		{"pho", "krit", 5, 9, "มากินซ้ำทุกอาทิตย์"},
		{"pasta", "nida", 4, 8, "คาโบนาราสูตรไข่แดงแท้ ไม่ใส่ครีม"},
		{"pasta", "ploy", 5, 17, "เส้นสดเหนียวนุ่ม บรรยากาศดีมากตอนพระอาทิตย์ตก"},
		{"pasta", "pim", 4, 23, "ทีรามิสุอร่อย แต่ทางเข้าร้านหายากนิดหน่อย"},
		{"isan", "chutipun", 4, 5, "ส้มตำปูปลาร้ารสเด็ด ไก่ย่างหนังกรอบ"},
		{"isan", "usertest", 5, 10, "ลาบเป็ดอร่อยที่สุดในย่านนี้"},
		{"isan", "krit", 3, 18, "อร่อยแต่ช่วงเที่ยงคนเยอะ รอนานหน่อย"},
		{"isan", "kanya", 5, 27, "แจ่วฮ้อนน้ำซุปหอม ผักสดเยอะ"},
		{"tomyum", "somsri", 5, 4, "ต้มยำกุ้งน้ำใสเปรี้ยวนำ หอมข่าตะไคร้"},
		{"tomyum", "ton", 4, 12, "ไข่เจียวปูฟูกรอบ ราคาไม่แพง"},
		{"tomyum", "nida", 4, 22, "ร้านเก่าแต่สะอาด รสชาติคงที่"},
		{"seafood", "usertest", 4, 7, "กุ้งเผาตัวโต น้ำจิ้มซีฟู้ดแซ่บ"},
		{"seafood", "june", 5, 15, "นั่งริมหาดตอนพระอาทิตย์ตก ของสดทุกอย่าง"},
		{"seafood", "arthit", 4, 24, "คุ้มค่าบุฟเฟต์ ปูม้าเนื้อแน่น"},
		{"seafood", "malee", 3, 31, "อาหารสดแต่ช่วงวันหยุดคนแน่นมาก"},
		{"khaosoi", "kanya", 5, 9, "ข้าวซอยไก่น้ำแกงเข้มข้น เส้นกรอบด้านบนหอม"},
		{"khaosoi", "pim", 5, 18, "ไส้อั่วรสจัดแบบเหนือแท้"},
		{"khaosoi", "nida", 4, 26, "น้ำพริกหนุ่มอร่อย อยากให้เปิดถึงเย็น"},
		{"ramen", "chutipun", 4, 9, "น้ำซุปมิโซะเข้มข้น ไข่ยางมะตูมกำลังดี"},
		{"ramen", "usertest", 4, 14, "เกี๊ยวซ่าทอดกรอบ ร้านเล็กควรจองก่อน"},
		{"ramen", "danai", 5, 3, "หมูชาชูนุ่มละลายในปาก"},
		{"bingsu", "somsri", 5, 3, "บิงซูสตรอว์เบอร์รีเนื้อเนียน ไม่หวานเกินไป"},
		{"bingsu", "ploy", 4, 11, "ร้านน่ารัก ถ่ายรูปสวย"},
		{"bingsu", "kanya", 4, 20, "บิงซูมะม่วงหวานฉ่ำ แบ่งกินสองคนได้"},
		{"hotpot", "usertest", 4, 6, "น้ำซุปหม่าล่าเผ็ดชา เนื้อสไลซ์บางดี"},
		{"hotpot", "somchai", 4, 13, "ลูกชิ้นกุ้งเด้ง น้ำซุปเติมให้ตลอด"},
		{"hotpot", "krit", 3, 28, "ของสดดีแต่เติมช้าตอนคนเยอะ"},
		{"hotpot", "june", 5, 2, "มากับเพื่อนสิบคน จองล่วงหน้าแล้วได้โต๊ะรวมพอดี"},
		{"bakery", "ton", 5, 4, "ครัวซองต์เนยสดกรอบนอกนุ่มใน"},
		{"bakery", "malee", 4, 10, "ซาวร์โดว์หอม กาแฟดี ที่นั่งน้อยไปหน่อย"},
		{"pizza", "somsri", 5, 5, "แป้งบางขอบพองจากเตาฟืน มาร์เกริตาหอมโหระพา"},
		{"pizza", "nida", 4, 12, "พิซซ่าทรัฟเฟิลหอมมาก ราคาค่อนข้างสูง"},
		{"pizza", "danai", 4, 19, "พานาคอตตาเนียนนุ่ม"},
		{"pizza", "ploy", 5, 29, "อร่อยเหมือนกินที่อิตาลี"},
		{"gaiyang", "usertest", 5, 8, "ไก่ย่างหนังกรอบ น้ำจิ้มแจ่วรสจัด"},
		{"gaiyang", "somchai", 4, 15, "ส้มตำไทยรสกลมกล่อม"},
		{"gaiyang", "pim", 4, 22, "คอหมูย่างนุ่ม ข้าวเหนียวมะม่วงหวานพอดี"},
		{"gaiyang", "kanya", 5, 1, "ร้านกว้าง มากันเป็นครอบครัวสะดวก"},
		{"jok", "kanya", 4, 6, "ข้าวต้มปลาร้อน ๆ ตอนดึก ถูกใจคนเลิกงานดึก"},
		{"jok", "june", 4, 13, "ผัดคะน้าปลาเค็มไฟแรงหอมกระทะ"},
		{"jok", "krit", 5, 17, "เปิดถึงตีสาม ช่วยชีวิตคนทำงานกะดึก"},
	}
	for i, r := range list {
		created := s.now.Add(-time.Duration(r.daysAgo) * 24 * time.Hour)
		a := aspectsAround(r.rating, i)
		if _, err := s.tx.Exec(s.ctx, `
			INSERT INTO reviews (restaurant_id, user_id, rating_food, rating_ambience, rating_service, rating_value,
			                     comment, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)`,
			s.rest[r.rest], s.users[r.user], a[0], a[1], a[2], a[3], r.comment, created); err != nil {
			return fmt.Errorf("review %s/%s: %w", r.rest, r.user, err)
		}
	}
	return nil
}

// aspectsAround กระจายคะแนน 4 ด้านรอบ ๆ คะแนนที่ตั้งใจไว้ แบบไม่สุ่ม (seed เดิมต้องได้ผลเดิมเสมอ)
// ด้านที่ i เลือกจากตารางออฟเซ็ต แล้วหนีบให้อยู่ในช่วง 1–5 · คะแนนรวมที่ฐานข้อมูลคำนวณจะใกล้ rating เดิม
func aspectsAround(rating, i int) [4]int {
	// แต่ละแถวบวกลบไม่เกิน 1 และรวมกันได้ 0 คะแนนเฉลี่ยจึงไม่เลื่อนจาก rating
	offsets := [4][4]int{
		{0, 0, 0, 0},
		{1, 0, -1, 0},
		{0, -1, 1, 0},
		{-1, 1, 0, 0},
	}
	row := offsets[i%len(offsets)]
	var out [4]int
	for k := range out {
		out[k] = min(5, max(1, rating+row[k]))
	}
	return out
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
