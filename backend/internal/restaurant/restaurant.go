package restaurant

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"devpool/internal/booking"
	"devpool/internal/db"
	"devpool/internal/httpx"
)

type Category struct {
	ID     int    `json:"id"`
	Slug   string `json:"slug"`
	NameTH string `json:"name_th"`
}

// Info คือชื่อ รายละเอียด หมวด และที่ตั้ง (ส่วนที่แก้ได้โดยไม่กระทบการจอง)
type Info struct {
	Name        string
	Description string
	CategoryIDs []int
	Address     *string   // nil = ไม่ระบุที่อยู่
	Location    *Location // nil = ไม่ได้ปักหมุดบนแผนที่
}

// Location คือพิกัดที่ตั้งร้าน ใช้แสดงแผนที่เท่านั้น ไม่มีผลกับกติกาการจอง
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// lat และ lng แปลงที่ตั้งเป็นค่าที่ส่งเข้า SQL ได้ (nil → NULL ทั้งคู่ ตรงกับ CHECK ในตาราง)
func lat(l *Location) *float64 {
	if l == nil {
		return nil
	}
	return &l.Latitude
}

func lng(l *Location) *float64 {
	if l == nil {
		return nil
	}
	return &l.Longitude
}

// NewRestaurant คือข้อมูลสร้างร้านที่ผ่านการตรวจรูปแบบแล้ว
type NewRestaurant struct {
	Info
	Settings
	Images []httpx.Image
}

// Detail คือ GET /api/restaurants/{id} (03 §4.6)
type Detail struct {
	ID                  int64      `json:"id"`
	Name                string     `json:"name"`
	Description         string     `json:"description"`
	Categories          []Category `json:"categories"`
	ImageIDs            []int64    `json:"image_ids"`
	Capacity            int        `json:"capacity"`
	OpenTime            string     `json:"open_time"`
	CloseTime           string     `json:"close_time"`
	CancelWindowMinutes int        `json:"cancel_window_minutes"`
	MinLeadMinutes      int        `json:"min_lead_minutes"`
	AcceptingBookings   bool       `json:"accepting_bookings"`
	Address             *string    `json:"address"`  // null เมื่อไม่ระบุที่อยู่
	Latitude            *float64   `json:"latitude"` // null คู่กับ longitude เมื่อไม่ระบุที่ตั้ง
	Longitude           *float64   `json:"longitude"`
	AvgRating           *float64   `json:"avg_rating"`
	// ค่าเฉลี่ยรายด้านของทุกรีวิวในร้าน null เมื่อยังไม่มีรีวิว
	AspectAverages *ReviewAspects `json:"aspect_averages"`
	// ช่วงราคาต่อคนที่คนส่วนใหญ่จ่าย ใช้มัธยฐานเพราะช่วงราคาเป็นข้อมูลลำดับ เอามาเฉลี่ยไม่ได้
	// null เมื่อมีคนระบุราคาน้อยกว่า 3 คน (น้อยกว่านั้นยังไม่ควรประกาศเป็นราคาของร้าน)
	PriceLevel      *int      `json:"price_level"`
	PriceLevelCount int       `json:"price_level_count"`
	ReviewCount     int       `json:"review_count"`
	IsOwner         bool      `json:"is_owner"`    // ใช้ซ่อนหรือแสดงปุ่มเท่านั้น สิทธิ์จริงตัดสินที่แต่ละ endpoint
	IsFavorite      bool      `json:"is_favorite"` // false เสมอเมื่อยังไม่ได้ login
	MyReview        *MyReview `json:"my_review"`
}

// ReviewAspects คือคะแนน 4 ด้านของรีวิว (ประกาศซ้ำในแพ็กเกจนี้แบบเดียวกับ Category
// เพื่อไม่ให้ restaurant ต้องพึ่ง review ซึ่งเป็นคนละส่วนของระบบ)
type ReviewAspects struct {
	Food     float64 `json:"food"`
	Ambience float64 `json:"ambience"`
	Service  float64 `json:"service"`
	Value    float64 `json:"value"`
}

