# 02 · Login ด้วย Keycloak, session และ CSRF

**สถานะ:** ยืนยันแล้ว 1 ต.ค. 2026
**อ่านก่อน:** `docs/00-overview.md`

---

## 0. วิธีใช้ไฟล์นี้

- เริ่มจาก **spike ใน §8 ก่อนเขียนโค้ดส่วนนี้** แล้วรายงานผลเป็นตาราง ถ้าข้อไหนไม่ผ่าน ให้หยุดและถามผู้ใช้
- ทำตามลำดับใน §10 จบแต่ละขั้นให้หยุด สรุป อธิบายเป็นภาษาไทย แล้วรอยืนยัน
- **จำกัดเวลาส่วนนี้ 1 วัน** ถ้าเกิน ให้หยุดและเสนอแผนถอยไปใช้ email + password

---

## 1. หลักการ

| ผู้เล่น | หน้าที่ |
|---|---|
| Keycloak | ยืนยันตัวตนอย่างเดียว (หน้า login, สมัครสมาชิก, รหัสผ่าน) |
| Go | OIDC client แบบ confidential คุยกับ Keycloak ฝั่งเซิร์ฟเวอร์ แล้วสร้าง **session ของแอปเอง** |
| Browser | ถือแค่ cookie ค่าสุ่ม **ไม่เคยเห็น token ของ Keycloak** |
| Next.js | ไม่เก็บอะไรเกี่ยวกับ auth ส่งต่อคำขอและ cookie ให้ Go เท่านั้น |

- หลัง login แอปไม่ใช้ token ของ Keycloak อีก ยกเว้น `id_token` ที่เก็บไว้ใช้ตอน logout
- ไลบรารี: `github.com/coreos/go-oidc/v3` (discovery + ตรวจ ID token) และ `golang.org/x/oauth2` (Authorization Code + PKCE)
- URL สาธารณะของแอปมาจาก config `PUBLIC_BASE_URL` เสมอ **ห้ามสร้างจาก header `Host`** เพราะคำขอผ่าน rewrite ของ Next.js

---

## 2. Pipelines

### 2.1 เริ่ม login — `GET /api/auth/login?return_to=/bookings`

