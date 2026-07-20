package handlers

import (
	"encoding/json"
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

	// Verify post exists
	if _, err := db.GetPost(postID); err != nil {
		jsonError(w, "post not found", http.StatusNotFound)
		return
	}

	// Prevent double-tipping
	tipped, err := db.HasUserTipped(userID, postID)
	if err != nil {
		jsonError(w, "failed to check tip status", http.StatusInternalServerError)
		return
	}
	if tipped {
		jsonError(w, "already tipped this post", http.StatusConflict)
		return
	}

	if err := db.CreateTip(userID, postID); err != nil {
		jsonError(w, "failed to create tip", http.StatusInternalServerError)
		return
	}

	count, _ := db.GetTipCount(postID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":        true,
		"tip_count": count,
	})
}