type MyReview struct {
	ID      int64         `json:"id"`
	Rating  float64       `json:"rating"` // คะแนนรวมที่ฐานข้อมูลคำนวณจาก 4 ด้าน
	Aspects ReviewAspects `json:"aspects"`
	Comment string        `json:"comment"`
}

// MyRestaurant คือรายการใน GET /api/me/restaurants
type MyRestaurant struct {
	ID                int64    `json:"id"`
	Name              string   `json:"name"`
	CoverImageID      *int64   `json:"cover_image_id"`
	Capacity          int      `json:"capacity"`
	OpenTime          string   `json:"open_time"`
	CloseTime         string   `json:"close_time"`
	AcceptingBookings bool     `json:"accepting_bookings"`
	AvgRating         *float64 `json:"avg_rating"`
	ReviewCount       int      `json:"review_count"`
	UpcomingCount     int      `json:"upcoming_count"`
}

// validateLocation ตรวจพิกัด ต้องส่งมาคู่กันหรือไม่ส่งเลย (03 §3)
func validateLocation(errs []httpx.FieldError, lat, lng *float64) ([]httpx.FieldError, *Location) {
	switch {
	case lat == nil && lng == nil:
		return errs, nil
	case lat == nil || lng == nil:
		return append(errs, httpx.FieldError{Field: "latitude", Message: "ต้องระบุทั้ง latitude และ longitude หรือไม่ระบุทั้งคู่"}), nil
	}
	ok := true
	if *lat < -90 || *lat > 90 {
		errs, ok = append(errs, httpx.FieldError{Field: "latitude", Message: "ต้องอยู่ระหว่าง -90 ถึง 90"}), false
	}
	if *lng < -180 || *lng > 180 {
		errs, ok = append(errs, httpx.FieldError{Field: "longitude", Message: "ต้องอยู่ระหว่าง -180 ถึง 180"}), false
	}
	if !ok {
		return errs, nil
	}
	return errs, &Location{Latitude: *lat, Longitude: *lng}
}

// validateAddress ตรวจที่อยู่ ไม่ส่งมาหรือส่งมาเป็นช่องว่างล้วน = ไม่ระบุ (03 §3)
func validateAddress(errs []httpx.FieldError, address *string) ([]httpx.FieldError, *string) {
	if address == nil {
		return errs, nil
	}
	v := strings.TrimSpace(*address)
	if v == "" {
		return errs, nil
	}
	if utf8.RuneCountInString(v) > 200 {
		return append(errs, httpx.FieldError{Field: "address", Message: "ต้องยาวไม่เกิน 200 ตัวอักษร"}), nil
	}
	return errs, &v
}

// validateInfo ตรวจชื่อ รายละเอียด หมวด ที่อยู่ และพิกัด (03 §3) หลัง TrimSpace นับความยาวเป็น rune
func validateInfo(errs []httpx.FieldError, name, description *string, categoryIDs []int, address *string, lat, lng *float64) ([]httpx.FieldError, Info) {
	var in Info
	errs, in.Address = validateAddress(errs, address)
	errs, in.Location = validateLocation(errs, lat, lng)
	if name != nil {
		in.Name = strings.TrimSpace(*name)
	}
	if n := utf8.RuneCountInString(in.Name); n < 1 || n > 80 {
		errs = append(errs, httpx.FieldError{Field: "name", Message: "ต้องยาว 1–80 ตัวอักษร"})
	}
	if description != nil {
		in.Description = strings.TrimSpace(*description)
	}
	if n := utf8.RuneCountInString(in.Description); n < 1 || n > 2000 {
		errs = append(errs, httpx.FieldError{Field: "description", Message: "ต้องยาว 1–2,000 ตัวอักษร"})
	}

	seen := map[int]bool{}
	for _, id := range categoryIDs {
		if seen[id] {
			errs = append(errs, httpx.FieldError{Field: "category_ids", Message: "ห้ามเลือกหมวดซ้ำ"})
			break
		}
		seen[id] = true
	}
	if len(categoryIDs) < 1 || len(categoryIDs) > 3 {
		errs = append(errs, httpx.FieldError{Field: "category_ids", Message: "ต้องเลือก 1–3 หมวด"})
	}
	in.CategoryIDs = categoryIDs
	return errs, in
}