| ขั้น | ทำอะไร |
|---|---|
| 1 | ตรวจ `return_to`: ต้องขึ้นต้นด้วย `/` และตัวที่สองต้องไม่ใช่ `/` หรือ `\` ถ้าไม่ผ่านใช้ `/` (กัน open redirect) |
| 2 | สุ่ม `state`, `nonce` (32 bytes) และ PKCE verifier (`oauth2.GenerateVerifier()`) |
| 3 | เก็บทั้งหมดรวม `return_to` ใน cookie `oidc_login` (`HttpOnly`, `SameSite=Lax`, `Path=/api/auth`, `Max-Age=600`, `Secure` ตาม `COOKIE_SECURE`) |
| 4 | 302 ไป authorization endpoint ของ Keycloak พร้อม `response_type=code`, `scope=openid profile`, `redirect_uri=PUBLIC_BASE_URL/api/auth/callback`, `state`, `nonce`, `code_challenge` แบบ S256 |

### 2.2 รับผลกลับ — `GET /api/auth/callback`

| ขั้น | ตรวจอะไร | ถ้าไม่ผ่าน → 302 `/auth/error?code=` |
|---|---|---|
| 1 | Keycloak ไม่ได้ส่ง `error` มา | `cancelled` |
| 2 | มี cookie `oidc_login` | `login_expired` |
| 3 | `state` ตรงกับ cookie แล้วลบ cookie ทันที (ใช้ได้ครั้งเดียว) | `invalid_state` |
| 4 | แลก code ด้วย client secret + PKCE verifier | `verification_failed` |
| 5 | ตรวจ ID token ด้วย go-oidc (ลายเซ็น, `iss`, `aud`, `exp`) และ **เทียบ `nonce` เอง** เพราะ go-oidc ไม่ตรวจให้ | `verification_failed` |
| 6 | Upsert ผู้ใช้ (§3) | — |
| 7 | สร้าง session (§4) ตั้ง cookie `session` แล้ว 302 ไป `return_to` | — |

แยกการคุยกับ Keycloak ไว้หลัง interface เพื่อให้ test ใช้ตัวปลอมได้:
```go
type Exchanger interface {
    Exchange(ctx context.Context, code, verifier string) (Claims, rawIDToken string, err error)
}
```

### 2.3 Middleware (ทุกคำขอ)

1. อ่าน cookie `session` แล้ว hash ด้วย SHA-256
2. `SELECT user_id FROM sessions WHERE token_hash = $1 AND expires_at > $now`
3. พบ → ใส่ userID ลง context · ไม่พบ → **ไม่ตอบ error** ให้ handler ตัดสินเองผ่าน `auth.UserID(ctx)` (มี endpoint สาธารณะ)

### 2.4 Logout — `POST /api/auth/logout` (ฟอร์ม HTML ธรรมดา)

1. ลบแถว session และล้าง cookie (`Max-Age=0`)
2. 303 ไป `end_session_endpoint` ของ Keycloak (อ่านจาก discovery document) พร้อม `id_token_hint` และ `post_logout_redirect_uri=PUBLIC_BASE_URL/`
3. ไม่มี session อยู่แล้ว → 303 ไป `/`

**ต้อง logout ที่ Keycloak ด้วย** ไม่อย่างนั้น Keycloak จะ login บัญชีเดิมให้อัตโนมัติ และสลับบัญชีตอนเดโมไม่ได้ · ใช้ POST เพื่อไม่ให้เว็บอื่นสั่ง logout ได้ด้วยแท็ก `<img>`

### 2.5 ผู้ใช้ปัจจุบัน — `GET /api/me`

200 `{ "id": 1, "display_name": "อลิซ ใจดี" }` · ไม่ได้ login → 401 `unauthenticated`

---

## 3. ผู้ใช้

```sql
INSERT INTO users (issuer, subject, display_name) VALUES ($1, $2, $3)
ON CONFLICT (issuer, subject) DO UPDATE SET display_name = EXCLUDED.display_name
RETURNING id;
```

**กฎชื่อแสดงผล** (ชื่อนี้แสดงต่อสาธารณะในรีวิว)
1. ใช้ claim `name`
2. ถ้าไม่มี ใช้ `preferred_username`
3. **ค่าที่มี `@` ห้ามใช้** ให้ข้ามไปใช้ `given_name` ถ้าไม่มีอีกให้ใช้ `"ผู้ใช้"`
4. ตัดช่องว่างหัวท้าย และตัดความยาวไม่เกิน 60 ตัวอักษร

**ไม่เก็บอีเมล** เพราะแอปไม่ใช้

---

## 4. Session และ cookie

| รายการ | ค่า |
|---|---|
| Token | สุ่ม 32 bytes ด้วย `crypto/rand` เข้ารหัส base64url |
| เก็บใน DB | SHA-256 ของ token ใน `sessions.token_hash` |
| อายุ | **7 วัน** นับจากตอน login (ไม่ต่ออายุอัตโนมัติ) |
| Cookie `session` | `HttpOnly`, `SameSite=Lax`, `Path=/`, `Max-Age=604800`, `Secure` ตาม `COOKIE_SECURE` |
| หมดอายุ | ไม่ต้องมี job ลบ ตอน login ครั้งถัดไปของผู้ใช้คนนั้นให้ลบแถวที่หมดอายุของเขาทิ้ง |

---

## 5. การกัน CSRF (3 ชั้น)

1. Cookie `SameSite=Lax`
2. `http.CrossOriginProtection` ของ Go 1.25+ ครอบ mux ทั้งหมด และ `AddTrustedOrigin(PUBLIC_BASE_URL)` ถ้า API ของเวอร์ชันที่ใช้มีวิธีกำหนด handler ตอนปฏิเสธ ให้ตอบเป็น problem `cross-origin-request` 403
3. Endpoint ที่ **อ่าน JSON body** ต้องมี `Content-Type: application/json` ไม่อย่างนั้น 415 `unsupported-media-type` · endpoint ที่ไม่มี body (เช่น `POST /api/bookings/{id}/cancel`, `DELETE` ทุกตัว) ไม่ตรวจ Content-Type และพึ่งชั้น 1–2
   **ยกเว้น** logout (ฟอร์ม) และ endpoint อัปโหลดแบบ multipart ใน `03-restaurants.md` ซึ่งพึ่งชั้น 1–2

---

## 6. Keycloak

| รายการ | ค่า |
|---|---|
| รัน | `docker-compose.yml` service `keycloak` พอร์ต `8081:8080`, คำสั่ง `start-dev --import-realm`, mount `./keycloak` ไปที่ `/opt/keycloak/data/import` |
| เวอร์ชัน | ใช้เวอร์ชันเสถียรล่าสุด ณ วันติดตั้ง แล้ว **ระบุ tag ตายตัว** ห้ามใช้ `latest` |
| Admin | ตั้งผ่าน environment ตามชื่อที่เวอร์ชันนั้นกำหนด (ตรวจใน spike ข้อ 1) · 26.8.0 ใช้ `KC_BOOTSTRAP_ADMIN_USERNAME` / `KC_BOOTSTRAP_ADMIN_PASSWORD` ค่า `admin` / `admin` ใน `docker-compose.yml` (ใช้บนเครื่องเท่านั้น) |
| Realm | `devpool` |
| ตั้งค่า realm | `registrationAllowed: true` (สไลด์ 5 กำหนดให้สมัครได้), `verifyEmail: false`, `bruteForceProtected: true` |
| Client | `devpool-web` · confidential · Standard flow เปิด · Direct access grants ปิด · Implicit ปิด · PKCE `S256` · redirect URI `http://localhost:3000/api/auth/callback` · post logout redirect `http://localhost:3000/` · secret `devpool-local-secret` |

