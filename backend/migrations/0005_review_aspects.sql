-- รีวิวแบ่งคะแนนเป็น 4 ด้าน แทนดาวรวมดาวเดียว (01 §7.7)
-- คะแนนรวมไม่ได้ให้ผู้ใช้กรอก แต่เป็น generated column ที่ PostgreSQL คำนวณจากทั้ง 4 ด้าน
-- จึงไม่มีทางที่คะแนนรวมกับคะแนนรายด้านจะไม่ตรงกัน ไม่ว่าจะเขียนผ่าน path ไหน

-- 1) เพิ่มทีละด้านแบบว่างได้ก่อน
ALTER TABLE reviews
  ADD COLUMN IF NOT EXISTS rating_food      SMALLINT,
  ADD COLUMN IF NOT EXISTS rating_ambience  SMALLINT,
  ADD COLUMN IF NOT EXISTS rating_service   SMALLINT,
  ADD COLUMN IF NOT EXISTS rating_value     SMALLINT;

-- 2) รีวิวเก่ามีแต่ดาวรวม ใช้ค่าเดิมเป็นค่าตั้งต้นของทุกด้าน (ข้อมูลเดโม ไม่ใช่ข้อมูลผู้ใช้จริง)
UPDATE reviews SET
  rating_food     = COALESCE(rating_food,     rating),
  rating_ambience = COALESCE(rating_ambience, rating),
  rating_service  = COALESCE(rating_service,  rating),
  rating_value    = COALESCE(rating_value,    rating)
 WHERE rating_food IS NULL OR rating_ambience IS NULL OR rating_service IS NULL OR rating_value IS NULL;

-- 3) บังคับให้ครบและอยู่ในช่วง 1–5
ALTER TABLE reviews
  ALTER COLUMN rating_food     SET NOT NULL,
  ALTER COLUMN rating_ambience SET NOT NULL,
  ALTER COLUMN rating_service  SET NOT NULL,
  ALTER COLUMN rating_value    SET NOT NULL;

ALTER TABLE reviews DROP CONSTRAINT IF EXISTS reviews_aspects_check;
ALTER TABLE reviews ADD CONSTRAINT reviews_aspects_check CHECK (
  rating_food     BETWEEN 1 AND 5 AND
  rating_ambience BETWEEN 1 AND 5 AND
  rating_service  BETWEEN 1 AND 5 AND
  rating_value    BETWEEN 1 AND 5
);

-- 4) เปลี่ยน rating เป็นคะแนนรวมที่คำนวณให้เอง (ค่าอยู่ระหว่าง 1.00 ถึง 5.00 เสมอตามข้อ 3)
ALTER TABLE reviews DROP COLUMN IF EXISTS rating;
ALTER TABLE reviews
  ADD COLUMN rating NUMERIC(3, 2)
    GENERATED ALWAYS AS ((rating_food + rating_ambience + rating_service + rating_value) / 4.0) STORED;
