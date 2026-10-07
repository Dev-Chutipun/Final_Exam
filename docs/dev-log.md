# บันทึกการพัฒนา (Dev log)

บันทึกทุกอย่างที่เกิดขึ้นระหว่างพัฒนาโปรเจกต์ร่วมกับ Claude Code (2–8 ต.ค. 2026):
ลำดับงาน การตัดสินใจ ปัญหาที่เจอและวิธีแก้ ผลทดสอบ Checkpoint สำหรับเตรียมสัมภาษณ์ และสิ่งที่ยังค้าง

> เอกสารอ้างอิงหลัก: [CLAUDE.md](../CLAUDE.md) · [00-overview.md](00-overview.md) (แหล่งความจริงเดียว) · [architecture.md](architecture.md) · [openapi.yaml](openapi.yaml) · [README](../README.md)

---

## 1. สถานะล่าสุด (8 ต.ค. 2026)

| ส่วน | สถานะ |
|---|---|
| Backend ระดับ A + B (01, 02, 03) | ✅ เสร็จ · `go test`, `-race`, `-race -count=100` ผ่าน |
| Frontend ระดับ A + B (05) | ✅ เสร็จ · `npm run lint`, `npm run build` ผ่าน |
| Seed + README (ส่วนที่ 6) | ✅ เสร็จ · clone ใหม่แล้ว build ผ่าน |
| ทดสอบในเบราว์เซอร์ (13 ข้อ) | ✅ ผู้ใช้ทดสอบครบ ไม่มีข้อไม่ผ่าน |
| เอกสารสถาปัตยกรรม + ADR + OpenAPI | ✅ เสร็จ |
| Push ขึ้น GitHub | ✅ https://github.com/Dev-Chutipun/Final_Exam |
| แก้ผลรีวิวความพร้อม (§9) | ✅ แก้แล้ว 10 ข้อ (8 ต.ค.) · `govulncheck` และ `npm audit --omit=dev` สะอาด |
| บัญชีทดสอบใหม่ 8 บัญชี + seed 20 ร้าน (§9) | ✅ เสร็จ 8 ต.ค. · รูปจริงรอผู้ใช้ใส่ใน `seed-images/` |
| Checkpoint A, B | ✅ ทำแล้ว (มีจุดที่ต้องทวน §8) |
| Checkpoint C | ⏳ ตั้งคำถามแล้ว รอคำตอบ |
| Checkpoint D, E | ⏳ ยังไม่ได้ทำ |
| อัดวิดีโอ (10 ต.ค.) · ส่งงาน (11 ต.ค.) | ⏳ |

---

## 2. ลำดับงานและ commit

ทำตามลำดับใน `docs/prompts.md` (P0b → P13 → ส่วนที่ 6) ผู้ใช้ขอให้ "เขียนโค้ดให้ครบก่อน ค่อย checkpoint ทีหลัง" ตั้งแต่ P5

| commit | งาน |
|---|---|
| `42c17c4` | **P1** โครง repo, compose (postgres), migrations 0001–0002, `config`, `db` (WithTx, LockRestaurant, Migrate), `httpx`, `testdb`, `cmd/migrate`, `test-race.ps1` |
| `afdb850` | เพิ่ม problem type `internal-error` (500) · แก้ `test-race.ps1` บน PowerShell 5.1 |
| `ea2aa1f` | **P2** `capacity.go` (MaxLoad, PeakAll), `rules.go` + test 10.1–10.3 (differential 10,000 ชุด) |
| `544c90e` | **P3** Spike: Keycloak 26.8.0 + realm, Next.js ขั้นต่ำ, handler login/callback ชั่วคราว |
| `7135d8f` | เอกสาร: อนุญาตรหัส admin Keycloak ที่ใช้บนเครื่องใน compose |
| `eba96d0` | **P4** จองใหม่ §7.1 + S1–S8, S16, K1, K2 |
| `24c0cb8` | **P5** แก้ไข ยกเลิก การจองของฉัน ที่ว่าง (S9–S15, S17–S18, A1–A6) |
| `36343f8` | **P5** ตั้งค่าร้าน + T1–T8, K4 |
| `e86057c` | **P6** รีวิว + รายการร้าน Bayesian (V1–V12, K3) |
| `f91e370` | **P6** `cmd/api`, handler test ทุก problem type (10.9), README Design Decisions |
| `8958f15` | **P7** Login จริง, session, middleware, logout, CSRF 3 ชั้น (ลบ `cmd/spike`) |
| `06c95a6` | **P8** จัดการร้านระดับ A (W1–W8, E1–E2, R1–R2, I1–I4, D1–D5, K5–K7) |
| `e236d4d` | **P9** ระดับ B: พักรับจอง การจองรายวัน ค้นหา (P1–P3, B1, Q1, K8) |
| `1329d2f` | **P10** Tailwind 4 + shadcn/ui, tokens, ฟอนต์ local, layout, `lib/*` |
| `08dca4a` | **P11** หน้า `/`, `/restaurants/[id]` (แผงจองทุกสถานะ, รีวิว), `/bookings` |
| `dc991c2` | **P12** หน้าเจ้าของร้าน: ร้านของฉัน, ฟอร์มสร้าง/แก้, จัดการร้าน |
| `d30edf3` | **P13** ระดับ B หน้าเว็บ: ค้นหา, กรองหมวด, การจองรายวัน · ลบ `/styleguide` |
| `4246d8a` | **ส่วนที่ 6** `cmd/seed` + README ฉบับเต็ม |
| `e00ab5b` | `docs/architecture.md` + ADR 0001–0004 |
| `1da4319` | Graceful shutdown (12-Factor IX) |
| `febe4bf` | `docs/openapi.yaml` (OpenAPI 3.1, 26 endpoint) |
| `2916b05` | `docs/dev-log.md` (ไฟล์นี้) |
| `1bda5b1` | แก้ผลรีวิวความพร้อม §9: open redirect, Go 1.26.6 + x/text, 404 problem+json, timeout, global error, security headers, password policy, README |

