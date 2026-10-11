# 01 · ที่นั่ง การจอง และรีวิว

**สถานะ:** ยืนยันแล้ว · ฉบับ 1.2 (2 ต.ค. 2026)
**ขอบเขตไฟล์นี้:** backend (Go + PostgreSQL)
**อ่านก่อน:** `docs/00-overview.md` (schema รวม, error ทั้งระบบ, ข้อตกลงร่วม)

---

## 0. วิธีใช้ไฟล์นี้ (สำหรับ Claude Code)

- อ่าน `00-overview.md` ก่อนเริ่ม แล้วทำตามลำดับใน §11 ทีละขั้น จบแต่ละขั้นให้ **หยุด** สรุปสิ่งที่เปลี่ยน อธิบายโค้ดส่วนสำคัญเป็นภาษาไทย แล้วรอผู้ใช้ยืนยันก่อนไปขั้นถัดไป
- ห้ามเพิ่มฟีเจอร์ ตาราง คอลัมน์ endpoint หรือ dependency ที่ไม่มีในไฟล์นี้ ถ้าสเปกกำกวมหรือขัดกันเอง **ให้ถาม ห้ามเดา**
- ทุกกติกาใน §5–§7 ต้องมี test ตาม §10
- เมื่อแก้ bug ให้เขียน test ที่จำลอง bug นั้นให้พังก่อน แล้วค่อยแก้

---

## 1. ขอบเขต

**อยู่ในส่วนนี้:** schema, pure functions (ที่นั่ง, เวลา), service ที่ทำงานใน transaction, HTTP handlers, tests ของ
- การจอง / แก้ไข / ยกเลิก / การจองของฉัน
- ที่ว่างรายช่องเวลา
- Owner แก้จำนวนที่นั่ง เวลาเปิด–ปิด และนโยบายของร้าน
- รีวิว และรายการร้านเรียงตามคะแนน

**ไม่อยู่ในส่วนนี้:** login จริง (`02-auth.md`), สร้าง/แก้/ลบร้าน รูป หมวดหมู่ พักรับจอง ค้นหา (`03-restaurants.md`), หน้าเว็บ (`05-frontend.md`)

**การระบุตัวผู้ใช้ระหว่างส่วนที่ 1**
- service ทุกตัวรับ `userID int64` เป็นพารามิเตอร์
- handler อ่าน userID ผ่าน `auth.UserID(ctx) (int64, bool)` ใน `internal/auth` ซึ่งจะ implement จริงในส่วนที่ 2
- ใน test ให้ใส่ userID ลง context ผ่าน fake middleware ที่อยู่ในไฟล์ `_test.go` เท่านั้น
- **ห้ามมี header, query parameter หรือช่องทางใดๆ สำหรับสลับผู้ใช้ใน production code**

---

## 2. หลักการที่ห้ามละเมิด

1. กติกาธุรกิจทุกข้อตรวจที่ Go เสมอ (สไลด์ 4) constraint ในฐานข้อมูลเป็นแค่ชั้นป้องกันที่สอง
2. ช่วงเวลาทุกช่วงเป็นแบบ half-open `[start, end)`
3. ข้อมูลของร้าน (capacity, เวลาเปิด–ปิด, นโยบาย, สถานะรับจอง) ที่ใช้ตัดสินการจอง ต้องอ่าน **หลัง** ล็อกแถวร้านด้วย `lockRestaurant` เท่านั้น (helper นี้กรองร้านที่ถูกลบออกด้วย)
4. Transaction ใช้ **Read Committed** (ค่าเริ่มต้นของ PostgreSQL) ห้ามใช้ Repeatable Read ในงานจอง เพราะ snapshot จะถูกจับก่อนรอ lock ทำให้มองไม่เห็นการจองที่อีกฝ่ายเพิ่ง commit และ overbook ได้
5. ลำดับการล็อกต้องเป็น **แถวร้าน → แถวการจอง** เสมอ เพื่อกัน deadlock
6. ห้ามเรียก `time.Now()` ใน domain หรือ service ให้รับ `now time.Time` เป็นพารามิเตอร์ handler เป็นผู้ส่งค่าเข้ามา
7. เขียน SQL เองผ่าน pgx ห้ามใช้ ORM
8. Error ทุกกรณีตอบเป็น `application/problem+json` (RFC 9457) ตาม `00-overview.md` §5
9. ความเป็นเอกลักษณ์ (เช่น 1 คน 1 รีวิว) ให้ unique constraint เป็นผู้ตัดสิน ห้ามใช้วิธีเช็คก่อนแล้วค่อย INSERT

---

## 3. Tech และโครงสร้าง

- Go 1.25+ ใช้ `net/http` ServeMux แบบมี method และ wildcard เช่น `GET /api/restaurants/{id}` ไม่ใช้ web framework
- `github.com/jackc/pgx/v5` (pgxpool)
- PostgreSQL
- ใส่ `import _ "time/tzdata"` ใน `cmd/api/main.go` และโหลด `Asia/Bangkok` ครั้งเดียวตอนเริ่มโปรแกรม
- Route ทั้งหมดขึ้นต้นด้วย `/api` (Next.js จะ rewrite `/api/*` มาที่ Go ในภายหลัง)
- JSON ใช้ชื่อฟิลด์แบบ snake_case, decode ด้วย `DisallowUnknownFields` และจำกัดขนาด body ด้วย `http.MaxBytesReader` (1 MB)
- Integration test อ่านฐานข้อมูลจาก env `TEST_DATABASE_URL` ถ้าไม่ได้ตั้งให้ `t.Skip`
- โครงสร้าง repo และ package ทั้งหมดอยู่ใน `00-overview.md` §3 ส่วนนี้ใช้ `internal/booking`, `internal/restaurant` (settings), `internal/review`, `internal/db`, `internal/httpx`


---

## 4. Schema

ใช้ schema รวมใน `00-overview.md` §4 ส่วนนี้ใช้ตาราง `users`, `restaurants`, `bookings`, `reviews`