**บัญชีทดสอบ** (กำหนด `id` ตายตัว เพราะ `sub` ใน token คือ id ของผู้ใช้ seed data จะผูกร้านกับเจ้าของได้ก่อน login ครั้งแรก)

| username | id (= `sub`) | ชื่อ | บทบาทใน seed |
|---|---|---|---|
| `chutipun` | `11111111-1111-4111-8111-111111111111` | Chutipun Homsaard | ลูกค้าอย่างเดียว |
| `ownertest` | `22222222-2222-4222-8222-222222222222` | Owner Test | เจ้าของ 6 ร้าน (ครัวริมคลอง, ซูชิบาร์ ท่าเรือ, สเต็กลุงพล, ใบชา คาเฟ่ ฯลฯ) |
| `usertest` | `33333333-3333-4333-8333-333333333333` | User Test | เจ้าของฟอเฮ้าส์และพาสต้าบ้านนา · ลูกค้าร้านอื่น |
| `somchai` | `44444444-4444-4444-8444-444444444444` | สมชาย ใจกว้าง | เจ้าของ 4 ร้าน |
| `nida` | `55555555-5555-4555-8555-555555555555` | นิดา รักดี | เจ้าของ 4 ร้าน · ลูกค้า |
| `arthit` | `66666666-6666-4666-8666-666666666666` | อาทิตย์ แสงทอง | เจ้าของ 4 ร้าน |
| `somsri` | `77777777-7777-4777-8777-777777777777` | สมศรี มีสุข | ลูกค้าและผู้รีวิว |
| `kanya` | `88888888-8888-4888-8888-888888888888` | กันยา พากเพียร | ลูกค้าและผู้รีวิว |

