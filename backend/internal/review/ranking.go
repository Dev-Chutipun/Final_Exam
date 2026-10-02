package review

import (
	"context"

	"github.com/jackc/pgx/v5"

	"devpool/internal/httpx"
)

type Category struct {
	ID     int    `json:"id"`
	Slug   string `json:"slug"`
	NameTH string `json:"name_th"`
}

// ListItem คือร้านหนึ่งรายการใน GET /api/restaurants (01 §7.8)
type ListItem struct {
	ID                int64      `json:"id"`
	Name              string     `json:"name"`
	Capacity          int        `json:"capacity"`
	OpenTime          string     `json:"open_time"`
	CloseTime         string     `json:"close_time"`
	AvgRating         *float64   `json:"avg_rating"` // null เมื่อยังไม่มีรีวิว
	ReviewCount       int        `json:"review_count"`
	AcceptingBookings bool       `json:"accepting_bookings"`
	Categories        []Category `json:"categories"`
	CoverImageID      *int64     `json:"cover_image_id"` // รูปที่ id น้อยที่สุดของร้าน
	IsMine            bool       `json:"is_mine"`
}

// sorts แมปค่า sort กับ ORDER BY เป็นสตริงคงที่ ทุกแบบมี id เป็นตัวตัดสินสุดท้าย
// เพื่อให้ลำดับคงที่และแบ่งหน้าได้ถูกต้อง
var sorts = map[string]string{
	"highest_rated": "score DESC NULLS LAST, review_count DESC, id ASC",
	"most_reviewed": "review_count DESC, avg_raw DESC NULLS LAST, id ASC",
}

// ValidSort ตรวจค่า sort ค่าว่างคือ highest_rated
func ValidSort(sort string) (string, error) {
	if sort == "" {
		return "highest_rated", nil
	}
	if _, ok := sorts[sort]; !ok {
		return "", httpx.Validation("การเรียงไม่ถูกต้อง",
			httpx.FieldError{Field: "sort", Message: "ต้องเป็น highest_rated หรือ most_reviewed"})
	}
	return sort, nil
}

// ListRestaurants คืนร้านที่ไม่ถูกลบ เรียงตามคะแนน viewer = 0 คือไม่ได้ login
//
// Bayesian score = (C × m + SUM(rating)) / (C + COUNT) โดย C = 5
// m = ค่าเฉลี่ยรีวิวทั้งระบบของร้านที่ไม่ถูกลบ (ยังไม่มีรีวิวเลยใช้ 3.0)
// ทำให้ 5.0 จาก 1 รีวิวไม่ชนะ 4.8 จาก 300 รีวิว: ร้านรีวิวน้อยถูกดึงเข้าหาค่าเฉลี่ยกลาง
func (s *Service) ListRestaurants(ctx context.Context, sort string, viewer int64, page httpx.Page) (httpx.PageResult[ListItem], error) {
	var res httpx.PageResult[ListItem]

	var total int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM restaurants WHERE deleted_at IS NULL`).Scan(&total); err != nil {
		return res, err
	}

	rows, err := s.Pool.Query(ctx, `
		WITH g AS (
		  SELECT COALESCE(AVG(v.rating), 3.0) AS m
		    FROM reviews v JOIN restaurants r ON r.id = v.restaurant_id
		   WHERE r.deleted_at IS NULL
		), s AS (
		  SELECT r.id, r.name, r.capacity, r.open_time, r.close_time, r.accepting_bookings, r.owner_id,
		         COUNT(v.id)                     AS review_count,
		         AVG(v.rating)                   AS avg_raw,
		         ROUND(AVG(v.rating), 1)::float8 AS avg_rating,
		         CASE WHEN COUNT(v.id) = 0 THEN NULL
		              ELSE (5 * g.m + SUM(v.rating)) / (5 + COUNT(v.id)) END AS score
		    FROM restaurants r
		   CROSS JOIN g
		    LEFT JOIN reviews v ON v.restaurant_id = r.id
		   WHERE r.deleted_at IS NULL
		   GROUP BY r.id, g.m
		)
		SELECT id, name, capacity, to_char(open_time, 'HH24:MI'), to_char(close_time, 'HH24:MI'),
		       accepting_bookings, owner_id = $3, review_count, avg_rating
		  FROM s
		 ORDER BY `+sorts[sort]+`
		 LIMIT $1 OFFSET $2`,
		page.Size, page.Offset(), viewer)
	if err != nil {
		return res, err
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ListItem, error) {
		it := ListItem{Categories: []Category{}}
		err := row.Scan(&it.ID, &it.Name, &it.Capacity, &it.OpenTime, &it.CloseTime,
			&it.AcceptingBookings, &it.IsMine, &it.ReviewCount, &it.AvgRating)
		return it, err
	})
	if err != nil {
		return res, err
	}
	if err := s.attachCategoriesAndCovers(ctx, items); err != nil {
		return res, err
	}
	return httpx.NewPageResult(items, page, total), nil
}

// attachCategoriesAndCovers ดึงหมวดและรูปปกของร้านในหน้านี้ด้วย query แยก (ครั้งละหนึ่ง query ไม่ใช่ทีละร้าน)
// ห้ามดึง restaurant_images.data ในรายการ: เลือกแค่ id ของรูป
func (s *Service) attachCategoriesAndCovers(ctx context.Context, items []ListItem) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]int64, len(items))
	byID := make(map[int64]*ListItem, len(items))
	for i := range items {
		ids[i] = items[i].ID
		byID[items[i].ID] = &items[i]
	}

	rows, err := s.Pool.Query(ctx, `
		SELECT rc.restaurant_id, c.id, c.slug, c.name_th
		  FROM restaurant_categories rc JOIN categories c ON c.id = rc.category_id
		 WHERE rc.restaurant_id = ANY($1)
		 ORDER BY c.sort_order`, ids)
	if err != nil {
		return err
	}
	for rows.Next() {
		var rid int64
		var c Category
		if err := rows.Scan(&rid, &c.ID, &c.Slug, &c.NameTH); err != nil {
			rows.Close()
			return err
		}
		byID[rid].Categories = append(byID[rid].Categories, c)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	rows, err = s.Pool.Query(ctx, `
		SELECT restaurant_id, MIN(id) FROM restaurant_images
		 WHERE restaurant_id = ANY($1) GROUP BY restaurant_id`, ids)
	if err != nil {
		return err
	}
	for rows.Next() {
		var rid, cover int64
		if err := rows.Scan(&rid, &cover); err != nil {
			rows.Close()
			return err
		}
		byID[rid].CoverImageID = &cover
	}
	return rows.Err()
}
