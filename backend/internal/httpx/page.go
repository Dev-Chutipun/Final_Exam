package httpx

import (
	"net/http"
	"strconv"
)

// Page คือค่าการแบ่งหน้าที่อ่านจาก query string
type Page struct {
	Page, Size int
}

// Offset คือจำนวนแถวที่ข้ามสำหรับ SQL OFFSET
func (p Page) Offset() int { return (p.Page - 1) * p.Size }

// PageResult คือรูปแบบคำตอบของทุก endpoint รายการ: { items, page, page_size, total }
type PageResult[T any] struct {
	Items    []T `json:"items"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

// NewPageResult ทำให้ items เป็น [] เสมอ ไม่ใช่ null
func NewPageResult[T any](items []T, p Page, total int) PageResult[T] {
	if items == nil {
		items = []T{}
	}
	return PageResult[T]{Items: items, Page: p.Page, PageSize: p.Size, Total: total}
}

// ParsePage อ่าน page และ page_size (00-overview.md §2 ข้อ 16)
// ไม่ส่งมา → ค่าเริ่มต้น · ไม่ใช่จำนวนเต็ม, page < 1, page_size < 1 หรือเกิน max → 400 (ไม่ปรับค่าให้เอง)
func ParsePage(r *http.Request, defaultSize, maxSize int) (Page, error) {
	p := Page{Page: 1, Size: defaultSize}
	var errs []FieldError
	q := r.URL.Query()

	if v := q.Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			errs = append(errs, FieldError{Field: "page", Message: "ต้องเป็นจำนวนเต็มตั้งแต่ 1"})
		}
		p.Page = n
	}
	if v := q.Get("page_size"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxSize {
			errs = append(errs, FieldError{Field: "page_size", Message: "ต้องเป็นจำนวนเต็ม 1–" + strconv.Itoa(maxSize)})
		}
		p.Size = n
	}
	if len(errs) > 0 {
		return Page{}, Validation("ค่าการแบ่งหน้าไม่ถูกต้อง", errs...)
	}
	return p, nil
}
