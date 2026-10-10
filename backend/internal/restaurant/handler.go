package restaurant

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"devpool/internal/auth"
	"devpool/internal/booking"
	"devpool/internal/httpx"
)

// Handler แปลง HTTP ↔ Service · Now เป็นที่เดียวที่อ่านเวลาปัจจุบัน
type Handler struct {
	Svc *Service
	Now func() time.Time
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/categories", h.categories)
	mux.HandleFunc("POST /api/restaurants", h.create)
	mux.HandleFunc("GET /api/restaurants/{id}", h.detail)
	mux.HandleFunc("PATCH /api/restaurants/{id}", h.updateInfo)
	mux.HandleFunc("DELETE /api/restaurants/{id}", h.delete)
	mux.HandleFunc("PATCH /api/restaurants/{id}/settings", h.updateSettings)
	mux.HandleFunc("POST /api/restaurants/{id}/images", h.addImage)
	mux.HandleFunc("DELETE /api/restaurants/{id}/images/{imageId}", h.deleteImage)
	mux.HandleFunc("GET /api/restaurants/{id}/images/{imageId}", h.image)
	mux.HandleFunc("GET /api/me/restaurants", h.mine)
	mux.HandleFunc("PATCH /api/restaurants/{id}/booking-status", h.bookingStatus)
	mux.HandleFunc("GET /api/restaurants/{id}/bookings", h.restaurantBookings)
}

// PATCH /api/restaurants/{id}/booking-status [B] body { "accepting_bookings": false }
func (h *Handler) bookingStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		httpx.Write(w, httpx.New("not-found", "ไม่พบร้านนี้"))
		return
	}
	var body struct {
		AcceptingBookings *bool `json:"accepting_bookings"`
	}
	if !httpx.ReadJSON(w, r, &body) {
		return
	}
	if body.AcceptingBookings == nil {
		httpx.Write(w, httpx.Validation("ข้อมูลไม่ครบ",
			httpx.FieldError{Field: "accepting_bookings", Message: "จำเป็นต้องระบุ"}))
		return
	}
	if err := h.Svc.SetAcceptingBookings(r.Context(), userID, id, *body.AcceptingBookings, h.Now()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "accepting_bookings": *body.AcceptingBookings})
}

// GET /api/restaurants/{id}/bookings [B] เฉพาะเจ้าของร้าน (03 §4.7)
//
//	?date=YYYY-MM-DD              → การจองของวันทำการนั้น พร้อม peak (มุมมองหน้างาน)
//	?tab=upcoming|past|cancelled  → รายการทั้งหมดแบ่งหน้า (ค่าเริ่มต้นเมื่อไม่ส่ง date)
func (h *Handler) restaurantBookings(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		httpx.Write(w, httpx.New("not-found", "ไม่พบร้านนี้"))
		return
	}
	if date := r.URL.Query().Get("date"); date != "" {
		day, err := booking.ParseDate("date", date)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		res, err := h.Svc.DailyBookings(r.Context(), userID, id, day)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, res)
		return
	}
	tab, err := ValidBookingTab(r.URL.Query().Get("tab"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	page, err := httpx.ParsePage(r, 20, 50)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	res, err := h.Svc.Bookings(r.Context(), userID, id, tab, page, h.Now())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// เพดานขนาด body ของ multipart (03 §4.1, §4.3)
const (
	maxCreateBody = 7 << 20
	maxImageBody  = 3 << 20
)

// viewer คือผู้ใช้ปัจจุบันสำหรับ endpoint สาธารณะ (0 = ไม่ได้ login)
func viewer(r *http.Request) int64 {
	id, _ := auth.UserID(r.Context())
	return id
}

// GET /api/categories (สาธารณะ)
func (h *Handler) categories(w http.ResponseWriter, r *http.Request) {
	list, err := h.Svc.Categories(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// createData คือฟิลด์ data (JSON) ของ multipart ตอนสร้างร้าน
// ไม่มี owner_id: ส่งมาจะเป็นฟิลด์ที่ไม่รู้จัก → 400
type createData struct {
	Name                *string  `json:"name"`
	Description         *string  `json:"description"`
	Capacity            *int     `json:"capacity"`
	OpenTime            *string  `json:"open_time"`
	CloseTime           *string  `json:"close_time"`
	CancelWindowMinutes *int     `json:"cancel_window_minutes"`
	MinLeadMinutes      *int     `json:"min_lead_minutes"`
	CategoryIDs         []int    `json:"category_ids"`
	Latitude            *float64 `json:"latitude"`
	Longitude           *float64 `json:"longitude"`
}

// POST /api/restaurants (03 §4.1) multipart: data (JSON) + images 1–3 ไฟล์
// อ่านและตรวจไฟล์ในหน่วยความจำให้เสร็จก่อนเข้า transaction
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	// 2: multipart และ body รวม ≤ 7 MB
	if err := httpx.ParseMultipart(w, r, maxCreateBody); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer r.MultipartForm.RemoveAll()

	// 3: data ถูกรูปแบบ
	var data createData
	if err := httpx.DecodeJSON(strings.NewReader(r.FormValue("data")), &data); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	errs, info := validateInfo(nil, data.Name, data.Description, data.CategoryIDs, data.Latitude, data.Longitude)
	errs, open, close := validateCapacityAndHours(errs, data.Capacity, data.OpenTime, data.CloseTime)
	files := r.MultipartForm.File["images"]
	if len(files) < 1 || len(files) > MaxImages {
		errs = append(errs, httpx.FieldError{Field: "images", Message: "ต้องมีรูป 1–3 รูป"})
	}
	if len(errs) > 0 {
		httpx.Write(w, httpx.Validation("ข้อมูลร้านไม่ถูกต้อง", errs...))
		return
	}

	// 4: นโยบายร้าน (ไม่ส่งมาใช้ค่าเริ่มต้น)
	in := NewRestaurant{Info: info, Settings: Settings{
		Capacity: *data.Capacity, Open: open, Close: close,
		CancelWindowMinutes: DefaultCancelWindow, MinLeadMinutes: DefaultMinLead,
	}}
	if data.CancelWindowMinutes != nil {
		in.CancelWindowMinutes = *data.CancelWindowMinutes
	}
	if data.MinLeadMinutes != nil {
		in.MinLeadMinutes = *data.MinLeadMinutes
	}
	if err := checkPolicy(in.CancelWindowMinutes, in.MinLeadMinutes); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// 5: ตรวจแต่ละรูป (ขนาด 413, ชนิดจากเนื้อไฟล์ 415)
	for _, fh := range files {
		img, err := httpx.ReadImage(fh)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		in.Images = append(in.Images, img)
	}

	id, err := h.Svc.Create(r.Context(), userID, in, h.Now())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	d, err := h.Svc.Detail(r.Context(), id, userID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, d)
}

// GET /api/restaurants/{id} (สาธารณะ)
func (h *Handler) detail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		httpx.Write(w, httpx.New("not-found", "ไม่พบร้านนี้"))
		return
	}
	d, err := h.Svc.Detail(r.Context(), id, viewer(r))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, d)
}