หมายเหตุเฉพาะส่วนนี้
- ไม่ใช้ trigger สำหรับ `updated_at` ให้ตั้งค่าในคำสั่ง UPDATE เอง
- การตรวจช่องเวลา 30 นาทีของการจองทำที่ Go เท่านั้น (ใน DB ค่า `EXTRACT` ของ `timestamptz` ขึ้นกับ timezone ของ session จึงไม่ใช้เป็น CHECK)
- `btrim` ใน CHECK ตัดเฉพาะช่องว่าง ฝั่ง Go ต้องใช้ `strings.TrimSpace` ซึ่งตัด whitespace ทุกชนิด
- ร้านที่ `deleted_at IS NOT NULL` ถือว่าไม่มีอยู่สำหรับทุก endpoint ในส่วนนี้ ยกเว้น "การจองของฉัน" ที่ยังแสดงประวัติ (§7.4)

---

## 5. โมเดลเวลา (`booking/rules.go` — pure)

- เก็บ `start_at`, `end_at`, `cancel_deadline` เป็น `timestamptz` รับและส่งใน API เป็น RFC 3339 พร้อม offset เช่น `2026-10-11T12:00:00+07:00`
- **ช่องเวลา 30 นาที:** เมื่อแปลงเป็นเวลา `Asia/Bangkok` นาทีต้องเป็น 0 หรือ 30 และวินาทีกับ nanosecond ต้องเป็น 0
- **ระยะเวลาจอง:** 30 นาที ≤ `end − start` ≤ 3 ชั่วโมง
- **เวลาเปิด–ปิดร้าน** (`TIME`): ถ้า `close_time < open_time` แปลว่าข้ามเที่ยงคืน ห้ามตั้งเท่ากัน (ร้าน 24 ชม. อยู่นอกขอบเขต)
- **หน้าต่างเปิดร้าน:** ให้ D = วันที่ท้องถิ่นของ `start` สำหรับ B ใน {D−1, D}
  - `ws = B + open_time`
  - `we = B + close_time` ถ้า `close_time > open_time` มิฉะนั้น `we = (B+1) + close_time`
  - การจองถูกต้องเมื่อมี B ที่ `ws ≤ start` และ `end ≤ we` (อยู่ในหน้าต่างเดียวตลอดทั้งช่วง)
  - ตัวอย่าง ร้าน 18:00–02:00: จอง 01:00–02:00 ของวันที่ 12 อยู่ในหน้าต่างของวันที่ 11 จึงผ่าน
- **เงื่อนไขเทียบกับ now** (ตรวจตามลำดับนี้ เพื่อให้ error บอกเหตุผลตรงที่สุด)
  1. `start > now` มิฉะนั้น `booking-in-past`
  2. `start ≥ now + min_lead_minutes` มิฉะนั้น `lead-time-not-met`
  3. `start ≤ now + 30 วัน` มิฉะนั้น `too-far-in-advance`
- **Deadline:** `cancel_deadline = start − cancel_window_minutes` โดยใช้ค่าของร้าน ณ เวลาที่สร้างหรือแก้การจอง แล้วบันทึกลงแถวการจอง
- **ยกเลิกหรือแก้ไขได้** เมื่อ `now ≤ cancel_deadline` (นับรวมเวลาพอดี)

---

## 6. อัลกอริทึมที่นั่ง (`booking/capacity.go` — pure)

```go
type Interval struct {
    Start, End time.Time // [Start, End)
    Size       int
}

// MaxLoad คืนโหลดสูงสุดของ existing ภายในช่วง [s, e)
// และจุดเวลาแรกที่เกิดโหลดนั้น
func MaxLoad(existing []Interval, s, e time.Time) (load int, at time.Time)

// PeakAll คืน peak ของทุกช่วงรวมกัน และจุดเวลาแรกที่เกิด peak
func PeakAll(all []Interval) (peak int, at time.Time)
```

**MaxLoad**
- จุดตรวจ = `s` รวมกับ `b.Start` ทุกตัวที่ `s < b.Start < e` เรียงจากเร็วไปช้า
- ที่จุด t: โหลด = ผลรวม `b.Size` ของทุก b ที่ `b.Start ≤ t < b.End`
- คืนโหลดที่มากที่สุด ถ้าเท่ากันหลายจุดคืนจุดแรก ถ้าไม่มีการจองเลยคืน `0, s`
- เหตุผล: ภายใน `[s, e)` โหลดจะเพิ่มขึ้นได้เฉพาะตอนมีการจองเริ่ม ค่าสูงสุดจึงอยู่ที่จุดใดจุดหนึ่งในชุดนี้เสมอ
- ฟังก์ชันต้องให้ผลถูกแม้ได้รับช่วงที่ไม่ทับ `[s, e)` มาด้วย
- การใช้งาน: จองได้เมื่อ `n + load ≤ capacity`, ที่ว่าง = `capacity − load`

**PeakAll** (sweep line)
- สร้าง event `+Size` ที่ Start และ `−Size` ที่ End แล้วเรียงตามเวลา
- **ถ้าเวลาเท่ากัน ให้ event จบ (−) มาก่อน event เริ่ม (+)** ซึ่งคือ half-open ในรูปของอัลกอริทึม
- เดินผ่าน event เก็บผลรวมสะสมและค่าสูงสุด

ความซับซ้อน: MaxLoad เป็น O(k²) โดย k คือจำนวนการจองที่ทับช่วง (สำหรับร้านเดียวในช่วงไม่เกิน 3 ชม. ถือว่าน้อย) ส่วน PeakAll เป็น O(k log k)

---

## 7. Pipelines และ API

รหัส error ใน pipeline หมายถึง `type` ใน `00-overview.md` §5

### 7.1 จองใหม่ — `POST /api/restaurants/{id}/bookings`

Body: `{ "party_size": 4, "start_at": "...", "end_at": "..." }`

