package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"redchef/db"
)

type PublicAnalyticsResponse struct {
	UmamiScriptURL  string `json:"umami_script_url"`
	UmamiWebsiteID  string `json:"umami_website_id"`
	TrackingEnabled bool   `json:"tracking_enabled"`
}

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

type SubscribeRequest struct {
	PostID *int64 `json:"post_id,omitempty"`
}

type SubscribeResponse struct {
	Ok      bool   `json:"ok"`
	Message string `json:"message"`
	Charged string `json:"charged"`
}

func Subscribe(w http.ResponseWriter, r *http.Request) {
	var req SubscribeRequest
	json.NewDecoder(r.Body).Decode(&req)

	if req.PostID != nil {
		// Per-item purchase — set individual unlock cookie
		post, err := db.GetPost(*req.PostID)
		if err != nil {
			http.Error(w, `{"error":"post not found"}`, http.StatusNotFound)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:   fmt.Sprintf("unlocked_%d", *req.PostID),
			Value:  "1",
			Path:   "/",
			MaxAge: 86400 * 365,
		})

		price := "0.05"
		if post.MediaType == "video" {
			price = "0.20"
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SubscribeResponse{
			Ok:      true,
			Message: "🔓 Ontgrendeld! Geniet ervan.",
			Charged: price,
		})
		return
	}

	// Subscription — set royal member cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "royal_member",
		Value:  "1",
		Path:   "/",
		MaxAge: 86400 * 365, // 1 year
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SubscribeResponse{
		Ok:      true,
		Message: "👑 Welkom bij het Royal Inner Circle!",
		Charged: "4.99",
	})
}

func PublicGetAnalyticsSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := db.GetAnalyticsSettings()
	if err != nil {
		// Return safe defaults instead of erroring
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PublicAnalyticsResponse{})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PublicAnalyticsResponse{
		UmamiScriptURL:  settings.UmamiScriptURL,
		UmamiWebsiteID:  settings.UmamiWebsiteID,
		TrackingEnabled: settings.TrackingEnabled,
	})
}


