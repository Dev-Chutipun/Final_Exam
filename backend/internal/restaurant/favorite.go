package restaurant

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"devpool/internal/booking"
	"devpool/internal/db"
	"devpool/internal/httpx"
)

// ร้านที่ถูกใจ (03 §4.8)
//
// เจ้าของกดถูกใจร้านตัวเองได้ ต่างจากการจองและรีวิวที่ห้าม เพราะถูกใจเป็นแค่ที่คั่นหน้าส่วนตัว
// ไม่ได้แสดงต่อสาธารณะและไม่มีผลกับอันดับร้าน จึงไม่มีเรื่องผลประโยชน์ทับซ้อน

// Favorite คือรายการใน GET /api/me/favorites
type Favorite struct {
	ID                int64    `json:"id"`
	Name              string   `json:"name"`
	CoverImageID      *int64   `json:"cover_image_id"`
	OpenTime          string   `json:"open_time"`
	CloseTime         string   `json:"close_time"`
	AcceptingBookings bool     `json:"accepting_bookings"`
	AvgRating         *float64 `json:"avg_rating"`
	ReviewCount       int      `json:"review_count"`
	FavoritedAt       string   `json:"favorited_at"`
}

// SetFavorite เพิ่มหรือเอาร้านออกจากรายการถูกใจ สั่งซ้ำได้ผลเท่าเดิม (idempotent)
// ร้านที่ถูกลบแล้ว → 404 · ไม่ต้องล็อกแถวร้านเพราะไม่มีกติกาใดขึ้นกับสถานะร้าน
func (s *Service) SetFavorite(ctx context.Context, userID, restaurantID int64, on bool, now time.Time) error {
	if _, err := db.GetRestaurant(ctx, s.Pool, restaurantID); errors.Is(err, db.ErrNotFound) {
		return httpx.New("not-found", "ไม่พบร้านนี้")
	} else if err != nil {
		return err
	}
	if !on {
		_, err := s.Pool.Exec(ctx, `DELETE FROM favorites WHERE user_id = $1 AND restaurant_id = $2`, userID, restaurantID)
		return err
	}
	// กดซ้ำไม่ใช่ error: primary key ตัดสินความซ้ำ ไม่ต้องเช็คก่อนแล้วค่อย INSERT
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO favorites (user_id, restaurant_id, created_at) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, restaurant_id) DO NOTHING`, userID, restaurantID, now)
	return err
}

// Favorites คืนร้านที่ผู้ใช้ถูกใจ ใหม่สุดก่อน ไม่รวมร้านที่ถูกลบ
func (s *Service) Favorites(ctx context.Context, userID int64) ([]Favorite, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT r.id, r.name,
		       (SELECT MIN(id) FROM restaurant_images WHERE restaurant_id = r.id),
		       to_char(r.open_time, 'HH24:MI'), to_char(r.close_time, 'HH24:MI'), r.accepting_bookings,
		       (SELECT ROUND(AVG(rating), 1)::float8 FROM reviews WHERE restaurant_id = r.id),
		       (SELECT count(*) FROM reviews WHERE restaurant_id = r.id),
		       f.created_at
		  FROM favorites f JOIN restaurants r ON r.id = f.restaurant_id
		 WHERE f.user_id = $1 AND r.deleted_at IS NULL
		 ORDER BY f.created_at DESC, r.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Favorite, error) {
		var f Favorite
		var at time.Time
		err := row.Scan(&f.ID, &f.Name, &f.CoverImageID, &f.OpenTime, &f.CloseTime,
			&f.AcceptingBookings, &f.AvgRating, &f.ReviewCount, &at)
		f.FavoritedAt = at.In(booking.Bangkok).Format(time.RFC3339)
		return f, err
	})
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []Favorite{}
	}
	return list, nil
}
