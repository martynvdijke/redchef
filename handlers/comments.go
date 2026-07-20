package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"redchef/db"
)

func ListComments(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	postID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid post id", http.StatusBadRequest)
		return
	}

	comments, err := db.GetCommentsByPost(postID)
	if err != nil {
		jsonError(w, "failed to load comments", http.StatusInternalServerError)
		return
	}
	if comments == nil {
		comments = []db.Comment{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comments)
}

func CreateComment(w http.ResponseWriter, r *http.Request) {
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
		Body     string `json:"body"`
		ParentID *int64 `json:"parent_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Body == "" {
		jsonError(w, "body is required", http.StatusBadRequest)
		return
	}

	// Verify post exists
	if _, err := db.GetPost(postID); err != nil {
		jsonError(w, "post not found", http.StatusNotFound)
		return
	}

	// If parent_id provided, verify it exists
	if req.ParentID != nil {
		if _, err := db.GetComment(*req.ParentID); err != nil {
			jsonError(w, "parent comment not found", http.StatusNotFound)
			return
		}
	}

	comment, err := db.CreateComment(postID, userID, req.ParentID, req.Body)
	if err != nil {
		jsonError(w, "failed to create comment", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(comment)
}

func AdminDeleteComment(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := db.DeleteComment(id); err != nil {
		jsonError(w, "failed to delete comment", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"ok":true}`))
}