| ขั้น | ตรวจอะไร | ถ้าไม่ผ่าน |
|---|---|---|
| **ก่อนเข้า transaction** | | |
| 1 | มี userID | 401 `unauthenticated` |
| 2 | รูปแบบ: `party_size` เป็นจำนวนเต็ม ≥ 1, เวลา parse ได้, start < end, ลงช่อง 30 นาที, ระยะเวลา 30 นาที–3 ชม. | 400 `validation` |
| **ใน transaction หลัง `lockRestaurant`** | | |
| 3 | ร้านมีอยู่และไม่ถูกลบ | 404 `not-found` |
| 4 | `owner_id ≠ userID` | 403 `own-restaurant-booking` |
| 5 | `accepting_bookings = true` | 409 `restaurant-not-accepting-bookings` |
| 6 | เงื่อนไขเทียบ now ตาม §5 | 422 `booking-in-past` / `lead-time-not-met` / `too-far-in-advance` |
| 7 | อยู่ในหน้าต่างเปิดร้าน | 422 `outside-opening-hours` |
| 8 | `party_size ≤ capacity` | 422 `party-exceeds-capacity` |
| 9 | ผู้ใช้ไม่มีการจอง active อื่นที่ทับช่วงนี้ในร้านเดียวกัน | 409 `overlapping-own-booking` |
| 10 | `party_size + MaxLoad ≤ capacity` (นับเฉพาะ active) | 409 `seat-capacity-exceeded` พร้อม `available`, `full_from` |
| 11 | INSERT พร้อม `cancel_deadline` แล้ว COMMIT | 201 คืนการจอง |

Query ที่ใช้ในขั้น 9–10 (เงื่อนไขทับช่วง `start_at < $e AND end_at > $s`):
```sql
SELECT 1 FROM bookings
 WHERE restaurant_id = $1 AND user_id = $2 AND status = 'active'
   AND start_at < $4 AND end_at > $3
 LIMIT 1;

SELECT start_at, end_at, party_size FROM bookings
 WHERE restaurant_id = $1 AND status = 'active'
   AND start_at < $3 AND end_at > $2;
```

### 7.2 แก้ไข — `PATCH /api/bookings/{id}`

Body: ส่งครบทั้ง `party_size`, `start_at`, `end_at` (แทนค่าเดิมทั้งหมด ห้ามเปลี่ยนร้าน)

1. 401 / 400 เหมือนขั้น 1–2 ของ §7.1
2. BEGIN แล้ว `SELECT restaurant_id FROM bookings WHERE id = $1 AND user_id = $2` ถ้าไม่พบ 404 `not-found` (ไม่เปิดเผยว่าเป็นการจองของคนอื่น)
3. ล็อกแถวร้านด้วย `lockRestaurant` (ร้านถูกลบ → 404) แล้วล็อกแถวการจอง `FOR UPDATE` (ตามลำดับใน §2 ข้อ 5)
4. `status ≠ 'active'` → 409 `booking-not-active`
5. `now > cancel_deadline` (ของการจองเดิม) → 422 `cancel-deadline-passed`
6. ร้านพักรับจอง → 409 `restaurant-not-accepting-bookings` (การแก้ไขคือการจองใหม่ ถ้าปล่อยให้ย้ายวันได้ ร้านจะลบไม่ได้เสียที)
7. ขั้น 6–10 ของ §7.1 กับค่าใหม่ โดย **ตัดการจองนี้ออก** (`AND id <> $bookingID`) ทั้งใน query ขั้น 9 และขั้น 10
8. UPDATE ค่าใหม่ คำนวณ `cancel_deadline` ใหม่จากนโยบายปัจจุบันของร้าน ตั้ง `updated_at` แล้ว COMMIT → 200

### 7.3 ยกเลิก — `POST /api/bookings/{id}/cancel`

ไม่เช็คที่นั่งและไม่ล็อกแถวร้าน ยกเลิกได้แม้ร้านพักรับจองอยู่ ใช้คำสั่งเดียวแบบมีเงื่อนไข:
```sql
UPDATE bookings
   SET status = 'cancelled', cancelled_at = $3, updated_at = $3
 WHERE id = $1 AND user_id = $2 AND status = 'active' AND $3 <= cancel_deadline
RETURNING *;
```
ถ้าไม่มีแถวถูกอัปเดต ให้อ่านแถวนั้นเพื่อหาสาเหตุ:

| สาเหตุ | ตอบ |
|---|---|
| ไม่พบ หรือไม่ใช่ของผู้ใช้คนนี้ | 404 `not-found` |
| ยกเลิกไปแล้ว | 409 `already-cancelled` |
| เลย deadline | 422 `cancel-deadline-passed` |
| สำเร็จ | 200 คืนการจอง |

### 7.4 การจองของฉัน — `GET /api/me/bookings`

- คืนการจองของผู้ใช้ตามแท็บ `tab` (ค่าเริ่มต้น `upcoming`, ค่าอื่น → 400 `validation`)
  - `upcoming`: `status = 'active' AND end_at > now` เรียง `start_at ASC, id ASC` (ใกล้สุดก่อน)
  - `past`: `status = 'active' AND end_at ≤ now` เรียง `start_at DESC, id DESC`
  - `cancelled`: `status = 'cancelled'` เรียง `cancelled_at DESC, id DESC`
- รวมการจองของร้านที่ถูกลบแล้วด้วย (เป็นประวัติของลูกค้า)
- ฟิลด์: `id`, `restaurant_id`, `restaurant_name`, `party_size`, `start_at`, `end_at`, `status`, `cancel_deadline` และฟิลด์ที่ **คำนวณตอนอ่าน ไม่เก็บลงฐานข้อมูล**
  - `is_past` = `end_at ≤ now`
  - `can_modify` = `status = 'active' AND now ≤ cancel_deadline AND ร้านยังไม่ถูกลบ`
  - `restaurant_deleted` = ร้านถูกลบแล้ว
- แบ่งหน้าด้วย `page`, `page_size` (ค่าเริ่มต้น 20 สูงสุด 50) ตอบ `{ items, page, page_size, total, counts: { upcoming, past, cancelled } }` (`counts` ใช้แสดงตัวเลขบนแท็บ)

### 7.5 ที่ว่างรายช่องเวลา — `GET /api/restaurants/{id}/availability?date=YYYY-MM-DD`

- `date` คือวันทำการ D ใช้หน้าต่างเปิดของ D ตาม §5
- ช่องทีละ 30 นาทีใน `[ws, we)` สำหรับแต่ละช่อง t:
  - `remaining = capacity − (ผลรวม party_size ของการจอง active ที่ start_at ≤ t < end_at)`
  - `limited = remaining > 0 AND remaining * 10 <= capacity * 3` (เหลือ ≤ 30% ใช้เลขจำนวนเต็มเพื่อเลี่ยงทศนิยม)
  - `bookable` = ร้าน `accepting_bookings = true` และช่องนั้นผ่านเงื่อนไขเทียบ now ใน §5 และ `remaining > 0`