---

## 3. การตัดสินใจที่ผู้ใช้เป็นผู้ตอบ

| เรื่อง | ทางที่เลือก |
|---|---|
| ราก repo | ย้าย `CLAUDE.md` และ `docs/` จาก `claude-code-kit/` ขึ้นมาที่ `E:\Devpool\Final_Exam\` |
| เวอร์ชัน Go | `go 1.26.5` ใน `go.mod` · image `golang:1.26.5` สำหรับ `test-race.ps1` |
| `page`/`page_size` ที่ผิด | ตอบ 400 `validation` ไม่ปรับค่าให้เอง (เขียนลง 00 §2 ข้อ 16) |
| error ที่ไม่คาดคิด | เพิ่ม `internal-error` 500 ไม่เปิดเผยรายละเอียด log ที่เซิร์ฟเวอร์ (เขียนลง 00 §5) |
| รหัส admin Keycloak (`admin`/`admin`) ใน compose | ยอมรับเป็นข้อยกเว้นเหมือน `OIDC_CLIENT_SECRET` (แก้ CLAUDE.md และ 02 §6) |
| `frontend/AGENTS.md`, `frontend/CLAUDE.md` ที่ Next สร้างเอง | เก็บไว้ |
| `backend/cmd/spike` | ลบหลังจบ spike |
| วิธีทำงาน | เขียนโค้ดให้ครบก่อน แล้วทำ Checkpoint ทีหลัง |
| งานต่อยอดจากวิชา Architecture | ทำ graceful shutdown, `openapi.yaml`, `architecture.md` + ADR |
| โฟลเดอร์ `01/` (เอกสารประกอบการเรียน) | ปล่อยไว้ก่อน ไม่ commit |

---

## 4. การตัดสินใจที่ทำระหว่างทาง (ควรรู้ไว้ตอบสัมภาษณ์)

| เรื่อง | เหตุผล |
|---|---|
| `db.LockRestaurant` ขึ้นต้นตัวใหญ่ (สเปกเขียน `lockRestaurant`) | ต้องเรียกจากหลาย package (`booking`, `restaurant`) |
| migration ฝังด้วย `go:embed` | `cmd/migrate` และ `testdb` ไม่ต้องหา path บนดิสก์ |
| กติกาคืน `*httpx.Problem` โดยตรง | service คืน error ที่ handler ส่งต่อได้ทันที (แลกกับ domain ผูกกับ httpx) |
| `title` ภาษาไทยของ problem ทุก type | สเปกไม่ได้กำหนด เขียนเอง (ผู้ใช้ยอมรับ) |
| ดาวน์เกรด TypeScript 7 → 6.0.3 และ ESLint 10 → 9.39.5 | `typescript-eslint` และ `eslint-plugin-react` ใน `eslint-config-next` ยังไม่รองรับ |
| shadcn ใช้แพ็กเกจ `cn` (ของ shadcn เอง) แทน clsx + tailwind-merge | เป็นค่าเริ่มต้นของ shadcn 4.x |
| overlay ของ Dialog ใช้ token ผ่าน `data-slot` ใน `globals.css` | ห้ามแก้ไฟล์ใน `components/ui/` |
| test V7 (Bayesian) เพิ่มร้านคะแนนกลางๆ | ร้าน 300 รีวิวชนะร้าน 1 รีวิวได้ก็ต่อเมื่อ m < 4.76 (ดู §8 Checkpoint C) |
| `GET /api/categories` และ `/api/me/restaurants` ตอบเป็น array | สเปกไม่ได้กำหนดรูปแบบ และไม่แบ่งหน้า |
| สวิตช์พักรับจองอยู่ในหน้าจัดการร้านตั้งแต่ขั้น 6 | 05 §4 ระบุไว้ใน "จัดการร้าน" |
| `seed` ปฏิเสธถ้ามีร้านอยู่แล้ว ต้องใช้ `-reset` | กันการล้างข้อมูลโดยไม่ตั้งใจ |
| `notFound()`/`redirect()` ตอบ HTTP 200 | `loading.tsx` ทำให้สตรีมหน้าแล้ว เบราว์เซอร์ยังแสดง/redirect ถูกต้อง |
| ไม่ทำ `/healthz` และ nginx `limit_req` | ยังเป็น Stage 0–1 เขียนไว้เป็นแผน Stage 2 ใน architecture.md |

---

## 5. ปัญหาสภาพแวดล้อมที่เจอและวิธีแก้

| อาการ | สาเหตุ | วิธีแก้ |
|---|---|---|
| Docker ต่อ daemon ไม่ได้ / Rancher ขึ้น `wsl.exe exited with code 4294967295` | **VT-x ปิดใน BIOS** (`HCS_E_HYPERV_NOT_INSTALLED`) | HP Pavilion: Esc → F10 → Configuration → Virtualization Technology = Enabled แล้ว `wsl --install --no-distribution` |
| Rancher Desktop ไม่เปิดเองหลังรีบูต | ค่าเริ่มต้นของ Rancher | เปิดเอง หรือ `rdctl start` แล้วรอ `docker info` |
| `test-race.ps1` พังเพราะ `go: downloading …` | PowerShell 5.1 + `$ErrorActionPreference = 'Stop'` แปลง stderr ของ docker เป็น error | เอาบรรทัดนั้นออก ใช้ exit code ตัดสิน |
| test ล้มแปลกๆ (แถวหายระหว่าง test) | container รอบก่อนยังรัน test ค้าง ล้างฐาน test ชนกัน | ห้ามรัน `go test` บนเครื่องพร้อมกับ `test-race.ps1` |
| `go test` ขึ้น `(cached)` หลังสร้าง `.env` | Go cache ไม่รู้ว่า `.env` เปลี่ยน | ใส่ `-count=1` |
| `package.json` parse ไม่ได้ | PowerShell `Set-Content -Encoding utf8` ใส่ BOM | เขียนไฟล์ใหม่ไม่มี BOM และล้าง `.next` |
| `Cannot find module '../lightningcss.linux-x64-gnu.node'` | รัน `npm run dev` ใน **WSL** แต่ `node_modules` ติดตั้งจาก Windows | รันใน Windows PowerShell เท่านั้น และลบ `.next` ที่ build จาก Linux |
| `npm.ps1 cannot be loaded because running scripts is disabled` | Execution Policy ของเครื่อง | ใช้ `npm.cmd run dev` หรือ `Set-ExecutionPolicy -Scope CurrentUser RemoteSigned` · สคริปต์ใช้ `powershell -ExecutionPolicy Bypass -File …` |
| `docker run -v E:\…` ขึ้น `mkdir /mnt/e: file exists` | Rancher Desktop เสียหลังใช้ WSL (Ubuntu) ร่วม | Quit Rancher → `wsl --shutdown` → เปิด Rancher ใหม่ → `docker compose up -d` |
| ภาษาไทยเป็น `???` เมื่อส่งผ่าน `curl.exe -F` | argument ของ command line บน Windows ถูกแปลงเป็น ANSI | ส่ง JSON จากไฟล์ UTF-8 (`--data-binary @file`) |
| localhost:3000 ไม่ขึ้นหลังรีสตาร์ท Docker | หน้าต่างที่รัน Go/Next ถูกปิดไปด้วย | เปิด `go run ./cmd/api` และ `npm.cmd run dev` ใหม่ |
| PDF สไลด์ภาษาไทยดึงเป็นข้อความไม่ได้ | ฟอนต์ไม่มีตาราง ToUnicode | เรนเดอร์เป็นภาพด้วย `Windows.Data.Pdf` แล้วอ่านจากภาพ |

---

## 6. บั๊กที่เจอระหว่างพัฒนา (แก้แล้ว)

| บั๊ก | สาเหตุ | แก้อย่างไร |
|---|---|---|
| การจองของฉัน แท็บ `cancelled` ได้ SQLSTATE 42P18 | query ไม่ได้ใช้ `$2` (now) PostgreSQL อนุมานชนิดไม่ได้ | คำนวณ `is_past = end_at <= $2` ใน SELECT ทุกแท็บ |
| helper ใน test ได้ 42P08 | ใช้ `$4 - interval …` ทำให้ชนิดของ `$4` ขัดกัน | คำนวณ deadline ใน Go แล้วส่งเป็นพารามิเตอร์ |
| K4 อาจ data race | หลาย goroutine ใช้ `rand.Rand` ตัวเดียว | สุ่มค่าทั้งหมดก่อนปล่อย goroutine |
| `bool_or` คืน NULL เมื่อร้านไม่มีรูป | aggregate บนชุดว่าง | `COALESCE(bool_or(...), false)` |
| ช่องค้นหาหลุดโฟกัสระหว่างพิมพ์ | `key={q}` ทำให้ remount ทุกครั้งที่ URL เปลี่ยน | ไม่ใช้ key แต่ซิงก์ค่าจาก URL เฉพาะเมื่อไม่ตรงกับที่พิมพ์ |
| พรีวิวรูปหายในฟอร์มสร้างร้าน | cleanup ของ `useEffect` revoke URL ทุกครั้งที่ state เปลี่ยน | revoke เฉพาะตอนกดลบรูป |
| React Compiler ข้ามการ optimize ฟอร์มจอง | `useMemo` ที่เขียนเอง | เอา `useMemo` ออก (คำนวณเบา) และรวมเป็น `endsFrom()` |

---

## 7. หลักฐานความถูกต้อง (ใช้ตอบสัมภาษณ์)

**Mutation test ของ `capacity.go`** — แอบทำให้ผิดแล้ว test จับได้ทุกแบบ:

| mutation | test ที่ล้ม |
|---|---|
| ใช้ช่วงปิด `t <= End` แทน half-open | TestMaxLoad, Differential |
| PeakAll เรียง event เริ่มก่อนจบ | TestPeakAll (P2 ได้ 20 แทน 10), Differential |
| MaxLoad ตรวจแค่จุด `s` | TestMaxLoad (C7), Differential |
| เท่ากันแล้วเลือกจุดสุดท้าย | TestMaxLoad, Differential |

**Mutation test ของการล็อก** (K1/K2):

| mutation | ผล |
|---|---|
| เอา `FOR UPDATE` ออก | จองเกิน PeakAll 12–19 ในร้าน 10 ที่ |
| ใช้ Repeatable Read | K1 สำเร็จ 8 จาก 20 คำขอ · **PeakAll 52 ในร้าน 10 ที่** |

**Spike Keycloak** ผ่านครบ 8 ข้อ (ทั้งด้วยคำสั่งและเบราว์เซอร์): iss ตรง `OIDC_ISSUER`, sub ตรงไฟล์ realm,
rewrite ส่ง 302 + Set-Cookie ได้, logout ไม่มีหน้ายืนยัน, CrossOriginProtection ผ่าน rewrite, multipart 3 × 2 MB ผ่าน (SHA-256 ตรง)
· พบว่า Next rewrite ส่ง `Host=localhost:8080` ให้ Go จึงต้อง `AddTrustedOrigin(PUBLIC_BASE_URL)`

**Graceful shutdown** ทดสอบใน container: ส่ง SIGTERM → "ปิด API เรียบร้อย" exit 0

---

## 8. Checkpoint (เตรียมสัมภาษณ์)

### Checkpoint A — MaxLoad, PeakAll, half-open ✅ (มีจุดต้องทวน)

| คำถาม | คำตอบผู้ใช้ | ผลตรวจ / คำตอบที่ครบ |
|---|---|---|
| 1. ร้าน 10 ที่ มีจอง 11:00–12:30 จำนวน 8 ขอ 12:00–13:00 จำนวน 3 | error, available = 2 | ✅ บางส่วน · ต้องระบุ **409 `seat-capacity-exceeded`**, `full_from` = **12:00** · จุดตรวจคือ **`s` และ Start ที่อยู่ใน (s, e)** กรณีนี้มีแค่ 12:00 (การจอง 11:00 ไม่ใช่จุดตรวจ แต่ถูกนับที่จุด 12:00) |
| 2. ทำไมไม่ต้องตรวจเวลาจบ และการจองที่เริ่มก่อน `s` นับได้อย่างไร | โหลดเพิ่มเมื่อมีการจอง ลดเมื่อจบ | ✅ ครึ่งหนึ่ง · **ทวน:** ที่เวลาจบโหลดมีแต่ลด จึงไม่สูงกว่าจุดก่อนหน้า · `s` อยู่ในจุดตรวจเสมอ และ `loadAt(s)` นับทุกการจองที่ `Start ≤ s < End` |
| 3. ถ้าใช้ช่วงปิด `[start, end]` | เวลาจะชนกัน | ✅ ทิศทางถูก · **ทวน:** ร้านเต็มพอดีรับจองต่อกันพอดีไม่ได้ (13:00 นับซ้อน 20), คนเดียวจอง 12–13 ต่อ 13–14 ไม่ได้, PeakAll รายงาน 20 ทำให้ลดที่นั่งได้ 409 ผิดๆ |

### Checkpoint B — transaction ของการจองใหม่ ✅ (มีจุดต้องทวน)

แกนสำคัญ: **ล็อก → อ่าน → ตัดสิน → เขียน** · ล็อกแถวร้านเพราะการจองใหม่ยังไม่มีแถว (phantom) · อ่านค่าร้านหลังล็อก · ห้าม Repeatable Read เพราะ snapshot ถูกจับก่อนได้ lock

| คำถาม | คำตอบผู้ใช้ | ผลตรวจ / คำตอบที่ครบ |
|---|---|---|
| 1. ย้าย `LockRestaurant` ไปหลังการนับที่นั่ง | ไม่เข้าใจ (ขอคำอธิบาย) | **กันไม่ได้** · A และ B นับพร้อมกันเห็นเหลือ 6 ทั้งคู่ → A ได้ lock จองแล้ว commit → B ได้ lock แต่ตัดสินไปแล้วจากตัวเลขเก่า → 4 + 6 + 6 = 16 ในร้าน 10 ที่ (check-then-act race) · ⏳ ยังไม่ได้ตอบซ้ำด้วยคำพูดตัวเอง |
| 2. ทำไมยกเลิกไม่ต้องล็อกร้าน | ไม่ต้องต่อคิว ไม่ต้องรอ | ✅ ข้อสรุปถูก · **เหตุผลที่ต้องพูด:** ยกเลิกมีแต่ทำให้ที่นั่งว่างขึ้น ไม่ต้องใช้ค่าร้าน (deadline อยู่ในแถวการจอง) · เงื่อนไขอยู่ใน UPDATE คำสั่งเดียว PostgreSQL ล็อกแถวการจองให้ · ยกเลิกซ้ำพร้อมกัน → ครั้งที่สองได้ 409 `already-cancelled` |
| 3. ร้าน A กับ B จองพร้อมกันร้านละ 100 คน / ถ้าใช้ `LOCK TABLE bookings` | ต้องรอ | ❌ **ร้าน A ไม่ต้องรอร้าน B** lock อยู่ที่แถวของแต่ละร้าน ร้านเดียวกันเท่านั้นที่ต่อคิว · `LOCK TABLE` ทำให้ทุกร้านในระบบต่อคิวเดียวกัน |

### Checkpoint C — Bayesian average และ unique constraint ⏳ รอคำตอบ

สรุปเนื้อหา:
- score = (C × m + SUM(rating)) / (C + COUNT) โดย C = 5 = "แถมรีวิวสมมติ 5 รีวิวที่ได้คะแนนเท่าค่าเฉลี่ยทั้งระบบ" ร้านรีวิวน้อยถูกดึงเข้าหา m
- m = ค่าเฉลี่ยรีวิวของร้านที่ยังไม่ถูกลบ (ไม่มีรีวิวเลยใช้ 3.0) · ร้านไม่มีรีวิว score เป็น NULL อยู่ท้าย · `id` เป็นตัวตัดสินสุดท้ายเพื่อให้แบ่งหน้าได้ถูก
- กันรีวิวซ้ำด้วย `UNIQUE (user_id, restaurant_id)`: INSERT เลย ถ้าได้ 23505 → 409 `already-reviewed` · ถ้า SELECT ก่อนแล้ว INSERT คำขอพร้อมกัน 2 ครั้งจะได้ 2 รีวิว (K3 ยืนยัน)

คำถามที่ยังรอคำตอบ:
1. ร้าน A ได้ 5 ดาวจาก 1 รีวิว ร้าน B เฉลี่ย 4.5 จาก 20 รีวิว ถ้า m = 4.0 ร้านไหนอยู่บน (แสดงวิธีคิด)
2. ทำไม m ต้องไม่นับรีวิวของร้านที่ถูกลบ และถ้า C = 100 แทน 5 ผลการเรียงเปลี่ยนอย่างไร
3. ทำไมกันรีวิวซ้ำใช้ unique constraint ได้ แต่กันจองเกินที่นั่งใช้ไม่ได้

### Checkpoint D, E ⏳ ยังไม่ได้ทำ

- **D:** flow login ตั้งแต่กดปุ่มจนได้ session, หน้าที่ของ state/nonce/PKCE, session เก็บที่ไหน, กัน CSRF อย่างไร
- **E:** หน้าไหนเป็น Server/Client Component เพราะอะไร, cookie เดินทางจาก browser ถึง Go อย่างไร, ทำไมไม่ใช้ Server Actions

---

## 9. รีวิวความพร้อม (ตรวจ 7 ต.ค. · แก้ 8 ต.ค. 2026)

ตรวจด้วย grep กติกา, test พิสูจน์ช่องโหว่, `govulncheck` และ `npm audit` · ลูน่าตรวจซ้ำเมื่อ 8 ต.ค. ยืนยันว่ายังอยู่ทุกข้อก่อนแก้

| ระดับ | # | จุด | สถานะ / วิธีแก้ |
|---|---|---|---|
| 🔴 | 1 | **Open redirect หลัง login** — `return_to=/%09/evil.com` ได้ `Location: /\t/evil.com` เบราว์เซอร์ตัด tab → `//evil.com` | ✅ `SafeReturnTo` ปฏิเสธอักขระควบคุมและช่องว่าง · test ใหม่ 7 กรณี**พังกับโค้ดเดิมก่อน**แล้วผ่านหลังแก้ · ทดสอบ login จริง: `/%09/evil.com` → `/` |
| 🔴 | 2 | Dependency ฝั่ง Go มีช่องโหว่ 7 รายการ | ✅ Go 1.26.5 → **1.26.6** (`go.mod`, image `golang:1.26.6`) · `golang.org/x/text` v0.29.0 → **v0.42.0** (ระหว่างแก้พบช่องโหว่ใหม่ GO-2026-6629 ใน v0.39.0 จึงขึ้นเป็นรุ่นล่าสุด) · `govulncheck`: **No vulnerabilities found** |
| 🟡 | 3 | route ที่ไม่มีอยู่ตอบ `text/plain` | ✅ `mux.HandleFunc("/api/", httpx.NotFound)` → 404 `/problems/not-found` (รวม method ที่ไม่รองรับ) · มี test |
| 🟡 | 4 | `http.Server` ไม่มี timeout | ✅ `ReadHeaderTimeout: 10 * time.Second` |
| 🟡 | 5 | ถ้า API ล่ม ทั้งเว็บพัง | ✅ header จับ error แล้วแสดงป้าย "เชื่อมต่อเซิร์ฟเวอร์ไม่ได้" แต่ละหน้าใช้ `error.tsx` ของตัวเอง · เพิ่ม `app/global-error.tsx` · ทดสอบโดยปิด API: หน้าเว็บตอบ 200 พร้อมป้าย |
| 🟢 | 6 | `npm audit` high 7 รายการ (มาจาก `shadcn` CLI) | ✅ ย้าย `shadcn` ไป `devDependencies` · `npm audit --omit=dev`: **found 0 vulnerabilities** |
| 🟢 | 7 | ไม่มี security header | ✅ `next.config.ts` ส่ง `X-Frame-Options: DENY`, `frame-ancestors 'none'`, `nosniff`, `Referrer-Policy` ทุกหน้า |
| 🟢 | 8 | ไม่จำกัดจำนวนการจองต่อคน | ⏸ ไม่แก้ สเปกไม่ได้กำหนด เป็นแผน Stage 2 (rate limit) ใน architecture.md |
| 🟢 | 9 | Keycloak สมัครได้โดยไม่มีกฎรหัสผ่าน | ✅ `passwordPolicy: length(8) and notUsername` · สร้าง container Keycloak ใหม่ให้ import realm แล้วยืนยันผ่าน Admin API |
| 🟢 | 10 | `id_token` เก็บใน DB แบบไม่ hash | ⏸ ยอมรับ ใช้แค่ตอน logout และหมดอายุเร็ว |
| 🟢 | 11 | `COOKIE_SECURE=false` | ✅ README อธิบายว่าใช้ได้เพราะเป็น localhost ถ้า HTTPS ต้องเป็น `true` |
| 📋 | 12 | ข้อมูล seed เก่า | ⏳ รัน `go run ./cmd/seed -reset` ก่อนอัดวิดีโอ |
| 📋 | 13 | README ไม่บอกเรื่อง Windows | ✅ เพิ่มหมายเหตุ: ใช้ PowerShell ไม่ใช่ WSL, `npm.cmd`/ExecutionPolicy, เปิด VT-x |
| 📋 | 14 | repo ไม่สะอาด | ⏳ `01/` ยังไม่ track (ผู้ใช้ให้ปล่อยไว้) · `frontend/next-env.d.ts` ต่างแค่ line ending |