// GET /api/me/restaurants
func (h *Handler) mine(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	list, err := h.Svc.Mine(r.Context(), userID, h.Now())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

type infoBody struct {
	Name        *string  `json:"name"`
	Description *string  `json:"description"`
	CategoryIDs []int    `json:"category_ids"`
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
}

// PATCH /api/restaurants/{id} (03 §4.2) ส่งครบ name, description, category_ids
func (h *Handler) updateInfo(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		httpx.Write(w, httpx.New("not-found", "ไม่พบร้านนี้"))
		return
	}
	var body infoBody
	if !httpx.ReadJSON(w, r, &body) {
		return
	}
	errs, info := validateInfo(nil, body.Name, body.Description, body.CategoryIDs, body.Latitude, body.Longitude)
	if len(errs) > 0 {
		httpx.Write(w, httpx.Validation("ข้อมูลร้านไม่ถูกต้อง", errs...))
		return
	}
	if err := h.Svc.UpdateInfo(r.Context(), userID, id, info, h.Now()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	d, err := h.Svc.Detail(r.Context(), id, userID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, d)
}

// DELETE /api/restaurants/{id} (03 §4.5) → 204
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		httpx.Write(w, httpx.New("not-found", "ไม่พบร้านนี้"))
		return
	}
	if err := h.Svc.Delete(r.Context(), userID, id, h.Now()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/restaurants/{id}/images (03 §4.3) multipart ไฟล์เดียวชื่อ image, body ≤ 3 MB
func (h *Handler) addImage(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		httpx.Write(w, httpx.New("not-found", "ไม่พบร้านนี้"))
		return
	}
	if err := httpx.ParseMultipart(w, r, maxImageBody); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer r.MultipartForm.RemoveAll()
	files := r.MultipartForm.File["image"]
	if len(files) != 1 {
		httpx.Write(w, httpx.Validation("ต้องส่งรูป 1 ไฟล์", httpx.FieldError{Field: "image", Message: "ต้องมี 1 ไฟล์"}))
		return
	}
	img, err := httpx.ReadImage(files[0])
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	imageID, err := h.Svc.AddImage(r.Context(), userID, id, img, h.Now())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]int64{"id": imageID})
}

// DELETE /api/restaurants/{id}/images/{imageId} → 204
func (h *Handler) deleteImage(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, ok1 := pathID(r, "id")
	imageID, ok2 := pathID(r, "imageId")
	if !ok1 || !ok2 {
		httpx.Write(w, httpx.New("not-found", "ไม่พบรูปนี้"))
		return
	}
	if err := h.Svc.DeleteImage(r.Context(), userID, id, imageID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/restaurants/{id}/images/{imageId} (สาธารณะ)
// รูปแต่ละ id ไม่เปลี่ยนเนื้อหา จึง cache ได้ตลอด · nosniff กันเบราว์เซอร์เดาชนิดไฟล์เอง
func (h *Handler) image(w http.ResponseWriter, r *http.Request) {
	id, ok1 := pathID(r, "id")
	imageID, ok2 := pathID(r, "imageId")
	if !ok1 || !ok2 {
		httpx.Write(w, httpx.New("not-found", "ไม่พบรูปนี้"))
		return
	}
	img, err := h.Svc.Image(r.Context(), id, imageID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", img.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Write(img.Data)
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
