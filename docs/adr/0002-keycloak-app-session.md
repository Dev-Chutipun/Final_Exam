# ADR-0002: ใช้ Keycloak ยืนยันตัวตน แล้วให้ Go สร้าง session ของแอปเอง

วันที่: 2026-10-01 · สถานะ: Accepted · ผู้ตัดสินใจ: ผู้พัฒนา (Final Exam)

## บริบท (Context)

- ต้องมี login และสมัครสมาชิกได้ (สไลด์ 5) โดยไม่เขียนระบบรหัสผ่านเอง
- หน้าเว็บเป็น Next.js ที่ส่งทุกคำขอ `/api/*` ต่อให้ Go ผ่าน rewrite
- ต้องตอบได้ชัดว่า "session เก็บที่ไหน" และกัน CSRF ได้

## ทางเลือกที่พิจารณา

1. **Keycloak (OIDC) + Go เป็น confidential client + session ของแอปใน PostgreSQL**
2. ส่ง access token ของ Keycloak ให้เบราว์เซอร์ แล้ว Go ตรวจ JWT ทุกคำขอ — token อยู่ใน JavaScript ได้, logout/เพิกถอนยากกว่า
3. เขียน email + password เอง — ต้องดูแลการเก็บรหัส ลืมรหัส brute force เอง (เก็บไว้เป็นแผนสำรองถ้า Keycloak ใช้ไม่ได้)
4. NextAuth / Auth.js ใน Next.js — กติกาสิทธิ์จะกระจายไปอยู่สองที่

## การตัดสินใจ (Decision)

เลือกข้อ 1: Go ทำ Authorization Code + PKCE (S256) กับ Keycloak ตรวจ ID token (ลายเซ็น, iss, aud, exp) และเทียบ nonce เอง
จากนั้นสร้าง session ของแอป: token สุ่ม 32 bytes ใน cookie `HttpOnly; SameSite=Lax` อายุ 7 วัน ฐานข้อมูลเก็บแค่ SHA-256 ของ token
ผูกผู้ใช้ด้วย `(iss, sub)` ไม่ใช้อีเมล · logout ลบ session แล้วพาไป logout ที่ Keycloak ด้วย `id_token_hint`
URL ทั้งหมดสร้างจาก `PUBLIC_BASE_URL` ไม่อ่าน header `Host`

## ผลที่ตามมา (Consequences)

- \+ token ของ Keycloak ไม่ถึงเบราว์เซอร์เลย · ฐานข้อมูลหลุดก็เอา hash ไปใช้เป็น session ไม่ได้
- \+ API stateless (session อยู่ใน DB) จึงรันหลายตัวได้ · restart Go แล้ว session เดิมยังใช้ได้
- \+ สมัครสมาชิก, brute force protection และ login ด้วย Google (ระดับ C) ทำผ่าน Keycloak ได้โดยไม่แก้โค้ด
- − ทุกคำขอที่มี cookie ต้อง query ตาราง `sessions` หนึ่งครั้ง (ถ้าโตมากย้ายไป Redis ได้)
- − Go ต้องรันบนเครื่องเดียวกับที่เบราว์เซอร์เรียก Keycloak (`localhost:8081`) ไม่อย่างนั้น `iss` ไม่ตรง
