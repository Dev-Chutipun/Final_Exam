-- ราคาเฉลี่ยต่อคนที่ผู้รีวิวจ่ายจริง (01 §7.7)
--
-- เก็บเป็นลำดับช่วง 1–5 ไม่ใช่จำนวนเงิน เพราะถ้าให้พิมพ์ตัวเลขเองจะได้ "250", "250 บาท", "~300",
-- "สองคน 600" ปนกันจนรวมเป็นสถิติไม่ได้ · ช่วงถี่ที่ราคาต่ำเพราะร้านส่วนใหญ่ในระบบไม่เกิน 500 บาท/คน
--   1 = ต่ำกว่า 100 · 2 = 100–200 · 3 = 201–300 · 4 = 301–500 · 5 = มากกว่า 500
--
-- ไม่บังคับ (NULL = ไม่ระบุ) เพราะบางคนมีคนเลี้ยงหรือจำไม่ได้ และรีวิวบังคับกรอกดาว 4 ด้านอยู่แล้ว
ALTER TABLE reviews
  ADD COLUMN IF NOT EXISTS price_level SMALLINT;

ALTER TABLE reviews DROP CONSTRAINT IF EXISTS reviews_price_level_check;
ALTER TABLE reviews
  ADD CONSTRAINT reviews_price_level_check
    CHECK (price_level IS NULL OR price_level BETWEEN 1 AND 5);
