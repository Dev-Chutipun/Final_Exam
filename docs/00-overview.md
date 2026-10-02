# 00 · ภาพรวมโปรเจกต์และข้อตกลงร่วม

**โปรเจกต์:** PEA DevPool 2026 Final Exam — ระบบจองโต๊ะร้านอาหารและ community รีวิวร้าน
**ฉบับ:** 1.2 (2 ต.ค. 2026 — ตอบคำถามข้อ 2.1–2.11 และย้ายมาเขียนและรันบน Windows เครื่องเดียว)
**สแตก:** Next.js 16 (frontend) · Go 1.25+ (backend API) · PostgreSQL · Keycloak (login)
**ส่งงาน:** อาทิตย์ 11 ต.ค. 2026 · **สัมภาษณ์:** 17–18 ต.ค. 2026
**UI Design (อ้างอิงหน้าตา):** https://claude.ai/artifact/3ZfLbyBdBdGCSA7vGzQSJo

ไฟล์นี้คือ **แหล่งความจริงเดียว** ของ schema, error, ข้อตกลงร่วม และลำดับงานรวม ไฟล์ส่วนอื่นอ้างถึงไฟล์นี้

| ไฟล์ | เนื้อหา |
|---|---|
| `docs/00-overview.md` | ไฟล์นี้ |
| `docs/01-booking-review.md` | ที่นั่ง การจอง แก้ไข ยกเลิก ที่ว่าง ตั้งค่าร้าน รีวิว รายการร้าน |
| `docs/02-auth.md` | Login ด้วย Keycloak, session, CSRF |
| `docs/03-restaurants.md` | สร้าง/แก้/ลบร้าน, รูป, หมวดหมู่, พักรับจอง, ค้นหา |
| `docs/05-frontend.md` | Next.js, หน้าเว็บ, design tokens โทนพาสเทลกระจก |
| `docs/prompts.md` | prompt สำหรับสั่งงานทีละขั้น |

---

## 1. ระดับขอบเขต

| ระดับ | งาน | กติกา |
|---|---|---|
| **A: ตามโจทย์ ต้องเสร็จ** | ที่นั่ง/จอง/แก้/ยกเลิก/การจองของฉัน, ที่ว่างรายช่อง + Limited Seats Left (แผงจองต้องใช้), differential test, ตรวจการจองอนาคตเมื่อแก้ที่นั่ง, รีวิว+การเรียง, login, สร้าง/แก้/ลบร้าน+รูป+หมวดหมู่, หน้าเว็บครบ flow, README, seed | ทำก่อนเสมอ |
| **B: ต้นทุนต่ำ** | ค้นหาด้วยชื่อและกรองตามหมวด, พักรับจอง, Owner ดูการจองรายวัน | ทำเมื่อระดับ A ของส่วนนั้นเสร็จ |
| **C: ไม่ทำ** | Login ด้วย Google, Idempotency-Key, กรองตามช่วงเวลาที่ว่าง, สั่งอาหารล่วงหน้า, โหมดโต๊ะ | เขียนไว้ใน README หัวข้อ Future work เท่านั้น |

ลำดับในตาราง "ลำดับการทำงาน" ของแต่ละเอกสารคือลำดับที่ต้องทำ งานระดับ B ในเอกสารติดป้าย **[B]** ไว้ทุกจุด

---

## 2. ข้อตกลงร่วม (ใช้ทุกส่วน)

**กติกาและความถูกต้อง**
1. กติกาธุรกิจทุกข้อตรวจที่ Go เสมอ หน้าเว็บตรวจได้เพื่อ UX เท่านั้น constraint ในฐานข้อมูลเป็นแนวป้องกันชั้นที่สอง
2. สิทธิ์ (เจ้าของร้าน, เจ้าของการจอง, ผู้เขียนรีวิว) ตรวจที่ API ทุกครั้ง ไม่ใช่แค่ซ่อนปุ่ม
3. ความเป็นเอกลักษณ์ใช้ unique constraint ตัดสิน ห้ามเช็คก่อนแล้วค่อย INSERT

