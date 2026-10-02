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
	mux.HandleFunc("PATCH /api/bookings/{id}", h.update)
	mux.HandleFunc("POST /api/bookings/{id}/cancel", h.cancel)
	mux.HandleFunc("GET /api/me/bookings", h.mine)
	mux.HandleFunc("GET /api/restaurants/{id}/availability", h.availability)
}

// requireUser คืน userID หรือตอบ 401 แล้วคืน false
func requireUser(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, ok := auth.UserID(r.Context())
	if !ok {
		httpx.Write(w, httpx.New("unauthenticated", "ต้องเข้าสู่ระบบก่อน"))
	}
	return id, ok
}

// PATCH /api/bookings/{id} (01 §7.2)
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	bookingID, ok := pathID(r, "id")
	if !ok {
		httpx.Write(w, httpx.New("not-found", "ไม่พบการจองนี้"))
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
	b, err := h.Svc.Update(r.Context(), userID, bookingID, req, h.Now())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, b)
}

// POST /api/bookings/{id}/cancel (01 §7.3) ไม่มี body
func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	bookingID, ok := pathID(r, "id")
	if !ok {
		httpx.Write(w, httpx.New("not-found", "ไม่พบการจองนี้"))
		return
	}
	b, err := h.Svc.Cancel(r.Context(), userID, bookingID, h.Now())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, b)
}

// GET /api/me/bookings?tab=upcoming|past|cancelled&page=&page_size= (01 §7.4)
func (h *Handler) mine(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	tab, err := ValidTab(r.URL.Query().Get("tab"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	page, err := httpx.ParsePage(r, 20, 50)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	res, err := h.Svc.MyBookings(r.Context(), userID, tab, page, h.Now())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// GET /api/restaurants/{id}/availability?date=YYYY-MM-DD (01 §7.5) สาธารณะ
func (h *Handler) availability(w http.ResponseWriter, r *http.Request) {
	restaurantID, ok := pathID(r, "id")
	if !ok {
		httpx.Write(w, httpx.New("not-found", "ไม่พบร้านนี้"))
		return
	}
	day, err := ParseDate("date", r.URL.Query().Get("date"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a, err := h.Svc.Availability(r.Context(), restaurantID, day, h.Now())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a)
}

// requestBody คือ JSON ที่รับ เวลารับเป็นสตริงเพื่อบอกได้ว่าฟิลด์ไหน parse ไม่ได้
type requestBody struct {
	PartySize int    `json:"party_size"`
	StartAt   string `json:"start_at"`
	EndAt     string `json:"end_at"`
}

// POST /api/restaurants/{id}/bookings (01 §7.1)
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
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
