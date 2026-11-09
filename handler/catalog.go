package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"shelfswap/mw"
	"shelfswap/store"
)

// intParam อ่านตัวเลขจาก query ถ้าไม่ส่งมาใช้ค่า def ถ้าส่งมาแต่ไม่ใช่ตัวเลขคืน ok=false
func intParam(q url.Values, name string, def int) (int, bool) {
	s := q.Get(name)
	if s == "" {
		return def, true
	}
	n, err := strconv.Atoi(s)
	return n, err == nil && n >= 0
}

// SearchListings GET /api/listings?q=&category=&condition=&minPrice=&maxPrice=&sort=&page=&pageSize=
// เส้นทางในคลิปคนที่ 1: FilterSidebar -> query string -> SearchParams -> SQL WHERE -> Grid
func (h *Handler) SearchListings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	p := store.SearchParams{
		Q:         strings.TrimSpace(q.Get("q")),
		Condition: q.Get("condition"),
		Sort:      q.Get("sort"),
	}
	cat, ok1 := intParam(q, "category", 0)
	minP, ok2 := intParam(q, "minPrice", 0)
	maxP, ok3 := intParam(q, "maxPrice", 0)
	page, ok4 := intParam(q, "page", 1)
	size, ok5 := intParam(q, "pageSize", 12)
	if !(ok1 && ok2 && ok3 && ok4 && ok5) {
		writeError(w, http.StatusBadRequest, "BAD_QUERY", "ค่าตัวกรองที่เป็นตัวเลขต้องเป็นจำนวนเต็มไม่ติดลบ")
		return
	}
	if p.Condition != "" && !validConditions[p.Condition] {
		writeError(w, http.StatusBadRequest, "BAD_QUERY", "สภาพสินค้าไม่ถูกต้อง")
		return
	}
	if maxP > 0 && minP > maxP {
		writeError(w, http.StatusBadRequest, "BAD_QUERY", "ราคาต่ำสุดต้องไม่มากกว่าราคาสูงสุด")
		return
	}
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 48 {
		size = 12
	}
	p.CategoryID, p.MinPrice, p.MaxPrice, p.Page, p.PageSize = int64(cat), minP, maxP, page, size

	items, total, err := h.Store.SearchListings(r.Context(), p)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":      items,
		"total":      total,
		"page":       page,
		"pageSize":   size,
		"totalPages": (total + size - 1) / size,
	})
}

// GetListing ใครก็ดูได้ถ้าขึ้นหน้าร้านแล้ว (available/reserved/sold)
// แต่ pending/rejected เห็นได้เฉพาะเจ้าของกับ admin คนอื่นได้ 404 (route นี้ครอบด้วย MaybeLogin)
func (h *Handler) GetListing(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	l, err := h.Store.GetListing(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "LISTING_NOT_FOUND", "ไม่พบประกาศนี้")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	if l.Status == "pending" || l.Status == "rejected" {
		who, loggedIn := mw.WhoFrom(r.Context())
		if !loggedIn || (who.ID != l.SellerID && who.Role != "admin") {
			writeError(w, http.StatusNotFound, "LISTING_NOT_FOUND", "ไม่พบประกาศนี้")
			return
		}
	}
	writeJSON(w, http.StatusOK, l)
}

func (h *Handler) Categories(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.ListCategories(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
