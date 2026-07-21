package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"redchef/db"
)

func TestToggleFavourite_Add(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	_, err := db.CreatePost("Test Post", "", "photo", "test.jpg", "test.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	req := authenticatedRequest(t, "POST", "/api/posts/1/favourite", nil)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	ToggleFavourite(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["favourited"] != true {
		t.Error("expected favourited=true")
	}
	if result["favourite_count"] == nil {
		t.Error("expected favourite_count")
	}
}

func TestToggleFavourite_Remove(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	_, err := db.CreatePost("Test Post", "", "photo", "test.jpg", "test.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	// Favourite first
	db.ToggleFavourite(1, 1)

	// Toggle again to remove
	req := authenticatedRequest(t, "POST", "/api/posts/1/favourite", nil)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	ToggleFavourite(resp, req)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["favourited"] != false {
		t.Error("expected favourited=false after toggle off")
	}
}

func TestToggleFavourite_Unauthenticated(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/api/posts/1/favourite", nil)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	ToggleFavourite(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestToggleFavourite_PostNotFound(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := authenticatedRequest(t, "POST", "/api/posts/999/favourite", nil)
	req.SetPathValue("id", "999")
	resp := httptest.NewRecorder()
	ToggleFavourite(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}
}

func TestToggleFavourite_InvalidID(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := authenticatedRequest(t, "POST", "/api/posts/abc/favourite", nil)
	req.SetPathValue("id", "abc")
	resp := httptest.NewRecorder()
	ToggleFavourite(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestListFavourites_Empty(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := authenticatedRequest(t, "GET", "/api/favourites", nil)
	resp := httptest.NewRecorder()
	ListFavourites(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var posts []interface{}
	json.NewDecoder(resp.Body).Decode(&posts)
	if len(posts) != 0 {
		t.Fatalf("expected 0 favourites, got %d", len(posts))
	}
}

func TestListFavourites_WithItems(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	_, err := db.CreatePost("Fave Post", "", "photo", "fave.jpg", "fave.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	// Favourite it
	_, err = db.ToggleFavourite(1, 1)
	if err != nil {
		t.Fatalf("ToggleFavourite: %v", err)
	}

	req := authenticatedRequest(t, "GET", "/api/favourites", nil)
	resp := httptest.NewRecorder()
	ListFavourites(resp, req)

	var posts []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&posts)
	if len(posts) != 1 {
		t.Fatalf("expected 1 favourite, got %d", len(posts))
	}
	if posts[0]["title"] != "Fave Post" {
		t.Errorf("expected title 'Fave Post', got %v", posts[0]["title"])
	}
}

func TestListFavourites_Unauthenticated(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/favourites", nil)
	resp := httptest.NewRecorder()
	ListFavourites(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}