**Transaction และการล็อก**
4. Read Committed (ค่าเริ่มต้น) ห้ามใช้ Repeatable Read หรือ Serializable ในงานที่ล็อกแถวร้าน
5. ทุก pipeline ที่อ่านค่าของร้านเพื่อตัดสินใจ ต้องเรียก helper กลาง `lockRestaurant(tx, id)` ซึ่งคือ `SELECT … FROM restaurants WHERE id = $1 AND deleted_at IS NULL FOR UPDATE` และอ่านค่าร้าน**หลัง**ล็อกเท่านั้น
6. ลำดับการล็อก: แถวร้าน → แถวการจอง

**เวลา**
7. Timezone ธุรกิจ `Asia/Bangkok` (UTC+7 ไม่มี DST) ใส่ `import _ "time/tzdata"`
8. เวลาในฐานข้อมูลเป็น `timestamptz` ใน API เป็น RFC 3339 พร้อม offset เช่น `2026-10-11T12:00:00+07:00`
9. ช่วงเวลาเป็นแบบ half-open `[start, end)` ทุกที่
10. ห้ามเรียก `time.Now()` ใน domain หรือ service ให้รับ `now time.Time` จาก handler

**API**
11. Route ขึ้นต้นด้วย `/api` · JSON ใช้ snake_case · decode ด้วย `DisallowUnknownFields` · จำกัด body ด้วย `http.MaxBytesReader` (JSON 1 MB, multipart ตามส่วนที่ 3)
12. Error ตอบเป็น `application/problem+json` (RFC 9457) ตามตารางในข้อ 5
13. ผู้ใช้ปัจจุบันอ่านผ่าน `auth.UserID(ctx) (int64, bool)` เท่านั้น
14. SQL เขียนเองผ่าน `pgx/v5` ห้ามใช้ ORM
15. decode JSON ไม่ผ่าน (ชนิดผิด เช่น `party_size: 4.5`, ฟิลด์ที่ไม่รู้จัก, JSON เสีย, body ว่าง หรือเกิน 1 MB) → 400 `validation` ใส่ `errors[].field` เมื่อหาได้จาก error ของ decoder (`*json.UnmarshalTypeError.Field` หรือข้อความ unknown field) ถ้าหาไม่ได้ให้ `errors` ว่างและอธิบายใน `detail`
16. การแบ่งหน้า (`page`, `page_size`) ในทุก endpoint รายการ: ค่าที่ไม่ใช่จำนวนเต็ม, `page < 1`, `page_size < 1` หรือ `page_size` เกินเพดานของ endpoint → 400 `validation` (ไม่ปรับค่าให้เอง) · ไม่ส่งมาใช้ค่าเริ่มต้น

---

## 3. โครงสร้าง repo

```
/
  CLAUDE.md
  README.md
  .gitattributes              # บังคับ LF
  .gitignore
  docker-compose.yml          # postgres + keycloak เท่านั้น, network ชื่อตายตัว devpool-net
  docker/postgres-init/01-test-db.sql   # CREATE DATABASE devpool_test (รันครั้งแรกที่สร้าง volume)
  scripts/test-race.ps1       # go test -race ใน container golang
  .env.example
  docs/
  keycloak/realm-devpool.json
  backend/
    cmd/api/main.go
    cmd/migrate/main.go       # รัน migrations ตามลำดับชื่อไฟล์
    cmd/seed/main.go          # seed ข้อมูลตัวอย่าง (ส่วนที่ 6)
    internal/
      auth/                   # OIDC, session, middleware, UserID(ctx)
      httpx/                  # problem+json, JSON เข้มงวด, multipart helper
      db/                     # pgxpool, WithTx, lockRestaurant
      booking/                # capacity.go, rules.go, service.go, handler.go
      restaurant/             # สร้าง/แก้/ลบ, settings, รูป, พักรับจอง
      review/                 # รีวิว, ranking (รายการร้าน)
      testdb/                 # helper ฐานข้อมูลสำหรับ integration test
      config/                 # อ่าน .env แบบง่าย (KEY=VALUE) ไม่เพิ่ม dependency
    migrations/
      0001_init.sql
      0002_categories.sql
  frontend/                   # Next.js 16 (ส่วนที่ 5)
```

