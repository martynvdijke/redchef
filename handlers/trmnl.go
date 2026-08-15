package handlers

import (
	"encoding/json"
	"net/http"

	"redchef/db"
)

// trmnlComment is the comment shape exposed to the TRMNL plugin.
type trmnlComment struct {
	Username  string `json:"username"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// trmnlLatestPost is the enriched latest post shape exposed to the TRMNL plugin.
// MediaURL is omitted (null) for locked posts, mirroring handlers/public.go.
type trmnlLatestPost struct {
	ID             int64   `json:"id"`
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	MediaType      string  `json:"media_type"`
	Thumbnail      string  `json:"thumbnail"`
	MediaURL       *string `json:"media_url,omitempty"`
	Locked         bool    `json:"locked"`
	CreatedAt      string  `json:"created_at"`
	FavouriteCount int     `json:"favourite_count"`
	TipCount       int     `json:"tip_count"`
	CommentCount   int     `json:"comment_count"`
}

// maxTRMNLComments caps how many comments the plugin payload carries.
const maxTRMNLComments = 4

// TRMNLLatestPost returns the newest post with its engagement counts and the
// newest comments, shaped for the TRMNL polling plugin. The endpoint is
// public (auth-aware, not auth-required) — post metadata, counts and comment
// text are already exposed via the existing public API.
func TRMNLLatestPost(w http.ResponseWriter, r *http.Request) {
	posts, err := db.GetPosts(&db.PostFilter{})
	if err != nil {
		jsonError(w, "failed to list posts", http.StatusInternalServerError)
		return
	}

	response := struct {
		LatestPost *trmnlLatestPost `json:"latest_post"`
		Comments   []trmnlComment   `json:"comments"`
	}{
		Comments: []trmnlComment{},
	}

	if len(posts) > 0 {
		post := posts[0]

		commentCount := 0
		comments, err := db.GetCommentsByPost(post.ID)
		if err == nil {
			commentCount = len(comments)
		}

		favCount, err := db.GetFavouriteCount(post.ID)
		if err != nil {
			favCount = 0
		}
		tipCount, err := db.GetTipCount(post.ID)
		if err != nil {
			tipCount = 0
		}

		latest := &trmnlLatestPost{
			ID:             post.ID,
			Title:          post.Title,
			Description:    post.Description,
			MediaType:      post.MediaType,
			Thumbnail:      post.Thumbnail,
			Locked:         post.Locked,
			CreatedAt:      post.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			FavouriteCount: favCount,
			TipCount:       tipCount,
			CommentCount:   commentCount,
		}
		if !post.Locked {
			url := "/uploads/" + post.Filename
			latest.MediaURL = &url
		}
		response.LatestPost = latest

		// GetCommentsByPost returns oldest-first; take the newest four and
		// reverse so the plugin shows the most recent comments first.
		if len(comments) > 0 {
			start := 0
			if len(comments) > maxTRMNLComments {
				start = len(comments) - maxTRMNLComments
			}
			response.Comments = make([]trmnlComment, 0, len(comments)-start)
			for i := len(comments) - 1; i >= start; i-- {
				response.Comments = append(response.Comments, trmnlComment{
					Username:  comments[i].Username,
					Body:      comments[i].Body,
					CreatedAt: comments[i].CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
