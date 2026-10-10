-- ที่อยู่ร้านเป็นข้อความ (ไม่บังคับ) แสดงคู่กับแผนที่ในหน้าร้าน
-- แยกจาก description เพราะที่อยู่กับพิกัดเป็นเรื่องเดียวกัน ควรอยู่ด้วยกัน
-- ส่วน description เหลือไว้เล่าเรื่องร้านอย่างเดียว
--
-- เป็นอิสระจาก latitude/longitude: ระบุที่อยู่อย่างเดียว หรือปักหมุดอย่างเดียวก็ได้
ALTER TABLE restaurants
  ADD COLUMN IF NOT EXISTS address TEXT;

ALTER TABLE restaurants
  DROP CONSTRAINT IF EXISTS restaurants_address_check;
ALTER TABLE restaurants
  ADD CONSTRAINT restaurants_address_check
    CHECK (address IS NULL OR char_length(btrim(address)) BETWEEN 1 AND 200);
