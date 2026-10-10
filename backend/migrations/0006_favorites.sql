-- ร้านที่ผู้ใช้กดถูกใจ (03 §4.8)
-- คู่ (user_id, restaurant_id) เป็น primary key อยู่แล้ว กดซ้ำจึงซ้ำไม่ได้โดยไม่ต้องเช็คก่อน INSERT
-- ลบร้านเป็น soft delete แถวนี้จึงยังอยู่ และกลับมาเห็นได้ถ้าร้านถูกกู้คืน
CREATE TABLE IF NOT EXISTS favorites (
  user_id       BIGINT      NOT NULL REFERENCES users(id),
  restaurant_id BIGINT      NOT NULL REFERENCES restaurants(id),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, restaurant_id)
);

-- ใช้ตอนนับว่าร้านนี้มีคนถูกใจกี่คน และตอนล้างข้อมูลของร้าน
CREATE INDEX IF NOT EXISTS favorites_restaurant_idx ON favorites (restaurant_id);
