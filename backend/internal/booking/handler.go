package booking

import (
	"net/http"
	"strconv"
	"time"

	"devpool/internal/auth"
	"devpool/internal/httpx"
)

// Handler แปลง HTTP ↔ Service
// Now เป็นที่เดียวที่อ่านเวลาปัจจุบัน (main ใส่ time.Now, test ใส่เวลาคงที่)
type Handler struct {
	Svc *Service
	Now func() time.Time
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/restaurants/{id}/bookings", h.create)
}

// requestBody คือ JSON ที่รับ เวลารับเป็นสตริงเพื่อบอกได้ว่าฟิลด์ไหน parse ไม่ได้
type requestBody struct {
	PartySize int    `json:"party_size"`
	StartAt   string `json:"start_at"`
	EndAt     string `json:"end_at"`
}

// POST /api/restaurants/{id}/bookings (01 §7.1)
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		httpx.Write(w, httpx.New("unauthenticated", "ต้องเข้าสู่ระบบก่อนจอง"))
		return
	}
	restaurantID, ok := pathID(r, "id")
	if !ok {
		httpx.Write(w, httpx.New("not-found", "ไม่พบร้านนี้"))
		return
	}
	var body requestBody
	if !httpx.ReadJSON(w, r, &body) {
		return
	}
	req, err := body.parse()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	b, err := h.Svc.Create(r.Context(), userID, restaurantID, req, h.Now())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, b)
}

// parse แปลงสตริงเวลาแบบ RFC 3339 (ต้องมี offset เช่น +07:00)
func (b requestBody) parse() (Request, error) {
	var errs []httpx.FieldError
	start, err := time.Parse(time.RFC3339, b.StartAt)
	if err != nil {
		errs = append(errs, httpx.FieldError{Field: "start_at", Message: "ต้องเป็นเวลาแบบ RFC 3339 เช่น 2026-10-11T12:00:00+07:00"})
	}
	end, err := time.Parse(time.RFC3339, b.EndAt)
	if err != nil {
		errs = append(errs, httpx.FieldError{Field: "end_at", Message: "ต้องเป็นเวลาแบบ RFC 3339 เช่น 2026-10-11T13:00:00+07:00"})
	}
	if len(errs) > 0 {
		return Request{}, httpx.Validation("รูปแบบเวลาไม่ถูกต้อง", errs...)
	}
	return Request{PartySize: b.PartySize, StartAt: start, EndAt: end}, nil
}

// pathID อ่าน id จาก path ค่าที่ไม่ใช่จำนวนเต็มบวกถือว่าไม่พบ
func pathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id, err == nil && id > 0
}