- ไม่ล็อก ผลลัพธ์เป็น **คำแนะนำเท่านั้น** การตัดสินจริงเกิดใน transaction ของ §7.1
- Response: `{ restaurant_id, date, capacity, accepting_bookings, slots: [{ start, end, remaining, limited, bookable }] }`
- `date` ผิดรูปแบบ → 400; ร้านไม่พบหรือถูกลบ → 404
- endpoint นี้เป็นสาธารณะ ไม่ต้อง login

### 7.6 Owner แก้ตั้งค่า — `PATCH /api/restaurants/{id}/settings`

Body: ส่งครบ `capacity`, `open_time` (`"HH:MM"`), `close_time`, `cancel_window_minutes`, `min_lead_minutes`

1. 401; รูปแบบผิด, `capacity` นอกช่วง 1–500, เวลาไม่ลงช่อง 30 นาที หรือเวลาเปิดเท่ากับเวลาปิด → 400 `validation` (ใช้กฎเดียวกับตอนสร้างร้านใน `03-restaurants.md` §3)
2. BEGIN แล้ว `lockRestaurant` ถ้าไม่พบหรือถูกลบ 404
3. `owner_id ≠ userID` → 403 `not-restaurant-owner`
4. นโยบาย: `cancel_window_minutes ≥ 30` และ `min_lead_minutes ≥ cancel_window_minutes` → ไม่ผ่าน 422 `invalid-restaurant-policy` (422 ใช้เฉพาะสองค่านี้)
5. ดึงการจอง active ที่ `end_at > now` แล้วตรวจ
   - `PeakAll ≤ capacity ใหม่`
   - ทุกการจองยังอยู่ในหน้าต่างเปิดตามเวลาใหม่
6. ถ้ามีข้อใดไม่ผ่าน → 409 `settings-conflict-with-bookings` พร้อม `conflicts: [{ booking_id, start_at, end_at, reason }]` โดย `reason` เป็น `capacity` หรือ `opening_hours`
   - `capacity`: ส่งการจองที่ครอบ **จุด peak แรก** ที่ `PeakAll` คืน และเขียนใน `detail` ว่าอาจมีช่วงอื่นที่ชนอีก (เจ้าของแก้แล้วบันทึกใหม่จะเห็นจุดถัดไป)
   - `opening_hours`: ส่งทุกการจองที่หลุดเวลาเปิดใหม่
7. UPDATE แล้ว COMMIT → 200

การเปลี่ยน `cancel_window_minutes` หรือ `min_lead_minutes` ไม่กระทบการจองที่มีอยู่ เพราะ deadline ถูกบันทึกไว้แล้ว ตั้งค่าได้แม้ร้านพักรับจองอยู่ (การพักรับจองอยู่ใน `03-restaurants.md`)

### 7.7 รีวิว

**สร้าง — `POST /api/restaurants/{id}/reviews`** Body: `{ "aspects": { "food": 5, "ambience": 4, "service": 3, "value": 4 }, "price_level": 3, "comment": "..." }`

ผู้รีวิวให้คะแนน **4 ด้าน** คือ `food` (อาหาร), `ambience` (บรรยากาศ), `service` (บริการ), `value` (ความคุ้มค่า)
`rating` ที่เป็นคะแนนรวมไม่ได้รับจาก body แต่เป็น **generated column** ที่ PostgreSQL คำนวณเป็นค่าเฉลี่ยของทั้งสี่ (`NUMERIC(3,2)`)

`price_level` คือช่วงราคาต่อคนที่ผู้รีวิวจ่ายจริง **ไม่บังคับ** (`null` หรือไม่ส่ง = ไม่ระบุ) เก็บเป็นลำดับ 1–5 ไม่ใช่จำนวนเงิน
เพราะถ้าให้พิมพ์ตัวเลขเองจะได้ "250", "250 บาท", "~300", "สองคน 600" ปนกันจนรวมเป็นสถิติไม่ได้
`1` = ต่ำกว่า 100 · `2` = 100–200 · `3` = 201–300 · `4` = 301–500 · `5` = มากกว่า 500 (ช่วงถี่ที่ราคาต่ำเพราะร้านส่วนใหญ่ไม่เกิน 500 บาท/คน)
คะแนนรวมกับรายด้านจึงไม่มีทางไม่ตรงกัน ไม่ว่าจะเขียนผ่าน path ไหน และการจัดอันดับแบบ Bayesian ใน §7.8 ยังใช้ `rating` เหมือนเดิม

| ขั้น | ตรวจอะไร | ถ้าไม่ผ่าน |
|---|---|---|
| 1 | มี userID | 401 |
| 2 | ทั้ง 4 ด้านเป็นจำนวนเต็ม 1–5 · `price_level` ถ้าส่งมาต้องเป็นจำนวนเต็ม 1–5 (ขาดด้านใดด้านหนึ่งก็ไม่ผ่าน · ชื่อฟิลด์ใน error คือ `aspects.food` เป็นต้น), `comment` หลัง `strings.TrimSpace` ยาว 1–1000 ตัวอักษร (นับเป็น rune) | 400 `validation` |
| 3 | ร้านมีอยู่และไม่ถูกลบ | 404 |
| 4 | ไม่ใช่เจ้าของร้าน (สไลด์ 9) | 403 `own-restaurant-review` |
| 5 | INSERT เลย ถ้าชน unique (SQLSTATE `23505`) | 409 `already-reviewed` |
| 6 | สำเร็จ | 201 |

**แก้ไข — `PUT /api/reviews/{id}`** เฉพาะผู้เขียน (ไม่ใช่ผู้เขียน → 404) ตรวจรูปแบบเหมือนขั้น 2 ตั้ง `updated_at` → 200

**ลบ — `DELETE /api/reviews/{id}`** เฉพาะผู้เขียน → 204 (**ไม่มี** endpoint ให้ Owner ลบรีวิวในร้านของตัวเอง)

