# ADR-0003: เก็บรูปร้านใน PostgreSQL (`bytea`)

วันที่: 2026-10-01 · สถานะ: Accepted (ทบทวนเมื่อขึ้น Stage 2) · ผู้ตัดสินใจ: ผู้พัฒนา (Final Exam)

## บริบท (Context)

- ร้านต้องมีรูป 1–3 รูป (สไลด์ 5) แต่ละรูปไม่เกิน 2 MB
- สร้างร้านพร้อมรูปต้องสำเร็จหรือล้มเหลวทั้งชุด
- กรรมการต้อง clone แล้วรันได้ทันทีโดยไม่ต้องตั้งค่าอะไรเพิ่ม

## ทางเลือกที่พิจารณา

1. **`bytea` ใน PostgreSQL**
2. ไฟล์บนดิสก์ของเครื่อง API — ไม่ stateless (ขัด 12-Factor VI), เสี่ยง path traversal และไฟล์กำพร้าเมื่อ transaction ล้ม
3. Object storage (S3/MinIO) + presigned URL — เหมาะกับขนาดใหญ่ แต่ต้องเพิ่ม service, credential และการเก็บกวาดไฟล์กำพร้า

## การตัดสินใจ (Decision)

เลือกข้อ 1: ตาราง `restaurant_images (content_type, data bytea ≤ 2 MB)`
ตรวจชนิดจาก**เนื้อไฟล์**ด้วย `http.DetectContentType` รับเฉพาะ JPEG/PNG/WebP (ไม่รับ SVG)
ส่งรูปผ่าน `GET /api/restaurants/{id}/images/{imageId}` พร้อม `Cache-Control: immutable` และ `X-Content-Type-Options: nosniff`
query รายการทุกตัวห้ามดึงคอลัมน์ `data`

## ผลที่ตามมา (Consequences)

- \+ สร้างร้าน + หมวด + รูปใน transaction เดียว ไม่มีไฟล์กำพร้า
- \+ API ยัง stateless · backup ฐานข้อมูลได้รูปไปด้วย · รันได้ด้วย `docker compose up` อย่างเดียว
- − ฐานข้อมูลโตตามรูป (ทั้งประเทศ ≈ 300 GB) และทุกการดูรูปใช้ connection ของ DB
- − **เกณฑ์ย้าย:** เมื่อรูปกิน bandwidth/พื้นที่จนกระทบ query หลัก ให้ย้ายไป S3 + CDN โดยเปลี่ยน `data` เป็น object key
  และให้ endpoint ส่งรูป redirect ไป CDN แทน (หน้าเว็บไม่ต้องแก้เพราะใช้ URL เดิม)