// Categories คืนหมวดทั้งหมดตามลำดับที่กำหนด (GET /api/categories)
func (s *Service) Categories(ctx context.Context) ([]Category, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, slug, name_th FROM categories ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Category])
}

// checkCategories ตรวจว่าทุก id มีอยู่จริง → ไม่ครบ 400 (03 §4.1 ขั้น 6)
func checkCategories(ctx context.Context, tx pgx.Tx, ids []int) error {
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM categories WHERE id = ANY($1)`, ids).Scan(&n); err != nil {
		return err
	}
	if n != len(ids) {
		return httpx.Validation("หมวดไม่ถูกต้อง", httpx.FieldError{Field: "category_ids", Message: "มีหมวดที่ไม่มีอยู่ในระบบ"})
	}
	return nil
}

func replaceCategories(ctx context.Context, tx pgx.Tx, restaurantID int64, ids []int) error {
	if _, err := tx.Exec(ctx, `DELETE FROM restaurant_categories WHERE restaurant_id = $1`, restaurantID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO restaurant_categories (restaurant_id, category_id)
		SELECT $1, unnest($2::smallint[])`, restaurantID, ids)
	return err
}

// nameTaken แปลง unique violation ของ restaurants_owner_name_uq เป็น 409
// ใช้ partial unique index ตัดสิน ไม่เช็คก่อนแล้วค่อย INSERT (ส่งพร้อมกัน 2 ครั้งจึงได้ร้านเดียว)
func nameTaken(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "restaurants_owner_name_uq" {
		return httpx.New("restaurant-name-taken", "คุณมีร้านชื่อนี้อยู่แล้ว")
	}
	return err
}

// Create สร้างร้านพร้อมหมวดและรูปใน transaction เดียว (03 §4.1 ขั้น 6–8)
// owner_id มาจาก session เท่านั้น
func (s *Service) Create(ctx context.Context, ownerID int64, in NewRestaurant, now time.Time) (int64, error) {
	var id int64
	err := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := checkCategories(ctx, tx, in.CategoryIDs); err != nil {
			return err
		}
		err := tx.QueryRow(ctx, `
			INSERT INTO restaurants (owner_id, name, description, capacity, open_time, close_time,
			                         cancel_window_minutes, min_lead_minutes, address, latitude, longitude, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $12)
			RETURNING id`,
			ownerID, in.Name, in.Description, in.Capacity, FormatClock(in.Open), FormatClock(in.Close),
			in.CancelWindowMinutes, in.MinLeadMinutes, in.Address, lat(in.Location), lng(in.Location), now).Scan(&id)
		if err != nil {
			return nameTaken(err)
		}
		if err := replaceCategories(ctx, tx, id, in.CategoryIDs); err != nil {
			return err
		}
		for _, img := range in.Images {
			if _, err := tx.Exec(ctx, `
				INSERT INTO restaurant_images (restaurant_id, content_type, data, created_at)
				VALUES ($1, $2, $3, $4)`, id, img.ContentType, img.Data, now); err != nil {
				return err
			}
		}
		return nil
	})
	return id, err
}

