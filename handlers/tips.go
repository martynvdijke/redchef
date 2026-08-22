package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"redchef/db"
)

func CreateTip(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	postID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid post id", http.StatusBadRequest)
		return
	}

	userID := getUserID(r)
	if userID == 0 {
		jsonError(w, "authentication required", http.StatusUnauthorized)
		return
	}

	var req struct {
		AmountCents int `json:"amount_cents"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.AmountCents < 1 {
		jsonError(w, "amount_cents must be at least 1", http.StatusBadRequest)
		return
	}

	// Verify post exists
	post, err := db.GetPost(postID)
	if err != nil {
		jsonError(w, "post not found", http.StatusNotFound)
		return
	}

	if err := db.CreateTip(userID, postID, req.AmountCents); err != nil {
		jsonError(w, "failed to create tip", http.StatusInternalServerError)
		return
	}

	// Send confirmation email + Gotify notification
	user, err := db.GetUserByID(userID)
	if err == nil {
		go SendTipNotification(user.Email, post.Title, req.AmountCents)
	} else {
		log.Printf("[tips] Failed to get user %d for notification: %v", userID, err)
	}

	count, _ := db.GetTipCount(postID)
	totalAmount, _ := db.GetTotalTipAmount(postID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":           true,
		"tip_count":    count,
		"amount_cents": req.AmountCents,
		"total_amount": totalAmount,
		"formatted":    fmt.Sprintf("€%d,%02d", req.AmountCents/100, req.AmountCents%100),
	})
}
