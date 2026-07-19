package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"redchef/db"
)

func ListPosts(w http.ResponseWriter, r *http.Request) {
	posts, err := db.GetPosts()
	if err != nil {
		http.Error(w, `{"error":"failed to list posts"}`, http.StatusInternalServerError)
		return
	}
	if posts == nil {
		posts = []db.Post{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(posts)
}

func GetPost(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	post, err := db.GetPost(id)
	if err != nil {
		http.Error(w, `{"error":"post not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(post)
}

type UnlockRequest struct {
	PostID int64 `json:"post_id"`
}

type UnlockResponse struct {
	Ok       bool   `json:"ok"`
	Message  string `json:"message"`
	Charged  string `json:"charged"`
}

func Unlock(w http.ResponseWriter, r *http.Request) {
	var req UnlockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	// Verify post exists
	_, err := db.GetPost(req.PostID)
	if err != nil {
		http.Error(w, `{"error":"post not found"}`, http.StatusNotFound)
		return
	}

	// Track unlocked posts in cookie
	var unlocked []string
	cookie, err := r.Cookie("unlocked_posts")
	if err == nil && cookie.Value != "" {
		unlocked = append(unlocked, cookie.Value)
	}
	unlocked = append(unlocked, strconv.FormatInt(req.PostID, 10))

	// Store as comma-separated list
	http.SetCookie(w, &http.Cookie{
		Name:     "unlocked_posts",
		Value:    joinUnlocked(unlocked),
		Path:     "/",
		MaxAge:   86400 * 365, // 1 year
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UnlockResponse{
		Ok:      true,
		Message: "Thank you for your payment! Your card has been charged $0.05.",
		Charged: "0.05",
	})
}

func joinUnlocked(ids []string) string {
	seen := make(map[string]bool)
	var result []string
	for _, id := range ids {
		// Split by comma in case the cookie already has multiple
		for _, part := range splitAndTrim(id) {
			if part != "" && !seen[part] {
				seen[part] = true
				result = append(result, part)
			}
		}
	}
	out := ""
	for i, s := range result {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}

func splitAndTrim(s string) []string {
	var result []string
	current := ""
	for _, c := range s {
		if c == ',' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