**ผลทดสอบหลังแก้:** `go test -p 1 -count=1 ./...` ผ่าน · `test-race.ps1` (golang:1.26.6) ผ่าน · `-Concurrent -count=100` ผ่าน ·
`npm run lint` / `npm run build` ผ่าน · ทดสอบผ่าน Next + Keycloak จริง: header ความปลอดภัยมาครบ, route ที่ไม่มีอยู่ได้ problem+json, login ยังใช้ได้

**ตรวจแล้วไม่มีปัญหา:** SQL injection, IDOR (สิทธิ์ครบทุก endpoint), CSRF 3 ชั้น, การตรวจชนิดรูป, ไม่มี `.env` ใน git,
ไม่มี `toISOString` / `proxy.ts` / Server Actions / `time.Now()` ใน service / Repeatable Read / ดึง `restaurant_images.data` ในรายการ / ใช้ header `Host` / สีนอก token

**ปัญหาสภาพแวดล้อมที่เจอระหว่างแก้:** mount ไดรฟ์ E: ของ Rancher Desktop เสียอีกครั้ง (ทั้งที่ไม่ได้ใช้ WSL) ทำให้ Keycloak start ไม่ขึ้น
แก้ด้วย `rdctl shutdown` → `wsl --shutdown` → `rdctl start` → `docker compose up -d` (ข้อมูล postgres ไม่หาย)

