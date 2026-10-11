// Package review คือรีวิวร้าน (01 §7.7) และรายการร้านเรียงตามคะแนน (01 §7.8)
package review

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"devpool/internal/booking"
	"devpool/internal/db"
	"devpool/internal/httpx"
)

type Service struct {
	Pool *pgxpool.Pool
}

// Aspects คือคะแนน 4 ด้านที่ผู้รีวิวให้ ทุกด้านเป็นจำนวนเต็ม 1–5
type Aspects struct {
	Food     int `json:"food"`
	Ambience int `json:"ambience"`
	Service  int `json:"service"`
	Value    int `json:"value"`
}

// aspectFields จับคู่ชื่อฟิลด์ใน JSON กับตัวชี้ไปยังค่าในโครงสร้าง ใช้ทั้งตอนตรวจและตอนอ่านค่าเฉลี่ย
func (a *Aspects) fields() []struct {
	name string
	val  *int
} {
	return []struct {
		name string
		val  *int
	}{
		{"food", &a.Food},
		{"ambience", &a.Ambience},
		{"service", &a.Service},
		{"value", &a.Value},
	}
}

type Review struct {
	ID int64 `json:"id"`
	// คะแนนรวมเป็นค่าเฉลี่ยของ 4 ด้าน ฐานข้อมูลคำนวณให้เองด้วย generated column
	// จึงไม่มีทางที่คะแนนรวมกับรายด้านจะไม่ตรงกัน
	Rating  float64 `json:"rating"`
	Aspects Aspects `json:"aspects"`
	// ช่วงราคาต่อคน 1–5 · nil = ผู้รีวิวไม่ได้ระบุ
	PriceLevel  *int      `json:"price_level"`
	Comment     string    `json:"comment"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
	Edited      bool      `json:"edited"`
	IsMine      bool      `json:"is_mine"`
}

// Input คือ body ของการสร้างหรือแก้รีวิว
type Input struct {
	Aspects    Aspects `json:"aspects"`
	PriceLevel *int    `json:"price_level"`
	Comment    string  `json:"comment"`
}

// validate ตรวจรูปแบบ (400) และคืน comment ที่ตัดช่องว่างหัวท้ายแล้ว
func (in Input) validate() (Input, error) {
	var errs []httpx.FieldError
	for _, f := range in.Aspects.fields() {
		if *f.val < 1 || *f.val > 5 {
			errs = append(errs, httpx.FieldError{Field: "aspects." + f.name, Message: "ต้องเป็นจำนวนเต็ม 1–5"})
		}
	}
	if in.PriceLevel != nil && (*in.PriceLevel < 1 || *in.PriceLevel > 5) {
		errs = append(errs, httpx.FieldError{Field: "price_level", Message: "ต้องเป็นจำนวนเต็ม 1–5 หรือไม่ส่งมา"})
	}
	in.Comment = strings.TrimSpace(in.Comment)
	if n := utf8.RuneCountInString(in.Comment); n < 1 || n > 1000 {
		errs = append(errs, httpx.FieldError{Field: "comment", Message: "ต้องยาว 1–1,000 ตัวอักษร"})
	}
	if len(errs) > 0 {
		return in, httpx.Validation("ข้อมูลรีวิวไม่ถูกต้อง", errs...)
	}
	return in, nil
}

// Create สร้างรีวิว (1 คน 1 รีวิวต่อร้าน รีวิวร้านตัวเองไม่ได้)
//
// ไม่เช็คก่อนว่าเคยรีวิวหรือยัง แต่ INSERT เลยแล้วให้ unique (user_id, restaurant_id) ตัดสิน
// ถ้าเช็คก่อน คำขอสองรายการที่มาพร้อมกันจะผ่านการเช็คทั้งคู่แล้ว INSERT ซ้ำได้
func (s *Service) Create(ctx context.Context, userID, restaurantID int64, in Input, now time.Time) (Review, error) {
	in, err := in.validate()
	if err != nil {
		return Review{}, err
	}
	r, err := db.GetRestaurant(ctx, s.Pool, restaurantID)
	if errors.Is(err, db.ErrNotFound) {
		return Review{}, httpx.New("not-found", "ไม่พบร้านนี้")
	}
	if err != nil {
		return Review{}, err
	}
	if r.OwnerID == userID {
		return Review{}, httpx.New("own-restaurant-review", "รีวิวร้านของตัวเองไม่ได้")
	}

	rv := Review{Aspects: in.Aspects, PriceLevel: in.PriceLevel, Comment: in.Comment, IsMine: true}
	a := in.Aspects
	err = s.Pool.QueryRow(ctx, `
		WITH ins AS (
		  INSERT INTO reviews (restaurant_id, user_id, rating_food, rating_ambience, rating_service, rating_value,
		                       price_level, comment, created_at, updated_at)
		  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
		  RETURNING id, user_id, rating, created_at)
		SELECT ins.id, ins.rating::float8, ins.created_at, u.display_name
		  FROM ins JOIN users u ON u.id = ins.user_id`,
		restaurantID, userID, a.Food, a.Ambience, a.Service, a.Value, in.PriceLevel, in.Comment, now).
		Scan(&rv.ID, &rv.Rating, &rv.CreatedAt, &rv.DisplayName)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return Review{}, httpx.New("already-reviewed", "คุณรีวิวร้านนี้แล้ว แก้ไขรีวิวเดิมได้")
	}
	if err != nil {
		return Review{}, err
	}
	rv.CreatedAt = rv.CreatedAt.In(booking.Bangkok)
	return rv, nil
}

// Update แก้รีวิวของตัวเอง ไม่ใช่ผู้เขียนหรือร้านถูกลบ → 404
func (s *Service) Update(ctx context.Context, userID, reviewID int64, in Input, now time.Time) (Review, error) {
	in, err := in.validate()
	if err != nil {
		return Review{}, err
	}
	rv := Review{IsMine: true}
	var updatedAt time.Time
	a := in.Aspects
	err = s.Pool.QueryRow(ctx, `
		UPDATE reviews v SET rating_food = $3, rating_ambience = $4, rating_service = $5, rating_value = $6,
		                     price_level = $7, comment = $8, updated_at = $9
		  FROM restaurants r, users u
		 WHERE v.id = $1 AND v.user_id = $2
		   AND r.id = v.restaurant_id AND r.deleted_at IS NULL AND u.id = v.user_id
		RETURNING v.id, v.rating::float8, v.rating_food, v.rating_ambience, v.rating_service, v.rating_value,
		          v.price_level, v.comment, u.display_name, v.created_at, v.updated_at`,
		reviewID, userID, a.Food, a.Ambience, a.Service, a.Value, in.PriceLevel, in.Comment, now).
		Scan(&rv.ID, &rv.Rating, &rv.Aspects.Food, &rv.Aspects.Ambience, &rv.Aspects.Service, &rv.Aspects.Value,
			&rv.PriceLevel, &rv.Comment, &rv.DisplayName, &rv.CreatedAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Review{}, httpx.New("not-found", "ไม่พบรีวิวนี้")
	}
	if err != nil {
		return Review{}, err
	}
	rv.Edited = updatedAt.After(rv.CreatedAt)
	rv.CreatedAt = rv.CreatedAt.In(booking.Bangkok)
	return rv, nil
}

// Delete ลบรีวิวของตัวเอง (ไม่มีทางให้เจ้าของร้านลบรีวิวในร้านตัวเอง)
func (s *Service) Delete(ctx context.Context, userID, reviewID int64) error {
	tag, err := s.Pool.Exec(ctx, `
		DELETE FROM reviews v USING restaurants r
		 WHERE v.id = $1 AND v.user_id = $2
		   AND r.id = v.restaurant_id AND r.deleted_at IS NULL`,
		reviewID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.New("not-found", "ไม่พบรีวิวนี้")
	}
	return nil
}

// List คืนรีวิวของร้านแบบแบ่งหน้า ใหม่สุดก่อน viewer = 0 คือไม่ได้ login
func (s *Service) List(ctx context.Context, restaurantID, viewer int64, page httpx.Page) (httpx.PageResult[Review], error) {
	var res httpx.PageResult[Review]
	if _, err := db.GetRestaurant(ctx, s.Pool, restaurantID); errors.Is(err, db.ErrNotFound) {
		return res, httpx.New("not-found", "ไม่พบร้านนี้")
	} else if err != nil {
		return res, err
	}

	var total int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM reviews WHERE restaurant_id = $1`, restaurantID).Scan(&total); err != nil {
		return res, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT v.id, v.rating::float8, v.rating_food, v.rating_ambience, v.rating_service, v.rating_value,
		       v.price_level, v.comment, u.display_name, v.created_at,
		       v.updated_at > v.created_at, v.user_id = $2
		  FROM reviews v JOIN users u ON u.id = v.user_id
		 WHERE v.restaurant_id = $1
		 ORDER BY v.created_at DESC, v.id DESC
		 LIMIT $3 OFFSET $4`,
		restaurantID, viewer, page.Size, page.Offset())
	if err != nil {
		return res, err
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Review, error) {
		var rv Review
		err := row.Scan(&rv.ID, &rv.Rating, &rv.Aspects.Food, &rv.Aspects.Ambience, &rv.Aspects.Service, &rv.Aspects.Value,
			&rv.PriceLevel, &rv.Comment, &rv.DisplayName, &rv.CreatedAt, &rv.Edited, &rv.IsMine)
		rv.CreatedAt = rv.CreatedAt.In(booking.Bangkok)
		return rv, err
	})
	if err != nil {
		return res, err
	}
	return httpx.NewPageResult(items, page, total), nil
}
