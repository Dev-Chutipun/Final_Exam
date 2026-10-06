# ADR-0004: Modular monolith + Next.js แบบ "Server Component อ่าน, Client Component ส่ง" ไม่มี proxy

วันที่: 2026-10-02 · สถานะ: Accepted · ผู้ตัดสินใจ: ผู้พัฒนา (Final Exam)

## บริบท (Context)

- ผู้พัฒนาคนเดียว ระยะเวลา 10 วัน และต้องอธิบายโค้ดส่วนสำคัญได้เอง
- โมดูลต่างๆ ต้องใช้ transaction ร่วมกัน เช่น ลบร้านต้องตรวจการจองในล็อกเดียวกัน, ตั้งค่าร้านต้องตรวจ peak ของการจอง
- Next.js 16 เปลี่ยน `middleware.ts` เป็น `proxy.ts` ซึ่ง buffer body ได้ไม่เกิน 10 MB และตัดส่วนเกินโดยไม่แจ้ง error

## ทางเลือกที่พิจารณา

1. **Go API ตัวเดียว แยก package ตามโดเมน (`auth`, `booking`, `restaurant`, `review`) + Next.js ที่ไม่มีกติกาธุรกิจ**
2. Microservices (booking / restaurant / review แยก service และ DB) — ต้องใช้ saga/outbox แทน transaction เดียว, ops ×5
3. Next.js เป็น backend เอง (Server Actions + ORM) — กติกาอยู่สองภาษา/สองที่ และตรวจ concurrency ยากกว่า

## การตัดสินใจ (Decision)

เลือกข้อ 1 ("Monolith first"): Go ใช้ `net/http` ServeMux + SQL ที่เขียนเองผ่าน `pgx` ไม่ใช้ framework/ORM
กติกาธุรกิจทั้งหมดอยู่ที่ Go · Next.js: Server Component อ่านข้อมูล (ส่งต่อ cookie, `cache: 'no-store'`),
Client Component ส่งผ่าน `/api/*` แล้ว `router.refresh()` · ไม่ใช้ Server Actions · ไม่มี `proxy.ts` ใช้ rewrite แทน

## ผลที่ตามมา (Consequences)

- \+ transaction ข้ามโมดูลทำได้ตรงๆ · debug และ test ง่าย (test ทั้งระบบรันใน `go test` เดียว)
- \+ อธิบายเส้นทางข้อมูลได้เส้นเดียว: เบราว์เซอร์ → Next.js → Go → PostgreSQL
- \+ multipart 3 × 2 MB ผ่าน rewrite ครบ (ทดสอบใน spike)
- − deploy ทุกโมดูลพร้อมกัน และ scale ทั้งก้อน (ยอมรับได้ที่ Stage 0–3)
- − ต้องรักษาเส้นแบ่งโมดูลเอง · **เกณฑ์แยก service:** เมื่อมีหลายทีมหรือโมดูลหนึ่งต้อง scale/deploy แยกจริง
  (ตัวเลือกแรกคือ notification worker ที่อ่านจาก queue ซึ่งไม่ต้องใช้ transaction ร่วม)
