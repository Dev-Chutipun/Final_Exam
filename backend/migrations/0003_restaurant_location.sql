-- พิกัดที่ตั้งร้าน (ไม่บังคับ) ใช้แสดงแผนที่ในหน้าร้าน
-- เก็บเป็นตัวเลข ไม่เก็บ URL ของ Google Maps เพราะลิงก์ย่อต้องให้เซิร์ฟเวอร์ตาม redirect ไปอ่าน (เสี่ยง SSRF)
-- และ URL ดิบที่ผู้ใช้กรอกเอาไปแสดงเป็นลิงก์ตรง ๆ ไม่ได้ · หน้าเว็บแกะพิกัดจากลิงก์ให้เองฝั่งเบราว์เซอร์
--
-- ทั้งสองคอลัมน์ต้องมีคู่กันเสมอ ร้านที่ยังไม่ระบุที่ตั้งเป็น NULL ทั้งคู่
ALTER TABLE restaurants
  ADD COLUMN IF NOT EXISTS latitude  DOUBLE PRECISION,
  ADD COLUMN IF NOT EXISTS longitude DOUBLE PRECISION;

ALTER TABLE restaurants
  DROP CONSTRAINT IF EXISTS restaurants_location_check;
ALTER TABLE restaurants
  ADD CONSTRAINT restaurants_location_check CHECK (
    (latitude IS NULL) = (longitude IS NULL)
    AND (latitude IS NULL OR (latitude BETWEEN -90 AND 90 AND longitude BETWEEN -180 AND 180))
  );
