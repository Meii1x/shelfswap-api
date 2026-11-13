package handler

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"shelfswap/store"
)

func reservationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "ไม่พบรายการนี้")
	case errors.Is(err, store.ErrOwnListing):
		writeError(w, http.StatusBadRequest, "OWN_LISTING", "จองสินค้าของตัวเองไม่ได้")
	case errors.Is(err, store.ErrNotAvailable):
		writeError(w, http.StatusConflict, "NOT_AVAILABLE", "สินค้านี้มีคนจองไปแล้วหรือขายแล้ว")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "WRONG_STATE", "การจองนี้ถูกตอบไปแล้ว")
	default:
		internal(w, err)
	}
}

type reserveReq struct {
	Message string `json:"message"`
}

// Reserve POST /api/listings/{id}/reserve
func (h *Handler) Reserve(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req reserveReq
	if !decode(w, r, &req) {
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if utf8.RuneCountInString(req.Message) > 300 {
		writeError(w, http.StatusBadRequest, "BAD_MESSAGE", "ข้อความถึงผู้ขายยาวได้ไม่เกิน 300 ตัวอักษร")
		return
	}
	res, err := h.Store.Reserve(r.Context(), id, me(r).ID, req.Message)
	if err != nil {
		reservationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

type decideReq struct {
	Action string `json:"action"` // accept | reject (ผู้ขาย), cancel (ผู้ซื้อ)
}

// DecideReservation PATCH /api/reservations/{id}
func (h *Handler) DecideReservation(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req decideReq
	if !decode(w, r, &req) {
		return
	}
	if req.Action != "accept" && req.Action != "reject" && req.Action != "cancel" {
		writeError(w, http.StatusBadRequest, "BAD_ACTION", "action ต้องเป็น accept, reject หรือ cancel")
		return
	}
	res, err := h.Store.Decide(r.Context(), id, me(r).ID, req.Action)
	if err != nil {
		reservationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// MyReservations GET /api/me/reservations  (ฉันเป็นผู้ซื้อ)
func (h *Handler) MyReservations(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.ListByBuyer(r.Context(), me(r).ID)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// MyRequests GET /api/me/requests  (ฉันเป็นผู้ขาย)
func (h *Handler) MyRequests(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.ListBySeller(r.Context(), me(r).ID)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