| บริการ | พอร์ต |
|---|---|
| Next.js | 3000 |
| Go API | 8080 |
| Keycloak | 8081 (map จาก 8080 ใน container) |
| PostgreSQL | 5432 |

**เวอร์ชัน Go:** `backend/go.mod` ระบุ `go 1.26.5` ให้ตรงกับเครื่องที่รันจริง (`scripts/test-race.ps1` จึงใช้ image `golang:1.26.5`)

**Go รันบนเครื่องโดยตรง ไม่รันใน Docker** (ดู `02-auth.md` เรื่อง issuer) ยกเว้น `scripts/test-race.ps1` ที่ใช้ container เฉพาะตอนรัน test

**การอ่าน config:** `internal/config` อ่านไฟล์ `.env` ที่ root (ค้นหาจากโฟลเดอร์ปัจจุบันขึ้นไปจนเจอ) บรรทัดละ `KEY=VALUE` ข้ามบรรทัดว่างและ `#` · ตัวแปรที่ตั้งไว้ใน environment จริงมีสิทธิ์เหนือกว่าไฟล์ · Next.js ใช้ `frontend/.env.local` ของตัวเอง

**`scripts/test-race.ps1`:** รัน `go test -race -p 1 ./...` ใน image `golang` (tag ตรงกับเวอร์ชันใน `go.mod`) mount โฟลเดอร์ `backend/`, ต่อ network `devpool-net`, ตั้ง `TEST_DATABASE_URL` ให้ชี้ `postgres:5432/devpool_test`, ใช้ named volume เก็บ module cache · พารามิเตอร์ `-Concurrent` รันเฉพาะ `TestConcurrent` แบบ `-count=100`

**Environment (`.env.example`)**

| ตัวแปร | ตัวอย่าง | ใช้ที่ |
|---|---|---|
| `DATABASE_URL` | `postgres://devpool:devpool@localhost:5432/devpool?sslmode=disable` | Go |
| `TEST_DATABASE_URL` | `postgres://devpool:devpool@localhost:5432/devpool_test?sslmode=disable` | Go test |
| `PORT` | `8080` | Go |
| `PUBLIC_BASE_URL` | `http://localhost:3000` | Go (redirect URI, trusted origin) |
| `OIDC_ISSUER` | `http://localhost:8081/realms/devpool` | Go |
| `OIDC_CLIENT_ID` | `devpool-web` | Go |
| `OIDC_CLIENT_SECRET` | `devpool-local-secret` (ใช้บนเครื่องเท่านั้น) | Go |
| `COOKIE_SECURE` | `false` | Go |
| `API_ORIGIN` | `http://localhost:8080` | Next.js (ฝั่งเซิร์ฟเวอร์เท่านั้น) |

---

## 4. Schema รวม

`migrations/0001_init.sql`

