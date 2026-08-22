package handlers

import (
	"encoding/json"
	"net/http"

	"redchef/db"
)

// ListTags serves GET /api/tags: every tag in use with its post count.
func ListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := db.ListTagsWithCounts()
	if err != nil {
		jsonError(w, "failed to list tags", http.StatusInternalServerError)
		return
	}
	if tags == nil {
		tags = []db.TagWithCount{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tags)
}
