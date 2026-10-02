package review

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
	mux.HandleFunc("GET /api/restaurants", h.listRestaurants)
	mux.HandleFunc("GET /api/restaurants/{id}/reviews", h.list)
	mux.HandleFunc("POST /api/restaurants/{id}/reviews", h.create)
	mux.HandleFunc("PUT /api/reviews/{id}", h.update)
	mux.HandleFunc("DELETE /api/reviews/{id}", h.delete)
}

func requireUser(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, ok := auth.UserID(r.Context())
	if !ok {
		httpx.Write(w, httpx.New("unauthenticated", "ต้องเข้าสู่ระบบก่อน"))
	}
	return id, ok
}

// viewer คือผู้ใช้ปัจจุบันสำหรับ endpoint สาธารณะ (0 = ไม่ได้ login)
func viewer(r *http.Request) int64 {
	id, _ := auth.UserID(r.Context())
	return id
}

func pathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id, err == nil && id > 0
}

// GET /api/restaurants?sort=&page=&page_size=&q=&category= (01 §7.8, 03 §8) สาธารณะ
func (h *Handler) listRestaurants(w http.ResponseWriter, r *http.Request) {
	sort, err := ValidSort(r.URL.Query().Get("sort"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	page, err := httpx.ParsePage(r, 20, 50)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// [B] ?q=ครัว&category=thai&category=seafood
	filter, err := h.Svc.ValidFilter(r.Context(), r.URL.Query().Get("q"), r.URL.Query()["category"])
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	res, err := h.Svc.ListRestaurants(r.Context(), sort, filter, viewer(r), page)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// GET /api/restaurants/{id}/reviews?page=&page_size= (01 §7.7) สาธารณะ
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	restaurantID, ok := pathID(r, "id")
	if !ok {
		httpx.Write(w, httpx.New("not-found", "ไม่พบร้านนี้"))
		return
	}
	page, err := httpx.ParsePage(r, 10, 50)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	res, err := h.Svc.List(r.Context(), restaurantID, viewer(r), page)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// POST /api/restaurants/{id}/reviews
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
	var in Input
	if !httpx.ReadJSON(w, r, &in) {
		return
	}
	rv, err := h.Svc.Create(r.Context(), userID, restaurantID, in, h.Now())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, rv)
}

// PUT /api/reviews/{id}
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	reviewID, ok := pathID(r, "id")
	if !ok {
		httpx.Write(w, httpx.New("not-found", "ไม่พบรีวิวนี้"))
		return
	}
	var in Input
	if !httpx.ReadJSON(w, r, &in) {
		return
	}
	rv, err := h.Svc.Update(r.Context(), userID, reviewID, in, h.Now())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, rv)
}

// DELETE /api/reviews/{id} → 204
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}
	reviewID, ok := pathID(r, "id")
	if !ok {
		httpx.Write(w, httpx.New("not-found", "ไม่พบรีวิวนี้"))
		return
	}
	if err := h.Svc.Delete(r.Context(), userID, reviewID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