```sql
CREATE TABLE users (
  id           BIGSERIAL PRIMARY KEY,
  issuer       TEXT NOT NULL,
  subject      TEXT NOT NULL,
  display_name TEXT NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (issuer, subject)
);

CREATE TABLE sessions (
  token_hash BYTEA PRIMARY KEY,                 -- SHA-256 ของ token
  user_id    BIGINT NOT NULL REFERENCES users(id),
  id_token   TEXT NOT NULL,                     -- ใช้ตอน logout เท่านั้น
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX sessions_user_idx ON sessions (user_id);

CREATE TABLE restaurants (
  id                    BIGSERIAL PRIMARY KEY,
  owner_id              BIGINT NOT NULL REFERENCES users(id),
  name                  TEXT NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 80),
  description           TEXT NOT NULL CHECK (char_length(btrim(description)) BETWEEN 1 AND 2000),
  capacity              INT  NOT NULL CHECK (capacity BETWEEN 1 AND 500),
  open_time             TIME NOT NULL,
  close_time            TIME NOT NULL,
  cancel_window_minutes INT  NOT NULL DEFAULT 30 CHECK (cancel_window_minutes >= 30),
  min_lead_minutes      INT  NOT NULL DEFAULT 120,
  accepting_bookings    BOOLEAN NOT NULL DEFAULT TRUE,
  deleted_at            TIMESTAMPTZ,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (open_time <> close_time),
  CHECK (EXTRACT(MINUTE FROM open_time)  IN (0, 30) AND EXTRACT(SECOND FROM open_time)  = 0),
  CHECK (EXTRACT(MINUTE FROM close_time) IN (0, 30) AND EXTRACT(SECOND FROM close_time) = 0),
  CHECK (min_lead_minutes >= cancel_window_minutes)
);
CREATE UNIQUE INDEX restaurants_owner_name_uq
  ON restaurants (owner_id, lower(btrim(name))) WHERE deleted_at IS NULL;
CREATE INDEX restaurants_owner_idx ON restaurants (owner_id);

CREATE TABLE categories (
  id         SMALLINT PRIMARY KEY,
  slug       TEXT NOT NULL UNIQUE,
  name_th    TEXT NOT NULL,
  sort_order SMALLINT NOT NULL
);

CREATE TABLE restaurant_categories (
  restaurant_id BIGINT   NOT NULL REFERENCES restaurants(id),
  category_id   SMALLINT NOT NULL REFERENCES categories(id),
  PRIMARY KEY (restaurant_id, category_id)
);

CREATE TABLE restaurant_images (
  id            BIGSERIAL PRIMARY KEY,
  restaurant_id BIGINT NOT NULL REFERENCES restaurants(id),
  content_type  TEXT   NOT NULL CHECK (content_type IN ('image/jpeg', 'image/png', 'image/webp')),
  data          BYTEA  NOT NULL CHECK (octet_length(data) BETWEEN 1 AND 2097152),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX restaurant_images_restaurant_idx ON restaurant_images (restaurant_id, id);

CREATE TABLE bookings (
  id              BIGSERIAL PRIMARY KEY,
  restaurant_id   BIGINT NOT NULL REFERENCES restaurants(id),
  user_id         BIGINT NOT NULL REFERENCES users(id),
  party_size      INT NOT NULL CHECK (party_size > 0),
  start_at        TIMESTAMPTZ NOT NULL,
  end_at          TIMESTAMPTZ NOT NULL,
  status          TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'cancelled')),
  cancel_deadline TIMESTAMPTZ NOT NULL,
  cancelled_at    TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (end_at - start_at BETWEEN INTERVAL '30 minutes' AND INTERVAL '3 hours'),
  CHECK (cancel_deadline <= start_at - INTERVAL '30 minutes'),
  CHECK ((status = 'cancelled') = (cancelled_at IS NOT NULL))
);
CREATE INDEX bookings_restaurant_start_idx ON bookings (restaurant_id, start_at);
CREATE INDEX bookings_user_start_idx       ON bookings (user_id, start_at);

CREATE TABLE reviews (
  id            BIGSERIAL PRIMARY KEY,
  restaurant_id BIGINT NOT NULL REFERENCES restaurants(id),
  user_id       BIGINT NOT NULL REFERENCES users(id),
  rating        SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
  comment       TEXT NOT NULL CHECK (char_length(btrim(comment)) BETWEEN 1 AND 1000),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, restaurant_id)
);
CREATE INDEX reviews_restaurant_idx ON reviews (restaurant_id);
```

`migrations/0002_categories.sql` (ข้อมูลอ้างอิงที่ต้องมีเสมอ)

| id | slug | name_th |
|---|---|---|
| 1 | thai | อาหารไทย |
| 2 | isan | อาหารอีสาน |
| 3 | vietnamese | อาหารเวียดนาม |
| 4 | japanese | อาหารญี่ปุ่น |
| 5 | chinese | อาหารจีน |
| 6 | korean | อาหารเกาหลี |
| 7 | italian | อาหารอิตาเลียน |
| 8 | steak-grill | สเต็กและกริลล์ |
| 9 | seafood | อาหารทะเล |
| 10 | buffet | บุฟเฟต์ |
| 11 | cafe-drinks | คาเฟ่และเครื่องดื่ม |
| 12 | dessert-bakery | ของหวานและเบเกอรี่ |