**รายการรีวิวของร้าน — `GET /api/restaurants/{id}/reviews?page=1&page_size=10`** (สาธารณะ)
- เรียง `created_at DESC, id DESC` ตอบ `{ items, page, page_size, total }` (`page_size` สูงสุด 50)
- หน้าเว็บแสดงเป็นกล่องเลื่อนสูงประมาณ 5 แถว โหลดหน้าถัดไปเองเมื่อเลื่อนใกล้ก้นกล่องจนครบ (ไม่มีปุ่มแบ่งหน้า) ดู `05-frontend.md` §4
- แต่ละรายการ: `id`, `rating` (คะแนนรวม ทศนิยม 2 ตำแหน่ง), `aspects` (`{ food, ambience, service, value }`), `price_level`, `comment`, `display_name`, `created_at`, `edited` (= `updated_at > created_at`), `is_mine` (`false` เมื่อไม่ได้ login)
- รีวิวของผู้ใช้ปัจจุบันสำหรับร้านนี้ส่งในรายละเอียดร้านเป็นฟิลด์ `my_review` (ดู `03-restaurants.md`) เพราะรายการแบ่งหน้าอาจไม่มีรีวิวนั้นในหน้าแรก
- แก้ไขหรือลบรีวิวของร้านที่ถูกลบ → 404

### 7.8 รายการร้านพร้อมคะแนน — `GET /api/restaurants?sort=highest_rated|most_reviewed&page=1&page_size=20`

- ค่าเริ่มต้น `sort=highest_rated`, `page_size` สูงสุด 50, ค่า sort ไม่รู้จัก → 400
- สาธารณะ ไม่ต้อง login และไม่แสดงร้านที่ถูกลบ
- ตอบ `{ items, page, page_size, total }`
- แต่ละรายการ: `id`, `name`, `capacity`, `open_time`, `close_time`, `avg_rating` (ทศนิยม 1 ตำแหน่ง หรือ `null`), `review_count`, `accepting_bookings`, `categories` (`[{ id, slug, name_th }]`), `cover_image_id` (รูปที่ `id` น้อยที่สุดของร้าน), `is_mine`
- หมวด, รูปปก และ `is_mine` ดึงด้วย query แยกจากรายการ id ในหน้านั้น **ห้ามดึง `restaurant_images.data`**
- ตัวกรอง `q` และ `category` (ระดับ B) อยู่ใน `03-restaurants.md` §8
- Bayesian score = `(C × m + SUM(rating)) / (C + COUNT)` โดย `C = 5` และ `m` = ค่าเฉลี่ยของรีวิวทั้งระบบ **เฉพาะร้านที่ไม่ถูกลบ** หรือ `3.0` ถ้ายังไม่มีรีวิวเลย
- **Highest rated:** `score DESC NULLS LAST, review_count DESC, id ASC` (ร้านที่ไม่มีรีวิวอยู่ท้ายสุด)
- **Most reviewed:** `review_count DESC, avg_raw DESC NULLS LAST, id ASC`
- ต้องมี `id` เป็นตัวตัดสินสุดท้ายเสมอ เพื่อให้ลำดับคงที่และแบ่งหน้าได้ถูกต้อง

แนวทาง SQL:
```sql
WITH g AS (
  SELECT COALESCE(AVG(v.rating), 3.0) AS m
    FROM reviews v JOIN restaurants r ON r.id = v.restaurant_id
   WHERE r.deleted_at IS NULL
), s AS (
  SELECT r.id, r.name, r.capacity,
         COUNT(v.id)             AS review_count,
         AVG(v.rating)           AS avg_raw,
         ROUND(AVG(v.rating), 1) AS avg_rating,
         CASE WHEN COUNT(v.id) = 0 THEN NULL
              ELSE (5 * g.m + SUM(v.rating)) / (5 + COUNT(v.id)) END AS score
    FROM restaurants r
   CROSS JOIN g
    LEFT JOIN reviews v ON v.restaurant_id = r.id
   WHERE r.deleted_at IS NULL
   GROUP BY r.id, g.m
)
SELECT id, name, capacity, avg_rating, review_count
  FROM s
 ORDER BY score DESC NULLS LAST, review_count DESC, id ASC
 LIMIT $1 OFFSET $2;
```

---

## 8. รูปแบบ Error

ใช้ตารางใน `00-overview.md` §5 ทั้งหมด ส่วนนี้ใช้ทุก `type` ที่มีคำว่า 01 ในคอลัมน์ส่วน

---

## 9. Design Decisions (ใช้ใน README)