---

### บัญชีทดสอบและข้อมูลตัวอย่างชุดใหม่ (8 ต.ค. 2026)

ผู้ใช้ขอเปลี่ยนบัญชีทดสอบและเพิ่มข้อมูลตัวอย่าง:

- **บัญชี Keycloak:** เปลี่ยน alice/bob/carol เป็น 8 บัญชี รหัสผ่าน `homsaard` ทุกบัญชี
  - `chutipun` เป็นลูกค้าอย่างเดียว
  - `ownertest` เป็นเจ้าของร้านอย่างเดียว
  - `usertest` เป็นทั้งเจ้าของร้านและลูกค้า (บทบาทผสมแบบที่ alice/bob/carol เคยเป็น)
  - เพิ่ม `somchai`, `nida`, `arthit` (เจ้าของร้าน) และ `somsri`, `kanya` (ลูกค้า)
  - ใช้ `id` ตายตัวชุดเดิมรูปแบบ `NNNNNNNN-...` เพื่อให้ seed ผูกร้านกับเจ้าของได้
- **Keycloak เก็บ username เป็นตัวพิมพ์เล็ก:** `Chutipun` ถูกเก็บเป็น `chutipun` แต่ login ด้วยตัวใหญ่ได้ ตรวจด้วยฟอร์ม login จริงผ่าน curl แล้ว
- **seed:** เพิ่มจาก 6 เป็น 20 ร้าน ครบทั้ง 12 หมวด มีการจอง 43 รายการและรีวิว 61 รายการ
  - ยังคงกรณีเดโมเดิมไว้ครบ: มีการจองล่วงหน้า (ลบไม่ได้), ไม่มีการจอง (ลบได้), ข้ามเที่ยงคืน, พักรับจอง, รีวิวเดียว 5 ดาว (เดโม Bayesian)
  - เพิ่มกรณีร้านใหม่ที่ไม่มีรีวิว
  - `book()` ตรวจว่าเจ้าของไม่ได้จองร้านตัวเอง
  - `TestSeedData` ตรวจกติกาข้อมูลร้าน