**การรัน migration:** `go run ./cmd/migrate` เขียนเองโดยไม่เพิ่ม dependency อ่านไฟล์ `.sql` ตามลำดับชื่อ บันทึกไฟล์ที่รันแล้วในตาราง `schema_migrations (filename TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL)` และรันแต่ละไฟล์ใน transaction เดียว

**ฐานข้อมูลสำหรับ test (`internal/testdb`):** รัน migrations บน `TEST_DATABASE_URL` ครั้งเดียวต่อ package แล้ว `TRUNCATE … RESTART IDENTITY CASCADE` ทุกตาราง **ยกเว้น `categories` และ `schema_migrations`** ก่อนแต่ละ test · รัน integration test ด้วย `go test -p 1` เพื่อไม่ให้หลาย package ล้างฐานข้อมูลชนกัน

หมายเหตุ
- ไม่ใช้ trigger กับ `updated_at` ให้ตั้งค่าในคำสั่ง UPDATE เอง
- ไม่มี `ON DELETE` ใดๆ เพราะร้านใช้ soft delete (`deleted_at`) และไม่มีการลบผู้ใช้
- การตรวจช่องเวลา 30 นาทีของการจอง, จำนวนหมวด 1–3 และจำนวนรูป 1–3 ตรวจที่ Go
- `btrim` ใน CHECK ตัดเฉพาะช่องว่าง ฝั่ง Go ใช้ `strings.TrimSpace`
- **query รายการใดๆ ห้ามดึงคอลัมน์ `restaurant_images.data`** ยกเว้น endpoint ส่งรูปโดยตรง

---

## 5. Error ทั้งระบบ (RFC 9457)

Header `Content-Type: application/problem+json` และ `type` ขึ้นต้นด้วย `/problems/`

```json
{ "type": "/problems/seat-capacity-exceeded", "title": "ที่นั่งไม่พอ", "status": 409,
  "detail": "ช่วง 12:30–13:00 เหลือ 3 ที่ แต่ขอ 5 ที่",
  "full_from": "2026-10-11T12:30:00+07:00", "available": 3 }
```

| `type` | Status | ใช้เมื่อ | ส่วน |
|---|---|---|---|
| `validation` | 400 | รูปแบบผิด มี `errors: [{ field, message }]` | ทุกส่วน |
| `unauthenticated` | 401 | ไม่มีผู้ใช้ | ทุกส่วน |
| `own-restaurant-booking` | 403 | จองร้านตัวเอง | 01 |
| `own-restaurant-review` | 403 | รีวิวร้านตัวเอง | 01 |
| `not-restaurant-owner` | 403 | จัดการร้านของคนอื่น | 01, 03 |
| `cross-origin-request` | 403 | คำขอเปลี่ยนข้อมูลจาก origin อื่น | 02 |
| `not-found` | 404 | ไม่พบ, ถูกลบ หรือไม่ใช่ของผู้ใช้ | ทุกส่วน |
| `image-too-large` | 413 | รูปเกิน 2 MB หรือ body เกินเพดาน | 03 |
| `unsupported-image-type` | 415 | ไม่ใช่ jpeg/png/webp (ตรวจจากเนื้อไฟล์) | 03 |
| `unsupported-media-type` | 415 | `Content-Type` ไม่ตรงกับที่ endpoint รับ | 02 |
| `booking-in-past` | 422 | เวลาเริ่มผ่านไปแล้ว | 01 |
| `lead-time-not-met` | 422 | จองล่วงหน้าไม่พอ | 01 |
| `too-far-in-advance` | 422 | เกิน 30 วัน | 01 |
| `outside-opening-hours` | 422 | นอกเวลาเปิด | 01 |
| `party-exceeds-capacity` | 422 | จำนวนคนเกินที่นั่งทั้งร้าน | 01 |
| `cancel-deadline-passed` | 422 | แก้หรือยกเลิกหลัง deadline | 01 |
| `invalid-restaurant-policy` | 422 | ค่านโยบายร้านผิดกติกา | 01, 03 |
| `overlapping-own-booking` | 409 | ผู้ใช้จองทับเวลาตัวเองในร้านเดียวกัน | 01 |
| `seat-capacity-exceeded` | 409 | ที่นั่งไม่พอ | 01 |
| `booking-not-active` | 409 | แก้การจองที่ยกเลิกแล้ว | 01 |
| `already-cancelled` | 409 | ยกเลิกซ้ำ | 01 |
| `already-reviewed` | 409 | รีวิวซ้ำ | 01 |
| `settings-conflict-with-bookings` | 409 | ตั้งค่าใหม่ชนการจองในอนาคต | 01 |
| `restaurant-not-accepting-bookings` | 409 | ร้านพักรับจอง | 01, 03 |
| `restaurant-name-taken` | 409 | ชื่อร้านซ้ำในร้านของเจ้าของคนเดียวกัน | 03 |
| `restaurant-has-upcoming-bookings` | 409 | ลบร้านที่ยังมีการจองในอนาคต | 03 |
| `image-limit-reached` | 409 | มีรูปครบ 3 แล้ว | 03 |
| `last-image` | 409 | ลบรูปสุดท้าย | 03 |
| `internal-error` | 500 | error ที่ไม่คาดคิด (เช่น ฐานข้อมูลล่ม) · ไม่เปิดเผยรายละเอียดใน response แต่ log error จริงที่เซิร์ฟเวอร์ | ทุกส่วน |

