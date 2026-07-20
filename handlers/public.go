package handlers

import (
	"encoding/json"
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
	filter := &db.PostFilter{
		Sort:     r.URL.Query().Get("sort"),
		Type:     r.URL.Query().Get("type"),
		DateFrom: r.URL.Query().Get("date_from"),
		DateTo:   r.URL.Query().Get("date_to"),
	}

	// Validate type param
	if filter.Type != "" && filter.Type != "photo" && filter.Type != "video" {
		jsonError(w, "invalid type filter: must be 'photo' or 'video'", http.StatusBadRequest)
		return
	}

	posts, err := db.GetPosts(filter)
	if err != nil {
		jsonError(w, "failed to list posts", http.StatusInternalServerError)
		return
	}
	if posts == nil {
		posts = []db.Post{}
	}

	// Determine user's access
	userID := getUserID(r)
	isPaid := false
	if userID > 0 {
		user, err := db.GetUserByID(userID)
		if err == nil {
			isPaid = user.Paid || user.Role == "admin"
		}
	}

	// Enrich response with access info
	type postResponse struct {
		db.Post
		MediaURL *string `json:"media_url"`
		Unlocked bool    `json:"unlocked"`
	}

	responses := make([]postResponse, len(posts))
	for i, p := range posts {
		unlocked := !p.Locked || isPaid
		var mediaURL *string
		if unlocked {
			url := "/uploads/" + p.Filename
			mediaURL = &url
		}
		responses[i] = postResponse{
			Post:     p,
			MediaURL: mediaURL,
			Unlocked: unlocked,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

func GetPost(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	post, err := db.GetPost(id)
	if err != nil {
		jsonError(w, "post not found", http.StatusNotFound)
		return
	}

	// Check access
	userID := getUserID(r)
	isPaid := false
	if userID > 0 {
		user, err := db.GetUserByID(userID)
		if err == nil {
			isPaid = user.Paid || user.Role == "admin"
		}
	}

	unlocked := !post.Locked || isPaid
	type postDetail struct {
		db.Post
		MediaURL *string `json:"media_url"`
		Unlocked bool    `json:"unlocked"`
	}
	response := postDetail{Post: *post, Unlocked: unlocked}
	if unlocked {
		url := "/uploads/" + post.Filename
		response.MediaURL = &url
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func PublicGetAnalyticsSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := db.GetAnalyticsSettings()
	if err != nil {
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