// Detail คืนรายละเอียดร้าน viewer = 0 คือไม่ได้ login (ร้านถูกลบ → 404)
func (s *Service) Detail(ctx context.Context, restaurantID, viewer int64) (Detail, error) {
	d := Detail{Categories: []Category{}, ImageIDs: []int64{}}
	// ค่าเฉลี่ยรายด้านเป็น NULL พร้อมกันทั้งสี่เมื่อร้านยังไม่มีรีวิว
	var food, ambience, service, value *float64
	var priceLevel *int
	err := s.Pool.QueryRow(ctx, `
		SELECT r.id, r.name, r.description, r.capacity,
		       to_char(r.open_time, 'HH24:MI'), to_char(r.close_time, 'HH24:MI'),
		       r.cancel_window_minutes, r.min_lead_minutes, r.accepting_bookings, r.owner_id = $2,
		       r.address, r.latitude, r.longitude,
		       EXISTS (SELECT 1 FROM favorites f WHERE f.restaurant_id = r.id AND f.user_id = $2),
		       (SELECT ROUND(AVG(rating), 1)::float8 FROM reviews WHERE restaurant_id = r.id),
		       (SELECT count(*) FROM reviews WHERE restaurant_id = r.id),
		       (SELECT ROUND(AVG(rating_food), 1)::float8     FROM reviews WHERE restaurant_id = r.id),
		       (SELECT ROUND(AVG(rating_ambience), 1)::float8 FROM reviews WHERE restaurant_id = r.id),
		       (SELECT ROUND(AVG(rating_service), 1)::float8  FROM reviews WHERE restaurant_id = r.id),
		       (SELECT ROUND(AVG(rating_value), 1)::float8    FROM reviews WHERE restaurant_id = r.id),
		       (SELECT count(*) FROM reviews WHERE restaurant_id = r.id AND price_level IS NOT NULL),
		       (SELECT PERCENTILE_DISC(0.5) WITHIN GROUP (ORDER BY price_level)
		          FROM reviews WHERE restaurant_id = r.id AND price_level IS NOT NULL)
		  FROM restaurants r
		 WHERE r.id = $1 AND r.deleted_at IS NULL`, restaurantID, viewer).
		Scan(&d.ID, &d.Name, &d.Description, &d.Capacity, &d.OpenTime, &d.CloseTime,
			&d.CancelWindowMinutes, &d.MinLeadMinutes, &d.AcceptingBookings, &d.IsOwner,
			&d.Address, &d.Latitude, &d.Longitude, &d.IsFavorite,
			&d.AvgRating, &d.ReviewCount, &food, &ambience, &service, &value, &d.PriceLevelCount, &priceLevel)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, httpx.New("not-found", "ไม่พบร้านนี้")
	}
	if err != nil {
		return d, err
	}
	if food != nil {
		d.AspectAverages = &ReviewAspects{Food: *food, Ambience: *ambience, Service: *service, Value: *value}
	}
	// ต้องมีคนระบุอย่างน้อย 3 คน ไม่อย่างนั้นคนเดียวตอบก็กลายเป็นราคาของร้านไปเลย
	if d.PriceLevelCount >= 3 {
		d.PriceLevel = priceLevel
	}

	rows, err := s.Pool.Query(ctx, `
		SELECT c.id, c.slug, c.name_th FROM restaurant_categories rc JOIN categories c ON c.id = rc.category_id
		 WHERE rc.restaurant_id = $1 ORDER BY c.sort_order`, restaurantID)
	if err != nil {
		return d, err
	}
	if d.Categories, err = pgx.CollectRows(rows, pgx.RowToStructByPos[Category]); err != nil {
		return d, err
	}

	// เลือกเฉพาะ id ของรูป ไม่ดึง data
	rows, err = s.Pool.Query(ctx, `SELECT id FROM restaurant_images WHERE restaurant_id = $1 ORDER BY id`, restaurantID)
	if err != nil {
		return d, err
	}
	if d.ImageIDs, err = pgx.CollectRows(rows, pgx.RowTo[int64]); err != nil {
		return d, err
	}

	// รีวิวของผู้ใช้ปัจจุบัน (รายการรีวิวแบ่งหน้าอาจไม่มีรีวิวนี้ในหน้าแรก)
	if viewer != 0 {
		var mr MyReview
		err := s.Pool.QueryRow(ctx, `
			SELECT id, rating::float8, rating_food, rating_ambience, rating_service, rating_value, comment
			  FROM reviews WHERE restaurant_id = $1 AND user_id = $2`,
			restaurantID, viewer).
			Scan(&mr.ID, &mr.Rating, &mr.Aspects.Food, &mr.Aspects.Ambience, &mr.Aspects.Service, &mr.Aspects.Value, &mr.Comment)
		if err == nil {
			d.MyReview = &mr
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return d, err
		}
	}
	return d, nil
}