**หลักเลือก:** 400 รูปแบบผิด · 422 ผิดกติกาเสมอ · 409 ชนสถานะปัจจุบัน (อาจสำเร็จถ้าข้อมูลเปลี่ยน) — อิง RFC 9110

---

## 6. ลำดับงานรวมและเส้นเวลา

| วัน | เฟส | เอกสาร | Checkpoint (ผู้ใช้อธิบายกลับ) |
|---|---|---|---|
| ศ. 2 ต.ค. | โครง repo, compose, migration, `db`, `httpx`, pure functions | 01 ขั้น 1–2 | **A**: MaxLoad และ half-open |
| ศ.–ส. 2–3 ต.ค. | Spike Keycloak + Next.js ขั้นต่ำ (≤ 2 ชม.) | 02 §8 | — |
| ส.–อา. 3–4 ต.ค. | จอง แก้ ยกเลิก ที่ว่าง ตั้งค่าร้าน รีวิว handlers | 01 ขั้น 3–8 | **B**: transaction + ล็อกแถวร้าน · **C**: Bayesian + unique |
| จ. 5 ต.ค. | Login จริง (**จำกัด 1 วัน** ถ้าเกินให้ถอยเป็น email+password) | 02 | **D**: flow login, state/nonce/PKCE, session |
| จ.–อ. 5–6 ต.ค. | จัดการร้าน รูป พักรับจอง | 03 | — |
| อ.–พฤ. 6–8 ต.ค. | หน้าเว็บทั้งหมด · **หยุดเพิ่มฟีเจอร์ 8 ต.ค.** | 05 | **E**: Server/Client Component และการดึงข้อมูล |
| ศ. 9 ต.ค. | seed (อิงเวลาปัจจุบัน), README, clone ใหม่แล้วรันจากศูนย์ | ส่วนที่ 6 | — |
| ส. 10 ต.ค. | อัดวิดีโอ ≤ 5 นาที | ส่วนที่ 6 | — |
| อา. 11 ต.ค. | เวลาสำรอง และส่งงาน | — | — |

**ส่วนที่ 6 (ยังไม่ออกแบบละเอียด):** seed ต้องสร้างการจองโดยอิงเวลาปัจจุบัน, ร้านตัวอย่างครอบคลุมกรณีสำคัญ (มีการจองล่วงหน้า, ไม่มีการจอง, เปิดข้ามเที่ยงคืน, พักรับจอง), README ต้องมี Requirements · Design Decisions · วิธีรัน · บัญชีทดสอบ · Future work

---

## 7. README ต้องมี

1. วิธีรันตั้งแต่ clone (compose → migration → seed → Go → Next.js)
2. บัญชีทดสอบ (จาก `02-auth.md`)
3. **Requirements:** สิ่งที่ทำตามโจทย์ พร้อมอ้างเลขสไลด์
4. **Design Decisions:** รวมตาราง Design Decisions จาก 01, 02, 03, 05
5. **Future work:** รายการระดับ C
6. วิธีรัน test
