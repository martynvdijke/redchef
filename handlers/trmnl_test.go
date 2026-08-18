package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"redchef/db"
)

func TestTRMNLLatestPost_Empty(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/trmnl/latest", nil)
	resp := httptest.NewRecorder()
	TRMNLLatestPost(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["latest_post"] != nil {
		t.Errorf("expected latest_post null, got %v", result["latest_post"])
	}
	if comments, ok := result["comments"].([]interface{}); !ok || len(comments) != 0 {
		t.Errorf("expected empty comments array, got %v", result["comments"])
	}
}

func TestTRMNLLatestPost_ReturnsNewestPostWithCounts(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	post, err := db.CreatePost("Older Post", "Old desc", "photo", "old.jpg", "old.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	post, err = db.CreatePost("Newest Post", "New desc", "photo", "new.jpg", "new.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	if _, err := db.ToggleFavourite(1, post.ID); err != nil {
		t.Fatalf("ToggleFavourite: %v", err)
	}
	if err := db.CreateTip(1, post.ID, 500); err != nil {
		t.Fatalf("CreateTip: %v", err)
	}
	if _, err := db.CreateComment(post.ID, 1, nil, "first"); err != nil {
		t.Fatalf("CreateComment: %v", err)
	}
	if _, err := db.CreateComment(post.ID, 1, nil, "second"); err != nil {
		t.Fatalf("CreateComment: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/trmnl/latest", nil)
	resp := httptest.NewRecorder()
	TRMNLLatestPost(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	latest := result["latest_post"].(map[string]interface{})
	if latest["title"] != "Newest Post" {
		t.Errorf("expected newest post, got %v", latest["title"])
	}
	if latest["favourite_count"].(float64) != 1 {
		t.Errorf("expected favourite_count 1, got %v", latest["favourite_count"])
	}
	if latest["tip_count"].(float64) != 1 {
		t.Errorf("expected tip_count 1, got %v", latest["tip_count"])
	}
	if latest["comment_count"].(float64) != 2 {
		t.Errorf("expected comment_count 2, got %v", latest["comment_count"])
	}
	if latest["media_url"] == nil {
		t.Error("expected media_url for unlocked post")
	}

	comments := result["comments"].([]interface{})
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}
	// Newest first: "second" was created after "first"
	if comments[0].(map[string]interface{})["body"] != "second" {
		t.Errorf("expected newest comment first, got %v", comments[0])
	}
	if comments[0].(map[string]interface{})["username"] == nil {
		t.Error("expected username on comment")
	}
}

func TestTRMNLLatestPost_CommentsCappedAtFour(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	post, err := db.CreatePost("Capped", "", "photo", "cap.jpg", "cap.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	// created_at is second precision; sleep so ordering is deterministic.
	for i := 1; i <= 5; i++ {
		if _, err := db.CreateComment(post.ID, 1, nil, "comment"); err != nil {
			t.Fatalf("CreateComment: %v", err)
		}
		time.Sleep(1100 * time.Millisecond)
	}

	req := httptest.NewRequest("GET", "/api/trmnl/latest", nil)
	resp := httptest.NewRecorder()
	TRMNLLatestPost(resp, req)

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	latest := result["latest_post"].(map[string]interface{})
	if latest["comment_count"].(float64) != 5 {
		t.Errorf("expected full comment_count 5, got %v", latest["comment_count"])
	}
	comments := result["comments"].([]interface{})
	if len(comments) != 4 {
		t.Fatalf("expected 4 comments in payload, got %d", len(comments))
	}
}

func TestTRMNLLatestPost_LockedIncludesMediaURL(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	if _, err := db.CreatePost("Locked", "", "video", "lock.mp4", "lock.jpg", true); err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/trmnl/latest", nil)
	resp := httptest.NewRecorder()
	TRMNLLatestPost(resp, req)

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	latest := result["latest_post"].(map[string]interface{})
	if latest["locked"] != true {
		t.Errorf("expected locked=true, got %v", latest["locked"])
	}
	// Posts are jokes, so the TRMNL plugin always gets the media.
	if latest["media_url"] == nil {
		t.Error("locked post should still expose media_url")
	}
}