- **เมนูอาหาร:** schema ไม่มีตารางเมนู และ CLAUDE.md ห้ามเพิ่มตารางโดยไม่ถาม จึงไม่เปลี่ยน schema
  - ใส่ "เมนูแนะนำ" เป็นรายการในรายละเอียดร้าน (หน้าร้านใช้ `whitespace-pre-line` จึงขึ้นบรรทัดตามข้อความ)
  - รูปเมนูใช้ช่องรูปร้านที่มีอยู่ ร้านละ 3 รูป: ปก 1 + เมนู 2
- **รูปจริง:** ไม่ได้ดาวน์โหลดจากเว็บเพราะติดลิขสิทธิ์
  - สร้าง `seed-images/<ลำดับ-key>/cover/` และ `/menu/` ให้ผู้ใช้ใส่รูปเอง
  - seed อ่านรูปด้วย `httpx.CheckImage` ซึ่งใช้กฎเดียวกับการอัปโหลด (แยกออกจาก `ReadImage`)
  - ช่องที่ไม่มีรูปใช้ภาพไล่สีแทน

## 10. เนื้อหาวิชา Architecture & Scaling (โฟลเดอร์ `01/`) และการนำมาใช้

เอกสารประกอบการเรียนที่ได้หลังรับโจทย์ (ไม่ได้ commit เข้า repo):