// Mine คืนร้านของผู้ใช้ที่ไม่ถูกลบ ใหม่สุดก่อน (GET /api/me/restaurants)
func (s *Service) Mine(ctx context.Context, userID int64, now time.Time) ([]MyRestaurant, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT r.id, r.name,
		       (SELECT MIN(id) FROM restaurant_images WHERE restaurant_id = r.id),
		       r.capacity, to_char(r.open_time, 'HH24:MI'), to_char(r.close_time, 'HH24:MI'), r.accepting_bookings,
		       (SELECT ROUND(AVG(rating), 1)::float8 FROM reviews WHERE restaurant_id = r.id),
		       (SELECT count(*) FROM reviews WHERE restaurant_id = r.id),
		       (SELECT count(*) FROM bookings WHERE restaurant_id = r.id AND status = 'active' AND end_at > $2)
		  FROM restaurants r
		 WHERE r.owner_id = $1 AND r.deleted_at IS NULL
		 ORDER BY r.created_at DESC, r.id DESC`, userID, now)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[MyRestaurant])
	if list == nil {
		list = []MyRestaurant{}
	}
	return list, err
}

// UpdateInfo แก้ชื่อ รายละเอียด และแทนที่หมวดทั้งชุด (03 §4.2)
func (s *Service) UpdateInfo(ctx context.Context, userID, restaurantID int64, in Info, now time.Time) error {
	return db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockOwned(ctx, tx, restaurantID, userID); err != nil {
			return err
		}
		if err := checkCategories(ctx, tx, in.CategoryIDs); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			UPDATE restaurants SET name = $2, description = $3, address = $4, latitude = $5, longitude = $6, updated_at = $7
			 WHERE id = $1`,
			restaurantID, in.Name, in.Description, in.Address, lat(in.Location), lng(in.Location), now)
		if err != nil {
			return nameTaken(err)
		}
		return replaceCategories(ctx, tx, restaurantID, in.CategoryIDs)
	})
}

// lockOwned ล็อกแถวร้านแล้วตรวจว่าเป็นของผู้ใช้ (ไม่พบหรือถูกลบ 404 · คนอื่น 403)
func lockOwned(ctx context.Context, tx pgx.Tx, restaurantID, userID int64) error {
	r, err := db.LockRestaurant(ctx, tx, restaurantID)
	if errors.Is(err, db.ErrNotFound) {
		return httpx.New("not-found", "ไม่พบร้านนี้")
	}
	if err != nil {
		return err
	}
	if r.OwnerID != userID {
		return httpx.New("not-restaurant-owner", "จัดการได้เฉพาะร้านของตัวเอง")
	}
	return nil
}

// Delete ลบร้านแบบ soft delete (03 §4.5)
//
// ล็อกแถวร้านก่อนนับการจอง: การจองใหม่ต้องรอ lock เดียวกัน จึงแทรกระหว่างตรวจกับลบไม่ได้
// soft delete ทำให้ประวัติการจองของลูกค้ายังอยู่ และไม่ต้องกำหนด ON DELETE
func (s *Service) Delete(ctx context.Context, userID, restaurantID int64, now time.Time) error {
	return db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		r, err := db.LockRestaurant(ctx, tx, restaurantID)
		if errors.Is(err, db.ErrNotFound) {
			return httpx.New("not-found", "ไม่พบร้านนี้")
		}
		if err != nil {
			return err
		}
		if r.OwnerID != userID {
			return httpx.New("not-restaurant-owner", "ลบได้เฉพาะร้านของตัวเอง")
		}

		var upcoming int
		var latest *time.Time
		err = tx.QueryRow(ctx, `
			SELECT count(*), max(end_at) FROM bookings
			 WHERE restaurant_id = $1 AND status = 'active' AND end_at > $2`, r.ID, now).Scan(&upcoming, &latest)
		if err != nil {
			return err
		}
		if upcoming > 0 {
			return httpx.New("restaurant-has-upcoming-bookings",
				"ยังมีการจองที่จะมาถึง ลบได้หลังการจองสุดท้ายจบ หรือพักรับจองไว้ก่อน").
				With("upcoming_count", upcoming).
				With("latest_booking_end", latest.In(booking.Bangkok).Format(time.RFC3339)).
				With("accepting_bookings", r.AcceptingBookings)
		}

		_, err = tx.Exec(ctx, `UPDATE restaurants SET deleted_at = $2, updated_at = $2 WHERE id = $1`, r.ID, now)
		return err
	})
}
