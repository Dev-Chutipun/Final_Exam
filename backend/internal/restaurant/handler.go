package restaurant

import (
	"net/http"
	"strconv"
	"time"

	"devpool/internal/auth"
	"devpool/internal/httpx"
)

// Handler แปลง HTTP ↔ Service · Now เป็นที่เดียวที่อ่านเวลาปัจจุบัน
type Handler struct {
	Svc *Service
	Now func() time.Time
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("PATCH /api/restaurants/{id}/settings", h.updateSettings)
}

func requireUser(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, ok := auth.UserID(r.Context())
	if !ok {
		httpx.Write(w, httpx.New("unauthenticated", "ต้องเข้าสู่ระบบก่อน"))
	}
	return id, ok
}

func pathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id, err == nil && id > 0
}

// settingsBody ใช้ pointer เพื่อแยก "ไม่ได้ส่งมา" (400) ออกจาก "ส่ง 0 มา"
type settingsBody struct {
	Capacity            *int    `json:"capacity"`
	OpenTime            *string `json:"open_time"`
	CloseTime           *string `json:"close_time"`
	CancelWindowMinutes *int    `json:"cancel_window_minutes"`
	MinLeadMinutes      *int    `json:"min_lead_minutes"`
}

// PATCH /api/restaurants/{id}/settings (01 §7.6) ส่งครบทุกค่า
func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	restaurantID, ok := pathID(r, "id")
	if !ok {
		httpx.Write(w, httpx.New("not-found", "ไม่พบร้านนี้"))
		return
	}
	var body settingsBody
	if !httpx.ReadJSON(w, r, &body) {
		return
	}

	errs, open, close := validateCapacityAndHours(nil, body.Capacity, body.OpenTime, body.CloseTime)
	if body.CancelWindowMinutes == nil {
		errs = append(errs, httpx.FieldError{Field: "cancel_window_minutes", Message: "จำเป็นต้องระบุ"})
	}
	if body.MinLeadMinutes == nil {
		errs = append(errs, httpx.FieldError{Field: "min_lead_minutes", Message: "จำเป็นต้องระบุ"})
	}
	if len(errs) > 0 {
		httpx.Write(w, httpx.Validation("ข้อมูลตั้งค่าไม่ถูกต้อง", errs...))
		return
	}

	res, err := h.Svc.UpdateSettings(r.Context(), userID, restaurantID, Settings{
		Capacity:            *body.Capacity,
		Open:                open,
		Close:               close,
		CancelWindowMinutes: *body.CancelWindowMinutes,
		MinLeadMinutes:      *body.MinLeadMinutes,
	}, h.Now())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}