- **สไลด์ (49 หน้า):** 12-Factor App · Scale จาก 1 ถึง 1 ล้าน user (Stage 0–6) · Architecture Design (quality attributes, monolith vs microservices, C4, ADR) · 8 System Design Patterns
- **Lab guide:** แล็บ Docker กับแอป todos (สลับ SQLite/MySQL ด้วย env, scale 3 replicas หลัง nginx ให้เห็นปัญหา stateful)
- **สไลด์ 47:** แบบฝึกหัด "ออกแบบระบบจองโต๊ะร้านอาหาร + รีวิวให้รองรับทั้งประเทศ" — **โจทย์เดียวกับ Final Exam**
- **สไลด์ 48 (การบ้าน):** เขียน `docs/architecture.md` + ADR อย่างน้อย 2 ข้อสำหรับ Final Exam
- **สไลด์ 46:** กรอบ 4 ขั้นตอบสัมภาษณ์: ถามโจทย์ → ประมาณตัวเลข → วาด C4 level 2 → เจาะจุดยาก + trade-off
- **เกณฑ์ของอาจารย์:** "เหตุผลสำคัญกว่าจำนวนกล่อง กลุ่มที่บอกว่า 'ยังไม่ต้องมี X เพราะ…' ได้คะแนนเท่ากับกลุ่มที่ใส่ X ถูกที่"
- **รูปภาพ:** ภาพ C4 level 2 ของคำตอบแบบฝึกหัด (LB + rate limit, Core API modular monolith, PostgreSQL primary + replicas, Redis, S3 + CDN, outbox → queue → worker → LINE, SSE) เส้นสีส้มเลข 4 คือเส้นทางจองช่วงพีค
- **ไฟล์ OpenAPI ตัวอย่าง:** สเปก API ของอีกระบบหนึ่ง ใช้เป็นตัวอย่างรูปแบบการเขียน

