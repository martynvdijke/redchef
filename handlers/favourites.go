package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"redchef/db"
)

func ToggleFavourite(w http.ResponseWriter, r *http.Request) {
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

	favourited, err := db.ToggleFavourite(userID, postID)
	if err != nil {
		jsonError(w, "failed to toggle favourite", http.StatusInternalServerError)
		return
	}

	count, _ := db.GetFavouriteCount(postID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"favourited":      favourited,
		"favourite_count": count,
	})
}

func ListFavourites(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == 0 {
		jsonError(w, "authentication required", http.StatusUnauthorized)
		return
	}

	posts, err := db.ListFavourites(userID)
	if err != nil {
		jsonError(w, "failed to load favourites", http.StatusInternalServerError)
		return
	}
	if posts == nil {
		posts = []db.Post{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(posts)
}