ทุกบัญชีใช้รหัสผ่าน `homsaard` (ผ่าน `passwordPolicy` `length(8) and notUsername`) มี email (เช่น `chutipun@example.com`), `emailVerified: true`, `firstName`, `lastName` เพื่อไม่ให้ Keycloak ถามข้อมูลเพิ่มตอน login · Keycloak เก็บ username เป็นตัวพิมพ์เล็กเสมอ
ผู้รีวิวตัวอย่างอื่นใน seed (เช่น ดนัย, มาลี) สร้างเฉพาะในตาราง `users` ด้วย `issuer = 'seed://local'` และไม่สามารถ login ได้

**ข้อยกเว้นเรื่อง secret:** `OIDC_CLIENT_SECRET`, รหัสผ่านทดสอบ และรหัส admin ของ Keycloak อยู่ในไฟล์ realm, `.env.example` และ `docker-compose.yml` ได้ เพราะใช้บนเครื่องเท่านั้น README ต้องเขียนกำกับไว้ชัด

**Go ต้องรันบนเครื่อง ไม่รันใน Docker:** ค่า `iss` ใน token ขึ้นกับ hostname ที่ใช้เรียก Keycloak ถ้า browser ใช้ `localhost:8081` แต่ Go ใช้ชื่อภายใน Docker ค่าจะไม่ตรงกัน

---

## 7. Design Decisions

