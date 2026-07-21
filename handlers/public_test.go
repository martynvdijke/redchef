package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"redchef/db"
)

func TestListPosts_Empty(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/posts", nil)
	resp := httptest.NewRecorder()
	ListPosts(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var posts []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(posts) != 0 {
		t.Fatalf("expected 0 posts, got %d", len(posts))
	}
}

func TestListPosts_WithPosts(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	_, err := db.CreatePost("First Post", "Description", "photo", "first.jpg", "first.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	_, err = db.CreatePost("Second Post", "Desc 2", "video", "second.mp4", "second.jpg", true)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/posts", nil)
	resp := httptest.NewRecorder()
	ListPosts(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var posts []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&posts)
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(posts))
	}

	// Unauthenticated: locked post should not have media_url
	for _, p := range posts {
		locked := p["locked"].(bool)
		mediaURL := p["media_url"]
		if locked && mediaURL != nil {
			t.Errorf("locked post should not have media_url for anonymous user")
		}
	}
}

func TestListPosts_FilterByType(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	if _, err := db.CreatePost("Photo Post", "", "photo", "p.jpg", "p.jpg", false); err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	if _, err := db.CreatePost("Video Post", "", "video", "v.mp4", "v.jpg", true); err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	// Verify posts exist in DB directly
	allPosts, err := db.GetPosts(nil)
	if err != nil {
		t.Fatalf("GetPosts (no filter): %v", err)
	}
	if len(allPosts) != 2 {
		t.Fatalf("expected 2 posts in DB, got %d", len(allPosts))
	}

	filteredPosts, err := db.GetPosts(&db.PostFilter{Type: "photo"})
	if err != nil {
		t.Fatalf("GetPosts (filter=photo): %v", err)
	}
	if len(filteredPosts) != 1 {
		t.Fatalf("expected 1 photo post in DB, got %d", len(filteredPosts))
	}

	req := httptest.NewRequest("GET", "/api/posts?type=photo", nil)
	resp := httptest.NewRecorder()
	ListPosts(resp, req)

	var posts []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&posts)
	if len(posts) != 1 {
		t.Fatalf("expected 1 photo post, got %d", len(posts))
	}
	if posts[0]["media_type"] != "photo" {
		t.Errorf("expected media_type 'photo', got %v", posts[0]["media_type"])
	}
}

func TestListPosts_InvalidType(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/posts?type=gif", nil)
	resp := httptest.NewRecorder()
	ListPosts(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestListPosts_SortOldest(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	if _, err := db.CreatePost("First", "", "photo", "f.jpg", "f.jpg", false); err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	if _, err := db.CreatePost("Second", "", "photo", "s.jpg", "s.jpg", false); err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/posts?sort=oldest", nil)
	resp := httptest.NewRecorder()
	ListPosts(resp, req)

	var posts []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&posts)
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(posts))
	}
	if posts[0]["title"] != "First" {
		t.Errorf("expected first post to be 'First', got %v", posts[0]["title"])
	}
}

func TestGetPost_Found(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	_, err := db.CreatePost("Test Post", "Desc", "photo", "test.jpg", "test.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/posts/1", nil)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	GetPost(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["title"] != "Test Post" {
		t.Errorf("expected title 'Test Post', got %v", result["title"])
	}
	if result["media_url"] == nil {
		t.Error("expected media_url for unlocked post")
	}
}

func TestGetPost_NotFound(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/posts/999", nil)
	req.SetPathValue("id", "999")
	resp := httptest.NewRecorder()
	GetPost(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}
}

func TestGetPost_InvalidID(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/posts/abc", nil)
	req.SetPathValue("id", "abc")
	resp := httptest.NewRecorder()
	GetPost(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestGetPost_LockedUnauthenticated(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	_, err := db.CreatePost("Locked", "", "photo", "locked.jpg", "locked.jpg", true)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/posts/1", nil)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	GetPost(resp, req)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["media_url"] != nil {
		t.Error("locked post should not have media_url for anonymous user")
	}
	if result["unlocked"] != false {
		t.Error("expected unlocked=false for anonymous user on locked post")
	}
}

func TestGetPost_LockedAuthenticatedUser(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	// Create a second user (non-admin)
	userID, err := db.CreateUser("user@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	_, err = db.CreatePost("Locked", "", "photo", "locked.jpg", "locked.jpg", true)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	// Create session for non-paid user
	session, err := db.CreateUserSession(userID)
	if err != nil {
		t.Fatalf("CreateUserSession: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/posts/1", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: session.Token})
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	GetPost(resp, req)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["media_url"] != nil {
		t.Error("locked post should not have media_url for non-paid user")
	}
}

func TestPublicGetAnalyticsSettings_Empty(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/settings/analytics", nil)
	resp := httptest.NewRecorder()
	PublicGetAnalyticsSettings(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var result PublicAnalyticsResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.TrackingEnabled {
		t.Error("expected tracking_enabled=false by default")
	}
}

func TestPublicGetAnalyticsSettings_Configured(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	db.UpdateAnalyticsSettings("https://umami.example.com/script.js", "test-id", true)

	req := httptest.NewRequest("GET", "/api/settings/analytics", nil)
	resp := httptest.NewRecorder()
	PublicGetAnalyticsSettings(resp, req)

	var result PublicAnalyticsResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.UmamiScriptURL != "https://umami.example.com/script.js" {
		t.Errorf("expected script URL, got %q", result.UmamiScriptURL)
	}
	if !result.TrackingEnabled {
		t.Error("expected tracking_enabled=true")
	}
}
