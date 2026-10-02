// Package httpx รวมตัวช่วย HTTP ที่ทุก handler ใช้: problem+json (RFC 9457) และการอ่าน JSON แบบเข้มงวด
package httpx

import (
	"encoding/json"
	"net/http"
)

// problemDef คือ status และ title ของแต่ละ type ตาม docs/00-overview.md §5
type problemDef struct {
	status int
	title  string
}

// problems ต้องตรงกับตารางใน 00-overview.md §5 ทุกแถว (มี test ตรวจ)
var problems = map[string]problemDef{
	"validation":                        {http.StatusBadRequest, "ข้อมูลไม่ถูกต้อง"},
	"unauthenticated":                   {http.StatusUnauthorized, "ต้องเข้าสู่ระบบ"},
	"own-restaurant-booking":            {http.StatusForbidden, "จองร้านของตัวเองไม่ได้"},
	"own-restaurant-review":             {http.StatusForbidden, "รีวิวร้านของตัวเองไม่ได้"},
	"not-restaurant-owner":              {http.StatusForbidden, "ไม่ใช่เจ้าของร้าน"},
	"cross-origin-request":              {http.StatusForbidden, "คำขอมาจากเว็บอื่น"},
	"not-found":                         {http.StatusNotFound, "ไม่พบข้อมูล"},
	"image-too-large":                   {http.StatusRequestEntityTooLarge, "ไฟล์ใหญ่เกินกำหนด"},
	"unsupported-image-type":            {http.StatusUnsupportedMediaType, "ชนิดรูปไม่รองรับ"},
	"unsupported-media-type":            {http.StatusUnsupportedMediaType, "Content-Type ไม่ถูกต้อง"},
	"booking-in-past":                   {http.StatusUnprocessableEntity, "เวลาที่จองผ่านไปแล้ว"},
	"lead-time-not-met":                 {http.StatusUnprocessableEntity, "จองล่วงหน้าไม่พอ"},
	"too-far-in-advance":                {http.StatusUnprocessableEntity, "จองล่วงหน้าเกิน 30 วัน"},
	"outside-opening-hours":             {http.StatusUnprocessableEntity, "อยู่นอกเวลาเปิดร้าน"},
	"party-exceeds-capacity":            {http.StatusUnprocessableEntity, "จำนวนคนเกินที่นั่งทั้งร้าน"},
	"cancel-deadline-passed":            {http.StatusUnprocessableEntity, "เลยเวลาที่แก้ไขหรือยกเลิกได้"},
	"invalid-restaurant-policy":         {http.StatusUnprocessableEntity, "นโยบายร้านไม่ถูกต้อง"},
	"overlapping-own-booking":           {http.StatusConflict, "คุณมีการจองช่วงเวลานี้อยู่แล้ว"},
	"seat-capacity-exceeded":            {http.StatusConflict, "ที่นั่งไม่พอ"},
	"booking-not-active":                {http.StatusConflict, "การจองนี้ถูกยกเลิกแล้ว"},
	"already-cancelled":                 {http.StatusConflict, "ยกเลิกไปแล้ว"},
	"already-reviewed":                  {http.StatusConflict, "คุณรีวิวร้านนี้แล้ว"},
	"settings-conflict-with-bookings":   {http.StatusConflict, "ตั้งค่าใหม่ชนกับการจองที่มีอยู่"},
	"restaurant-not-accepting-bookings": {http.StatusConflict, "ร้านพักรับจองอยู่"},
	"restaurant-name-taken":             {http.StatusConflict, "ชื่อร้านซ้ำ"},
	"restaurant-has-upcoming-bookings":  {http.StatusConflict, "ร้านยังมีการจองที่จะมาถึง"},
	"image-limit-reached":               {http.StatusConflict, "มีรูปครบ 3 รูปแล้ว"},
	"last-image":                        {http.StatusConflict, "ลบรูปสุดท้ายไม่ได้"},
}

// FieldError คือรายการใน errors[] ของ problem แบบ validation
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Problem คือ error ที่ตอบกลับเป็น application/problem+json
// เป็น error ด้วย service จึงคืนค่านี้ได้ตรงๆ แล้ว handler ส่งต่อด้วย Write
type Problem struct {
	Type   string // ไม่มี /problems/ นำหน้า เช่น "seat-capacity-exceeded"
	Title  string
	Status int
	Detail string
	Errors []FieldError    // ใช้กับ validation เท่านั้น
	Extra  map[string]any // ฟิลด์เพิ่ม เช่น available, full_from, conflicts
}

// New สร้าง Problem จาก type ในตาราง §5
// type ที่ไม่มีในตารางคือ bug ของโปรแกรม จึง panic ให้ test จับได้ทันที
func New(typ, detail string) *Problem {
	def, ok := problems[typ]
	if !ok {
		panic("httpx: unknown problem type " + typ)
	}
	return &Problem{Type: typ, Title: def.title, Status: def.status, Detail: detail}
}

// Validation สร้าง problem 400 พร้อมรายการฟิลด์ที่ผิด
func Validation(detail string, errs ...FieldError) *Problem {
	p := New("validation", detail)
	p.Errors = errs
	if p.Errors == nil {
		p.Errors = []FieldError{} // ส่ง [] เสมอ ไม่ส่ง null
	}
	return p
}

// With เพิ่มฟิลด์ extension ของ RFC 9457
func (p *Problem) With(key string, value any) *Problem {
	if p.Extra == nil {
		p.Extra = map[string]any{}
	}
	p.Extra[key] = value
	return p
}

func (p *Problem) Error() string {
	if p.Detail != "" {
		return p.Type + ": " + p.Detail
	}
	return p.Type
}

func (p *Problem) MarshalJSON() ([]byte, error) {
	m := make(map[string]any, len(p.Extra)+5)
	for k, v := range p.Extra {
		m[k] = v
	}
	m["type"] = "/problems/" + p.Type
	m["title"] = p.Title
	m["status"] = p.Status
	if p.Detail != "" {
		m["detail"] = p.Detail
	}
	if p.Type == "validation" {
		errs := p.Errors
		if errs == nil {
			errs = []FieldError{}
		}
		m["errors"] = errs
	}
	return json.Marshal(m)
}

// Write ตอบ problem พร้อม Content-Type: application/problem+json
func Write(w http.ResponseWriter, p *Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

// WriteJSON ตอบ JSON ปกติ
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