| การตัดสินใจ | เหตุผล | ที่มา |
|---|---|---|
| นับเป็นที่นั่ง ไม่แยกโต๊ะ | โจทย์กำหนด | สไลด์ 5 |
| ช่วงเวลาแบบ half-open | จบกับเริ่มเวลาเดียวกันไม่ทับกัน ตรงกับตัวอย่างขวา | สไลด์ 7 |
| ตรวจ peak ไม่ใช่ผลรวมของการจองที่ทับ | ผลรวม (17) ให้คำตอบผิด peak (10) ให้คำตอบถูก | สไลด์ 7 |
| แก้ไขต้องตัดการจองเดิมออกก่อนเช็ค | ไม่นับที่นั่งเดิมซ้ำ | สไลด์ 8 |
| ล็อกแถวร้าน + Read Committed | การจองใหม่ยังไม่มีแถวให้ล็อก (phantom) จึงล็อกตัวแทนทรัพยากรคือร้าน | สไลด์ 11 |
| ไม่ใช้ exclusion constraint | กันได้แค่ห้ามทับ ไม่ใช่ผลรวมไม่เกิน | ออกแบบเอง |
| ช่องเวลา 30 นาที, จองได้ 30 นาที–3 ชม., ล่วงหน้าไม่เกิน 30 วัน | UI ง่าย กันการกักที่นั่งนานเกินไป และจำกัดขอบเขตข้อมูล | ตัดสินใจเอง |
| จองล่วงหน้าขั้นต่ำ ร้านตั้งเอง ค่าเริ่มต้น 2 ชม. และต้อง ≥ เวลายกเลิก | ร้านต่างประเภทต้องการเวลาเตรียมต่างกัน และกันการจองที่ยกเลิกไม่ได้ตั้งแต่ตอนสร้าง | ตัดสินใจเอง |
| ยกเลิกค่าเริ่มต้น 30 นาที ร้านตั้งนานขึ้นได้ ห้ามต่ำกว่า 30 | โจทย์กำหนด | สไลด์ 8 |
| บันทึก deadline ไว้ในการจองตั้งแต่ตอนจอง | นโยบายที่เปลี่ยนภายหลังไม่ย้อนหลังทำให้ลูกค้าเสียเปรียบ | ตัดสินใจเอง |
| แก้ไขใช้ deadline เดียวกับยกเลิก | กันการลดจำนวนคนนาทีสุดท้ายเพื่อเลี่ยงกติกายกเลิก | ตัดสินใจเอง |
| Deadline นับรวมเวลาพอดี | "ยกเลิกได้ถึง 11:30" | สไลด์ 8 |
| Owner จองร้านตัวเองไม่ได้ | บทบาทลูกค้าใช้กับร้านของคนอื่น | สไลด์ 3 |
| ห้ามผู้ใช้จองทับเวลาตัวเองในร้านเดียวกัน | กันการกักที่นั่งและการกดจองซ้ำ | ตัดสินใจเอง |
| Owner แก้ตั้งค่าไม่ได้ถ้าทำให้การจองเดิมผิดกติกา | กติกาต้องจริง "ไม่ว่าเวลาไหน" ไม่ใช่แค่ตอนจอง | สไลด์ 7 |
| แยก 409 / 422 | 409 ชนสถานะปัจจุบัน, 422 ผิดกติกาเสมอ | RFC 9110 |
| 1 คน 1 รีวิวต่อร้าน, ไม่ต้องเคยจอง, รีวิวร้านตัวเองไม่ได้ | กันปั่นคะแนน, โจทย์ห้ามเฉพาะรีวิวร้านตัวเอง | สไลด์ 9 |
| Owner ลบรีวิวในร้านตัวเองไม่ได้ | กันการลบรีวิวที่ไม่ชอบ ซึ่งทำลายความน่าเชื่อถือของคะแนน | ตัดสินใจเอง |
| คำนวณค่าเฉลี่ยตอนอ่าน | ข้อมูลเล็ก ความถูกต้องสำคัญกว่า มีแหล่งความจริงเดียว | สไลด์ 11 |
| เรียงด้วย Bayesian (C = 5, m สำรอง 3.0), ร้านไม่มีรีวิวอยู่ท้าย | 5.0 จาก 1 รีวิวไม่ควรชนะ 4.8 จาก 300 รีวิว | สไลด์ 11 |
| สถานะ "ผ่านไปแล้ว" คำนวณจากเวลา | ไม่ต้องมี cron job | ออกแบบเอง |
| Limited Seats Left เมื่อเหลือ ≤ 30% | ความหมายของ "เหลือ 3 ที่" ขึ้นกับขนาดร้าน | สไลด์ 10 |

---

## 10. Test plan

ค่าที่ใช้ร่วมกัน (ถ้าไม่ระบุเป็นอย่างอื่น): ร้าน capacity 10 เปิด 10:00–22:00, `min_lead_minutes = 120`, `cancel_window_minutes = 30`, `now = 2026-10-10T08:00:00+07:00`, วันที่จอง 2026-10-11

### 10.1 Unit: `MaxLoad` / `PeakAll` (table-driven)

| # | existing | คำขอ | คาด |
|---|---|---|---|
| C1 | [12:00–13:00, 7] | 12:30–13:00, n=5 | load 7 ที่ 12:30 → ✕ available 3 (สไลด์ 7 ซ้าย) |
| C2 | A [12:00–12:30, 7], B [12:30–13:00, 7] | 12:00–13:00, n=3 | load 7 → ✓ (สไลด์ 7 ขวา) |
| C3 | B [12:00–13:00, 3] (ตัด A แล้ว) | A แก้เป็น 8 คน 12:00–13:00 | 3+8 = 11 → ✕ (สไลด์ 8) |
| C4 | B [12:00–13:00, 3] (ตัด A แล้ว) | A แก้เป็น 5 คน | 8 → ✓ (สไลด์ 8) |
| C5 | [12:00–13:00, 6] | 12:00–13:00, n=4 | 10 เต็มพอดี → ✓ |
| C6 | [12:00–13:00, 10] | 13:00–14:00, n=10 | load 0 → ✓ (ต่อกันพอดี) |
| C7 | [12:00–12:30, 4], [12:30–13:00, 6], [13:00–13:30, 2] | 12:00–13:30, n=4 / n=5 | load 6 ที่ 12:30 → ✓ / ✕ available 4 |
| C8 | [11:00–12:30, 8] | 12:00–13:00, n=2 / n=3 | load 8 ที่ 12:00 → ✓ / ✕ |
| C9 | ไม่มี | ใดๆ | load 0 |
| C10 | มีช่วงที่ไม่ทับ [09:00–10:00, 9] | 12:00–13:00 | ไม่ถูกนับ |
| P1 | PeakAll: [12:00–13:00, 7], [12:30–13:30, 5] | | 12 ที่ 12:30 |
| P2 | PeakAll: [12:00–13:00, 10], [13:00–14:00, 10] | | 10 (ไม่ใช่ 20) |
| P3 | PeakAll: ว่าง | | 0 |

### 10.2 Unit: rules (เวลา)

| # | กรณี | คาด |
|---|---|---|
| R1 | 12:15–13:00 | 400 (ไม่ลงช่อง) |
| R2 | 12:00–15:30 | 400 (เกิน 3 ชม.) |
| R3 | 12:00–12:00 | 400 |
| R4 | เริ่ม 2026-10-10 07:30 | 422 `booking-in-past` |
| R5 | เริ่ม 2026-10-10 09:30 | 422 `lead-time-not-met` |
| R6 | เริ่ม 2026-10-10 10:00 (ล่วงหน้า 2 ชม. พอดี) | ✓ |
| R7 | เริ่ม 2026-11-10 12:00 | 422 `too-far-in-advance` |
| R8 | 21:00–22:00 | ✓ (จบตรงเวลาปิด) |
| R9 | 21:30–22:30 | 422 `outside-opening-hours` |
| R10 | 09:30–10:30 | 422 `outside-opening-hours` |
| R11 | ร้าน 18:00–02:00: 11 ต.ค. 23:30 → 12 ต.ค. 00:30 | ✓ |
| R12 | ร้าน 18:00–02:00: 12 ต.ค. 01:00–02:00 | ✓ (หน้าต่างของวันที่ 11) |
| R13 | ร้าน 18:00–02:00: 12 ต.ค. 01:30–02:30 | 422 |
| R14 | ร้าน 18:00–02:00: 11 ต.ค. 17:30–18:30 | 422 |
| R15 | deadline: now = deadline พอดี / deadline + 1 นาที | แก้ได้ / ไม่ได้ |

