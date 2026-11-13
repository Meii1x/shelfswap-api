package handler

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"shelfswap/store"
)

// AdminStats GET /api/admin/stats
func (h *Handler) AdminStats(w http.ResponseWriter, r *http.Request) {
	st, err := h.Store.Stats(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

var validStatuses = map[string]bool{"": true, "pending": true, "available": true, "reserved": true, "sold": true, "rejected": true}

// AdminListings GET /api/admin/listings?status=pending
func (h *Handler) AdminListings(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if !validStatuses[status] {
		writeError(w, http.StatusBadRequest, "BAD_STATUS", "สถานะไม่ถูกต้อง")
		return
	}
	items, err := h.Store.AdminListings(r.Context(), status)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type reviewReq struct {
	Action string `json:"action"` // approve | reject
	Reason string `json:"reason"` // จำเป็นเมื่อ reject ผู้ขายจะเห็นเหตุผลนี้
}

// AdminReviewListing PATCH /api/admin/listings/{id}
func (h *Handler) AdminReviewListing(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req reviewReq
	if !decode(w, r, &req) {
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	switch {
	case req.Action != "approve" && req.Action != "reject":
		writeError(w, http.StatusBadRequest, "BAD_ACTION", "action ต้องเป็น approve หรือ reject")
		return
	case req.Action == "reject" && req.Reason == "":
		writeError(w, http.StatusBadRequest, "REASON_REQUIRED", "ไม่อนุมัติต้องใส่เหตุผลให้ผู้ขาย")
		return
	case utf8.RuneCountInString(req.Reason) > 200:
		writeError(w, http.StatusBadRequest, "BAD_REASON", "เหตุผลยาวได้ไม่เกิน 200 ตัวอักษร")
		return
	}
	l, err := h.Store.ReviewListing(r.Context(), id, req.Action == "approve", req.Reason)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "LISTING_NOT_FOUND", "ไม่พบประกาศนี้")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "WRONG_STATE", "ประกาศนี้ไม่ได้อยู่ในสถานะรออนุมัติแล้ว")
	case err != nil:
		internal(w, err)
	default:
		writeJSON(w, http.StatusOK, l)
	}
}
