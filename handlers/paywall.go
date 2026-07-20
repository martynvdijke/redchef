package handlers

import (
	"encoding/json"
	"net/http"
	"redchef/db"
)

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
			"message": "Already unlocked! Enjoy.",
			"paid":    true,
		})
		return
	}

	if err := db.UpdateUserPaid(userID, true); err != nil {
		jsonError(w, "failed to process payment", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "🎉 Unlocked! Enjoy all content.",
		"paid":    true,
	})
}
