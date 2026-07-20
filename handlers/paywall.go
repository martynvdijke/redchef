package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"redchef/db"
)

// Item prices in cents — mock iDEAL, nothing is ever charged.
const (
	PricePhotoCents = 5  // €0,05
	PriceVideoCents = 20 // €0,20
)

func itemPriceCents(mediaType string) int {
	if mediaType == "video" {
		return PriceVideoCents
	}
	return PricePhotoCents
}

func formatEuros(cents int) string {
	return fmt.Sprintf("%d,%02d", cents/100, cents%100)
}

// PayUnlock handles the mock subscription purchase (€4,99/maand).
// The fake iDEAL payment always succeeds.
func PayUnlock(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == 0 {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := db.GetUserByID(userID)
	if err != nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}

	if user.Paid {
		// Already paid — idempotent
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      true,
			"message": "Je bent al geabonneerd! Geniet ervan.",
			"paid":    true,
		})
		return
	}

	// Mock iDEAL payment — always succeeds, nothing is actually charged.
	if err := db.UpdateUserPaid(userID, true); err != nil {
		jsonError(w, "failed to process payment", http.StatusInternalServerError)
		return
	}

	// Email the subscription invoice (async, best-effort)
	go SendSubscriptionInvoice(user.Email)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Je bent geabonneerd! Alle content is nu ontgrendeld.",
		"paid":    true,
	})
}

// PayItem handles a mock per-item purchase (€0,05 photo / €0,20 video).
// The fake iDEAL payment always succeeds and unlocks that single post.
func PayItem(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == 0 {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		PostID int64  `json:"post_id"`
		Bank   string `json:"bank"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PostID == 0 {
		jsonError(w, "post_id is required", http.StatusBadRequest)
		return
	}

	post, err := db.GetPost(req.PostID)
	if err != nil {
		jsonError(w, "post not found", http.StatusNotFound)
		return
	}

	user, err := db.GetUserByID(userID)
	if err != nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}

	// Already has access (subscriber, admin, free post, or previous purchase) — idempotent
	if user.Paid || user.Role == "admin" || !post.Locked {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      true,
			"message": "Je hebt al toegang tot deze content.",
		})
		return
	}
	if purchased, _ := db.HasUserPurchased(userID, post.ID); purchased {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      true,
			"message": "Deze content heb je al ontgrendeld.",
		})
		return
	}

	priceCents := itemPriceCents(post.MediaType)
	if err := db.CreatePurchase(userID, post.ID, priceCents); err != nil {
		jsonError(w, "failed to process payment", http.StatusInternalServerError)
		return
	}

	// Email the item invoice (async, best-effort)
	go SendItemInvoice(user.Email, post.Title, formatEuros(priceCents))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Gepind! Content ontgrendeld.",
		"charged": formatEuros(priceCents),
	})
}