### 10.3 Differential test

- สุ่มสถานการณ์ 10,000 ชุด บนตาราง 48 ช่อง (ช่องละ 30 นาทีใน 1 วัน): existing 0–15 ช่วง แต่ละช่วงยาว 1–6 ช่อง ขนาด 1–10 และคำขอแบบสุ่ม
- Oracle: ไล่ทุกช่องใน `[s, e)` รวมขนาดของช่วงที่ครอบจุดเริ่มช่อง แล้วเอาค่ามากสุด (ถูกแน่นอนเพราะทุกเวลาลงช่อง)
- ต้องได้ `MaxLoad.load` เท่ากับ oracle ทุกชุด
- ใช้ seed คงที่ และพิมพ์ seed กับสถานการณ์ที่พังออกมาเมื่อ test ไม่ผ่าน

### 10.4 Integration: การจอง

| # | กรณี | คาด |
|---|---|---|
| S1–S4 | กรณี C1–C4 ผ่าน service จริง | ตรงกับ unit |
| S5 | active 7 + cancelled 5 ในช่วงเดียวกัน ขอ 3 | ✓ (ไม่นับที่ยกเลิก) |
| S6 | ขอ 11 คนในร้าน 10 ที่ | 422 `party-exceeds-capacity` |
| S7 | Owner จองร้านตัวเอง | 403 |
| S8 | ผู้ใช้เดิมจองทับเวลาตัวเองในร้านเดียวกัน / ไม่ทับ / ผู้ใช้อื่นทับ | 409 / ✓ / ✓ |
| S9 | แก้หลัง deadline | 422 |
| S10 | แก้การจองที่ยกเลิกแล้ว | 409 |
| S11 | แก้การจองของคนอื่น | 404 |
| S12 | แก้แล้ว `cancel_deadline` ถูกคำนวณใหม่ตามนโยบายปัจจุบัน | ✓ |
| S13 | ยกเลิกตรง deadline / หลัง 1 นาที / ซ้ำ / ของคนอื่น | 200 / 422 / 409 / 404 |
| S14 | ร้านเปลี่ยนเวลายกเลิก 30 → 120 หลังมีการจอง | deadline ของการจองเดิมไม่เปลี่ยน |
| S15 | การจองของฉัน: 3 แท็บได้รายการและลำดับถูกต้อง, `counts` ถูกต้อง, `is_past` และ `can_modify` ถูกต้อง, `tab` ผิด → 400 | ✓ |
| S16 | จองร้านที่พักรับจองทั้งที่ที่นั่งพอ | 409 `restaurant-not-accepting-bookings` |
| S17 | แก้ไขการจองของร้านที่พักรับจอง / ยกเลิกการจองของร้านที่พักรับจอง | 409 / 200 |
| S18 | จองหรือแก้ไขกับร้านที่ถูกลบ / การจองเก่าของร้านที่ถูกลบยังอยู่ในการจองของฉันพร้อม `restaurant_deleted` | 404 / ✓ |

### 10.5 Integration: ที่ว่าง

| # | กรณี | คาด |
|---|---|---|
| A1 | มีการจอง 7 คน 12:00–13:00 | ช่อง 12:00 และ 12:30 เหลือ 3, `limited = true`; ช่อง 13:00 เหลือ 10, `limited = false` |
| A2 | เหลือ 4 จาก 10 | `limited = false` |
| A3 | เหลือ 0 | `limited = false`, `bookable = false` |
| A4 | ช่องที่เร็วกว่า now + lead | `bookable = false` |
| A5 | ร้านพักรับจอง | `accepting_bookings = false` และ `bookable = false` ทุกช่อง |
| A6 | ร้านถูกลบ | 404 |

### 10.6 Integration: ตั้งค่าร้าน

| # | กรณี | คาด |
|---|---|---|
| T1 | ลดที่นั่ง 10 → 8 ขณะที่อนาคตมี peak 10 | 409 พร้อมรายการการจองที่ชน |
| T2 | ลด 10 → 8 ขณะที่ peak 8 | 200 |
| T3 | เปลี่ยนเวลาเปิดจนการจองเดิมหลุดเวลา | 409 `reason = opening_hours` |
| T4 | `cancel_window_minutes = 20` / `min_lead = 20` กับ cancel 30 | 422 |
| T5 | ไม่ใช่เจ้าของร้าน | 403 |
| T6 | การจองที่ `end_at ≤ now` ไม่ถูกนับในการตรวจ | ✓ |
| T7 | `capacity` 0 หรือ 501, เวลาไม่ลงช่อง, เปิดเท่ากับปิด | 400 |
| T8 | ลดที่นั่งจนชนสองช่วงเวลา | 409 ส่งเฉพาะการจองของจุด peak แรก |

### 10.7 Integration: รีวิวและการเรียง

