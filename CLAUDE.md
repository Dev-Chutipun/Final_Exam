# CLAUDE.md — DevPool Final Exam (ระบบจองโต๊ะร้านอาหาร)

## โปรเจกต์
ระบบจองโต๊ะร้านอาหารและรีวิวร้าน: Next.js 16 + Go 1.25+ + PostgreSQL + Keycloak
ส่งงาน 11 ต.ค. 2026 · ผู้พัฒนาต้องอธิบายโค้ดส่วนสำคัญได้เองในวิดีโอและสัมภาษณ์ **โค้ดต้องเรียบง่ายพอที่จะอธิบายได้**

## สภาพแวดล้อม (Windows เครื่องเดียว ทั้งเขียนและรัน)
- Shell หลักคือ PowerShell (มี Git Bash ถ้าติดตั้ง Git for Windows) · เรียก `curl.exe` ไม่ใช่ `curl` เพื่อเลี่ยง alias ของ PowerShell
- จบทุกขั้น: `go build ./...`, `go vet ./...` และ test ของขั้นนั้นต้อง**รันจริงและผ่าน** · **ห้ามบอกว่าผ่านถ้ายังไม่ได้รัน**
- `-race` บน Windows ต้องใช้ gcc ซึ่งโปรเจกต์นี้ไม่ลง ให้ใช้ `scripts/test-race.ps1` (รัน test ใน container `golang` ของ Docker) แทน
- งานที่ต้องใช้เบราว์เซอร์ (login, หน้าเว็บ) ให้เขียนขั้นตอนทดสอบด้วยมือให้ผู้ใช้ทำ แล้วรอผล
- ไฟล์ทั้งหมดใช้ LF ตาม `.gitattributes` · ใช้ `filepath` ในโค้ด Go ไม่เขียน path ตายตัว
- ถ้าพอร์ต 5432, 8080, 8081 หรือ 3000 ถูกใช้อยู่ ให้แจ้งผู้ใช้ก่อน ห้ามเปลี่ยนพอร์ตเอง

## เอกสาร (อ่านก่อนเริ่มงานทุกครั้ง)
- `docs/00-overview.md` — schema รวม, error ทั้งระบบ, ข้อตกลงร่วม, ลำดับงาน (**แหล่งความจริงเดียว**)
- `docs/01-booking-review.md` — ที่นั่ง การจอง ที่ว่าง ตั้งค่าร้าน รีวิว
- `docs/02-auth.md` — Keycloak, session, CSRF
- `docs/03-restaurants.md` — จัดการร้าน รูป หมวด พักรับจอง ค้นหา
- `docs/05-frontend.md` — หน้าเว็บและ design tokens
ถ้าเอกสารขัดกัน ให้ยึด `00-overview.md` และแจ้งผู้ใช้

## วิธีทำงาน
- ทำทีละขั้นตามตารางลำดับงานในแต่ละเอกสาร จบแต่ละขั้นให้หยุด สรุปสิ่งที่เปลี่ยน อธิบายโค้ดสำคัญเป็นภาษาไทย แล้วรอผู้ใช้ยืนยัน
- ห้ามเพิ่มฟีเจอร์ ตาราง คอลัมน์ endpoint หรือ dependency ที่ไม่มีในเอกสารโดยไม่ถาม
- งานที่ติดป้าย [B] ทำหลังงานระดับ A ของส่วนนั้นเสร็จเท่านั้น งานระดับ C ห้ามทำ
- สเปกกำกวม → ถาม ห้ามเดา
- ทุกกติกาธุรกิจต้องมี test · แก้ bug ให้เขียน test ที่พังก่อนแล้วค่อยแก้

## Backend (Go)
- `net/http` ServeMux (method + wildcard) ไม่ใช้ web framework · `pgx/v5` + SQL ที่เขียนเอง ไม่ใช้ ORM
- กติกาธุรกิจตรวจที่ Go เสมอ · สิทธิ์ตรวจที่ API ทุก endpoint
- Read Committed + `lockRestaurant` (กรองร้านที่ถูกลบ) · อ่านค่าร้านหลังล็อกเท่านั้น · ลำดับล็อก: ร้าน → การจอง
- ห้ามเรียก `time.Now()` ใน domain/service ให้รับ `now` จาก handler
- `import _ "time/tzdata"` · timezone `Asia/Bangkok` · ช่วงเวลาแบบ half-open `[start, end)`
- Error เป็น `application/problem+json` ตาม `00-overview.md` §5
- JSON snake_case, `DisallowUnknownFields`, `http.MaxBytesReader`
- ความเป็นเอกลักษณ์ใช้ unique constraint ตัดสิน ห้ามเช็คก่อนแล้วค่อย INSERT
- URL สาธารณะมาจาก `PUBLIC_BASE_URL` ห้ามสร้างจาก header `Host`
- query รายการใดๆ ห้ามดึง `restaurant_images.data`

## Frontend (Next.js 16)
- `await` ทุก `cookies()`, `headers()`, `params`, `searchParams`
- **ห้ามสร้าง `proxy.ts` (หรือ `middleware.ts`)**
- Server Component อ่านข้อมูล (ส่งต่อ cookie), Client Component ส่งข้อมูลผ่าน `/api/*` แล้ว `router.refresh()` · ไม่ใช้ Server Actions
- เวลา: ใช้ `lib/time.ts` เท่านั้น · ส่ง API เป็นสตริง `+07:00` · ห้ามใช้ `toISOString()`
- `FormData` ห้ามตั้ง `Content-Type` เอง
- สี ฟอนต์ และปุ่มตาม `docs/05-frontend.md` §5–6 เท่านั้น ห้ามใช้สีนอก token
- shadcn/ui อยู่ที่ `components/ui/` ห้ามแก้นอกจาก variant ปุ่มที่กำหนด
- ไม่มีคำสั่ง `next lint` ให้ใช้ ESLint ตรง

## ห้ามทำ
- ห้ามมี header, query หรือช่องทางใดๆ สำหรับสลับผู้ใช้ใน production code (fake auth อยู่ใน `_test.go` เท่านั้น)
- ห้ามใช้ Repeatable Read หรือ Serializable ในงานที่ล็อกแถวร้าน
- ห้าม commit secret จริง **ยกเว้น** `OIDC_CLIENT_SECRET`, รหัสผ่านบัญชีทดสอบ และรหัส admin ของ Keycloak ที่ใช้บนเครื่องเท่านั้น (ต้องระบุใน README)
- ห้ามรัน Go ใน Docker (ค่า issuer ของ token จะไม่ตรง)

## คำสั่ง
- `docker compose up -d` (postgres + keycloak)
- `go run ./cmd/migrate` (ในโฟลเดอร์ `backend/`, อ่าน `.env` ที่ root)
- `go test -p 1 ./...` (test ปกติบน Windows ไม่ใช้ `-race`)
- `./scripts/test-race.ps1` (test ทั้งหมดแบบ `-race` ใน container)
- `./scripts/test-race.ps1 -Concurrent` (เฉพาะ `TestConcurrent` แบบ `-count=100 -race`)
- `npm run dev` ในโฟลเดอร์ `frontend/` (Node.js 20.9+)
