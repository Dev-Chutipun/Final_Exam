package restaurant

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"devpool/internal/db"
	"devpool/internal/httpx"
)

const MaxImages = 3

// AddImage เพิ่มรูปหนึ่งรูป (03 §4.3)
// นับรูปหลังล็อกแถวร้าน: ถ้านับก่อนล็อก การอัปโหลดพร้อมกันจะเห็น 2 รูปทั้งคู่แล้วรวมเป็น 4 รูปได้
func (s *Service) AddImage(ctx context.Context, userID, restaurantID int64, img httpx.Image, now time.Time) (int64, error) {
	var id int64
	err := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockOwned(ctx, tx, restaurantID, userID); err != nil {
			return err
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM restaurant_images WHERE restaurant_id = $1`, restaurantID).Scan(&n); err != nil {
			return err
		}
		if n >= MaxImages {
			return httpx.New("image-limit-reached", "ร้านมีรูปครบ 3 รูปแล้ว ลบรูปเดิมก่อนเพิ่มรูปใหม่")
		}
		return tx.QueryRow(ctx, `
			INSERT INTO restaurant_images (restaurant_id, content_type, data, created_at)
			VALUES ($1, $2, $3, $4) RETURNING id`, restaurantID, img.ContentType, img.Data, now).Scan(&id)
	})
	return id, err
}

// DeleteImage ลบรูปหนึ่งรูป รูปต้องเป็นของร้านนี้ และต้องเหลืออย่างน้อย 1 รูป
func (s *Service) DeleteImage(ctx context.Context, userID, restaurantID, imageID int64) error {
	return db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockOwned(ctx, tx, restaurantID, userID); err != nil {
			return err
		}
		var total int
		var belongs bool
		err := tx.QueryRow(ctx, `
			SELECT count(*), COALESCE(bool_or(id = $2), false) FROM restaurant_images WHERE restaurant_id = $1`,
			restaurantID, imageID).Scan(&total, &belongs)
		if err != nil {
			return err
		}
		// รูปของร้านอื่น → 404 (กันใช้ path ร้านตัวเองลบรูปร้านอื่น)
		if !belongs {
			return httpx.New("not-found", "ไม่พบรูปนี้ในร้าน")
		}
		if total <= 1 {
			return httpx.New("last-image", "ร้านต้องมีรูปอย่างน้อย 1 รูป")
		}
		_, err = tx.Exec(ctx, `DELETE FROM restaurant_images WHERE id = $1`, imageID)
		return err
	})
}

// Image คืนรูปหนึ่งรูป (endpoint เดียวที่ดึงคอลัมน์ data) ร้านถูกลบหรือรูปไม่ใช่ของร้านนี้ → 404
func (s *Service) Image(ctx context.Context, restaurantID, imageID int64) (httpx.Image, error) {
	var img httpx.Image
	err := s.Pool.QueryRow(ctx, `
		SELECT i.content_type, i.data
		  FROM restaurant_images i JOIN restaurants r ON r.id = i.restaurant_id
		 WHERE i.id = $2 AND i.restaurant_id = $1 AND r.deleted_at IS NULL`,
		restaurantID, imageID).Scan(&img.ContentType, &img.Data)
	if errors.Is(err, pgx.ErrNoRows) {
		return img, httpx.New("not-found", "ไม่พบรูปนี้")
	}
	return img, err
}