| # | กรณี | คาด |
|---|---|---|
| V1 | ด้านใดด้านหนึ่ง = 0, 6, 4.5 หรือส่งไม่ครบ 4 ด้าน | 400 |
| V2 | comment ว่างหรือมีแต่ช่องว่าง | 400 |
| V3 | รีวิวร้านตัวเอง | 403 |
| V4 | รีวิวร้านเดิมครั้งที่สอง | 409 |
| V5 | แก้หรือลบรีวิวของคนอื่น | 404 |
| V6 | คะแนน 4, 5, 4 | `avg_rating = 4.3`, `review_count = 3` |
| V7 | 5.0 จาก 1 รีวิว เทียบ 4.8 จาก 300 รีวิว | ร้าน 300 รีวิวอยู่บน |
| V8 | ร้านไม่มีรีวิว | อยู่ท้ายสุดใน highest_rated, `avg_rating = null` |
| V13 | คะแนน 4 ด้านไม่เท่ากัน (5, 4, 3, 4) | `rating = 4.00` ฐานข้อมูลคำนวณให้ · ส่งไม่ครบหรือนอกช่วง 400 |
| V14 | `aspect_averages` ในรายละเอียดร้าน | ค่าเฉลี่ยรายด้าน · `null` เมื่อร้านยังไม่มีรีวิว |
| V15 | `price_level` ไม่บังคับ · นอกช่วง 1–5 ได้ 400 · แก้และล้างได้ | — |
| V16 | ราคาของร้านใช้ **มัธยฐาน** ไม่ใช่ค่าเฉลี่ย | [2,2,3,5] → `2` (ค่าเฉลี่ยจะเป็น 3.0) · มีคนระบุน้อยกว่า 3 คน → `null` |
| V9 | ร้านที่ค่าทุกอย่างเท่ากัน | ลำดับเหมือนเดิมทุกครั้ง (ตาม `id`) |
| V10 | most_reviewed เมื่อจำนวนเท่ากัน | ค่าเฉลี่ยสูงกว่าอยู่บน |
| V11 | รายการรีวิว: `is_mine` ถูกต้องทั้งตอน login และไม่ login, แบ่งหน้าและ `total` ถูกต้อง | ✓ |
| V12 | ร้านที่ถูกลบไม่อยู่ในรายการร้าน และรีวิวใหม่ได้ 404 | ✓ |

### 10.8 Concurrency (ชื่อ test ขึ้นต้นด้วย `TestConcurrent`)

| # | กรณี | คาด |
|---|---|---|
| K1 | ร้าน 10 ที่ มีจองแล้ว 4 เหลือ 6 — ผู้ใช้ **20 คนที่ต่างกัน** จองคนละ 6 ที่ช่วงเดียวกันพร้อมกัน | สำเร็จ 1, ได้ 409 อีก 19, `PeakAll ≤ 10` |
| K2 | 50 goroutine จองแบบสุ่มขนาดและเวลาพร้อมกัน | `PeakAll ≤ capacity` หลังจบ |
| K3 | ผู้ใช้เดียวส่งรีวิวร้านเดียวกันพร้อมกัน 2 ครั้ง | มีรีวิว 1 แถว อีกคำขอได้ 409 |
| K4 | ลดที่นั่งพร้อมกับมีการจองเข้ามา | ค่าสุดท้าย: `PeakAll(อนาคต) ≤ capacity` |

วิธีทำ: สร้าง channel `start` ให้ทุก goroutine รอ แล้ว `close(start)` เพื่อปล่อยพร้อมกัน ใช้ `sync.WaitGroup` รอจบ และนับผลด้วย `sync/atomic`
(K1 ต้องใช้ผู้ใช้ต่างกัน ไม่อย่างนั้นกติกา `overlapping-own-booking` จะตัดสินก่อน และ test จะไม่ได้ทดสอบ race ของที่นั่ง)

### 10.9 Handler tests

ใช้ `httptest` กับ fake auth middleware ในไฟล์ `_test.go` ตรวจ status code และ `type` ของ problem ครบทุก `type` ที่ส่วนนี้ใช้ (ดู `00-overview.md` §5) อย่างน้อย type ละ 1 กรณี

### คำสั่ง

```
go test -p 1 ./...                      # ปกติบน Windows
./scripts/test-race.ps1                # ทั้งหมดแบบ -race ใน container
./scripts/test-race.ps1 -Concurrent    # TestConcurrent -count=100 -race
```

---

## 11. ลำดับการทำงาน

| ขั้น | งาน | จบเมื่อ |
|---|---|---|
| 1 | `docker-compose.yml` (postgres), `migrations/0001_init.sql` และ `0002_categories.sql` ตาม `00-overview.md` §4, `cmd/migrate`, `internal/testdb`, `db.WithTx`, `db.lockRestaurant`, `httpx` (problem+json, JSON เข้มงวด) ตาม `00-overview.md` §4 | migration รันบนฐานข้อมูลเปล่าได้ |
| 2 | `capacity.go`, `rules.go` + test 10.1, 10.2, 10.3 | test ผ่าน → **Checkpoint A: ผู้ใช้อธิบาย MaxLoad และ half-open ได้** |
| 3 | จองใหม่ (7.1) + S1–S8, S16 + K1, K2 | test ผ่าน → **Checkpoint B: ผู้ใช้อธิบายลำดับ transaction และเหตุผลที่ล็อกแถวร้านได้** |
| 4 | แก้ไข / ยกเลิก / การจองของฉัน (7.2–7.4) + S9–S15, S17–S18 | test ผ่าน |
| 5 | ที่ว่าง (7.5) + A1–A6 | test ผ่าน |
| 6 | ตั้งค่าร้าน (7.6) + T1–T8 + K4 | test ผ่าน |
| 7 | รีวิวและรายการร้าน (7.7–7.8) + V1–V12 + K3 | test ผ่าน → **Checkpoint C: ผู้ใช้อธิบาย Bayesian และ unique constraint ได้** |
| 8 | handlers ทั้งหมด + test 10.9 | test ผ่าน |
| 9 | ส่วน Design Decisions ของ README จาก §9 (README ฉบับเต็มทำในส่วนที่ 6) | ครบ |

---

## 12. Definition of Done ของส่วนที่ 1

- [ ] test ทุกข้อใน §10 ผ่าน รวม `-race`
- [ ] concurrency test ผ่านต่อเนื่อง 100 รอบ
- [ ] error ทุกกรณีตอบเป็น problem+json ที่มี `type` ตาม `00-overview.md` §5
- [ ] ตาราง Design Decisions ของส่วนนี้อยู่ใน README
- [ ] ผู้ใช้ผ่าน Checkpoint A, B, C โดยอธิบายได้โดยไม่ต้องเปิดโค้ด

---

## 13. เชื่อมกับส่วนอื่น

- ระหว่างทำส่วนนี้ `auth.UserID(ctx)` ยังเป็น stub ส่วนจริงอยู่ใน `02-auth.md`
- ข้อมูลร้านสำหรับ test ให้ INSERT ตรงลงฐานข้อมูลใน test helper (ฟอร์มสร้างร้านอยู่ใน `03-restaurants.md`)
- รายการระดับ C อยู่ใน `00-overview.md` §1