สิ่งที่นำมาใช้แล้ว: `docs/architecture.md` (ภาพตอนนี้ + ภาพเป้าหมาย + ตัวเลขประมาณ + สิ่งที่ตั้งใจยังไม่ทำ),
ADR 0001–0004, เช็กลิสต์ 12-Factor (ผ่านครบหลังเพิ่ม graceful shutdown), `docs/openapi.yaml`

ประโยคสำคัญตอนสัมภาษณ์: คอขวดแรกเมื่อโตคือ**การอ่านและรูป ไม่ใช่การจอง** เพราะ lock แยกตามร้าน ·
API เป็น stateless จึงเพิ่มเครื่องได้โดยไม่แก้ logic · ที่ว่างอ่านจาก replica/cache ได้ แต่การจองต้องไป primary เพราะต้องล็อก

---

## 11. วิธีรันบนเครื่องนี้ (Windows)

```powershell
# 0. ถ้า docker info ไม่ตอบ: เปิด Rancher Desktop ก่อน (VT-x ต้องเปิดใน BIOS แล้ว)
cd E:\Devpool\Final_Exam
docker compose up -d

# 1. API (Windows PowerShell เท่านั้น ห้ามใช้ WSL)
cd backend
go run ./cmd/migrate
go run ./cmd/seed -reset        # ข้อมูลเดโมอิงเวลาปัจจุบัน
go run ./cmd/api                # Ctrl+C ปิดแบบ graceful

# 2. หน้าเว็บ (อีกหน้าต่าง)
cd ..\frontend
npm.cmd run dev                 # http://localhost:3000

# 3. Test
cd ..\backend; go test -p 1 -count=1 ./...
cd ..; powershell -ExecutionPolicy Bypass -File .\scripts\test-race.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\test-race.ps1 -Concurrent
```

บัญชีทดสอบ: `chutipun` / `ownertest` / `usertest` และอีก 5 บัญชี รหัส `homsaard` ทุกบัญชี (บทบาทอยู่ใน README)

---

## 12. สิ่งที่ยังค้าง

1. ~~แก้ผลรีวิวความพร้อม §9~~ ✅ เสร็จ 8 ต.ค.
2. ตอบ Checkpoint C · ทำ Checkpoint D, E · ทวนจุดที่ขาดของ A และ B
3. ใส่รูปปก/เมนูใน `seed-images/` (ไม่บังคับ) แล้ว `go run ./cmd/seed -reset` และอัดวิดีโอ ≤ 5 นาที (10 ต.ค.)
4. ตัดสินใจเรื่องโฟลเดอร์ `01/` (`.gitignore` หรือย้ายออก)
5. ส่งงาน (11 ต.ค.)