| การตัดสินใจ | เหตุผล |
|---|---|
| Keycloak + session ของแอปเอง (Go เป็น OIDC client) | อาจารย์สอน, สมัครสมาชิกได้, token ไม่ถึง browser, ตอบได้ชัดว่า session เก็บที่ไหน |
| เก็บ hash ของ token ไม่เก็บค่าจริง | ฐานข้อมูลหลุดก็เอาไปใช้เป็น session ไม่ได้ |
| state + nonce + PKCE | กัน login CSRF, การใช้ token ซ้ำ และ code หลุด (RFC 9700 แนะนำ PKCE ทุก client) |
| ตรวจ `return_to` | กัน open redirect รวมกรณี `/\` |
| Logout ที่ Keycloak ด้วย และใช้ POST | สลับบัญชีได้จริง และเว็บอื่นสั่ง logout ไม่ได้ |
| ผูกผู้ใช้ด้วย `iss` + `sub` ไม่ใช้อีเมล | `sub` ไม่ซ้ำและไม่ถูกใช้ซ้ำภายใน issuer เดียวกัน ส่วนอีเมลเปลี่ยนได้ |
| ชื่อที่มี `@` ห้ามแสดง | กันอีเมลหลุดไปแสดงในรีวิว |
| Session 7 วัน | สะดวกตอนเดโม |
| CSRF 3 ชั้น | `SameSite`, ตรวจ origin, บังคับ JSON |
| Login ด้วย Google | ระดับ C ทำผ่าน Identity Provider ของ Keycloak ได้โดยไม่แก้โค้ด |

---

## 8. Spike (ทำก่อน ≤ 2 ชม.)

สร้าง compose + realm + Next.js ขั้นต่ำที่มี rewrite `/api/:path*` → Go + handler login/callback แบบง่าย แล้วรายงานผล 8 ข้อ:

| # | ต้องยืนยัน |
|---|---|
| 1 | Keycloak เวอร์ชันที่เลือกรันได้ และชื่อ env ของ admin ถูกต้อง |
| 2 | Import realm สำเร็จ: ผู้ใช้ได้ id ตามไฟล์, login ด้วยรหัสผ่านได้, มีลิงก์สมัครสมาชิก, client ใช้ secret + PKCE ได้ |
| 3 | Rewrite ของ Next.js ส่ง 302 และ `Set-Cookie` จาก Go กลับไปถึง browser ทั้งตอน login และ callback |
| 4 | `iss` ใน ID token ตรงกับ `OIDC_ISSUER` |
| 5 | `sub` ตรงกับ id ในไฟล์ realm |
| 6 | Logout พร้อม `id_token_hint` กลับมาหน้าแรกโดยไม่มีหน้ายืนยันคั่น และ login ครั้งต่อไปต้องถามรหัสใหม่ |
| 7 | `CrossOriginProtection` ยอมให้ fetch แบบ POST จากหน้าเว็บของเราผ่าน rewrite ได้ |
| 8 | อัปโหลด multipart 3 ไฟล์ ไฟล์ละ 2 MB ผ่าน rewrite ได้ครบ (ไม่มี `proxy.ts`) |

---

## 9. Test

**อัตโนมัติ** (ใช้ `Exchanger` ตัวปลอม)

| กลุ่ม | กรณี |
|---|---|
| Login | cookie `oidc_login` มี attribute ครบ · URL มี `state`, `nonce`, `code_challenge_method=S256`, `redirect_uri` จาก `PUBLIC_BASE_URL` |
| `return_to` | `/bookings` ผ่าน · `https://evil.com`, `//evil.com`, `/\evil.com` กลายเป็น `/` |
| Callback | Keycloak ส่ง error · ไม่มี cookie · state ไม่ตรง · แลก code ล้มเหลว · **nonce ไม่ตรง** → ทุกกรณีไป `/auth/error` ด้วยรหัสที่ถูก · สำเร็จ: สร้างผู้ใช้ใหม่, อัปเดตชื่อผู้ใช้เดิม, session เก็บเป็น hash, cookie ถูกตั้ง, `oidc_login` ถูกลบ |
| ชื่อแสดงผล | `name` · fallback `preferred_username` · `preferred_username` เป็นอีเมล → `given_name` · ไม่มีอะไรเลย → `"ผู้ใช้"` |
| Middleware | session ถูกต้อง / หมดอายุ / ไม่มีในระบบ / ค่าถูกแก้ไข |
| Logout | session ถูกลบ · cookie ถูกล้าง · 303 พร้อม `id_token_hint` · ไม่มี session → 303 `/` |
| CSRF | `Sec-Fetch-Site: cross-site` → 403 · `Origin` ต่างเว็บ → 403 · origin เดียวกัน → ผ่าน · ไม่มีทั้งสอง header (server ต่อ server) → ผ่าน · JSON endpoint ที่ส่ง `text/plain` → 415 |
| `/api/me` | 200 / 401 |

**ด้วยมือ:** spike ทั้ง 8 ข้อซ้ำอีกรอบก่อนส่งงาน + สมัครบัญชีใหม่แล้ว login ครั้งแรกต้องได้แถวใน `users` + รีสตาร์ท Go แล้ว session เดิมยังใช้ได้

---

## 10. ลำดับการทำงาน

| ขั้น | งาน | จบเมื่อ |
|---|---|---|
| 1 | Spike §8 | รายงานผลครบ 8 ข้อ |
| 2 | `keycloak/realm-devpool.json` ฉบับจริง + compose | login ด้วยบัญชีทดสอบทั้ง 3 ได้ |
| 3 | config, provider discovery, `Exchanger` จริง | ต่อ Keycloak ได้ตอนเริ่มโปรแกรม (ถ้าต่อไม่ได้ให้แจ้ง error ชัดเจน) |
| 4 | login, callback, upsert ผู้ใช้, session, middleware, `/api/me` + test | test ผ่าน |
| 5 | logout + CSRF + test | test ผ่าน → **Checkpoint D: ผู้ใช้อธิบาย flow login, หน้าที่ของ state/nonce/PKCE และ session เก็บที่ไหน** |
| 6 | เปลี่ยน stub ของ `auth.UserID(ctx)` เป็นของจริง แล้วรัน test ทั้งหมดของส่วนที่ 1 | ผ่านทั้งหมด |
