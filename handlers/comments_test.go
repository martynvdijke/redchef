package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"redchef/db"
)

func TestListComments_Empty(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	// Create a post first
	_, err := db.CreatePost("Test", "", "photo", "test.jpg", "test.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/posts/1/comments", nil)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	ListComments(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var comments []interface{}
	json.NewDecoder(resp.Body).Decode(&comments)
	if len(comments) != 0 {
		t.Fatalf("expected 0 comments, got %d", len(comments))
	}
}

func TestListComments_WithComments(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	_, err := db.CreatePost("Test", "", "photo", "test.jpg", "test.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	// Create comment as admin
	_, err = db.CreateComment(1, 1, nil, "Nice post!")
	if err != nil {
		t.Fatalf("CreateComment: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/posts/1/comments", nil)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	ListComments(resp, req)

	var comments []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&comments)
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if comments[0]["body"] != "Nice post!" {
		t.Errorf("expected body 'Nice post!', got %v", comments[0]["body"])
	}
}

func TestListComments_InvalidPostID(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/posts/abc/comments", nil)
	req.SetPathValue("id", "abc")
	resp := httptest.NewRecorder()
	ListComments(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestCreateComment_Success(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	_, err := db.CreatePost("Test", "", "photo", "test.jpg", "test.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	body := bytes.NewReader([]byte(`{"body":"Great content!"}`))
	req := authenticatedRequest(t, "POST", "/api/posts/1/comments", body)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	CreateComment(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["body"] != "Great content!" {
		t.Errorf("expected body 'Great content!', got %v", result["body"])
	}
}

func TestCreateComment_Unauthenticated(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"body":"test"}`))
	req := httptest.NewRequest("POST", "/api/posts/1/comments", body)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	CreateComment(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestCreateComment_EmptyBody(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"body":""}`))
	req := authenticatedRequest(t, "POST", "/api/posts/1/comments", body)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	CreateComment(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestCreateComment_PostNotFound(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"body":"test"}`))
	req := authenticatedRequest(t, "POST", "/api/posts/999/comments", body)
	req.SetPathValue("id", "999")
	resp := httptest.NewRecorder()
	CreateComment(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}
}

func TestCreateComment_InvalidPostID(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"body":"test"}`))
	req := authenticatedRequest(t, "POST", "/api/posts/abc/comments", body)
	req.SetPathValue("id", "abc")
	resp := httptest.NewRecorder()
	CreateComment(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestAdminDeleteComment_Success(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	_, err := db.CreatePost("Test", "", "photo", "test.jpg", "test.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	comment, err := db.CreateComment(1, 1, nil, "Delete me")
	if err != nil {
		t.Fatalf("CreateComment: %v", err)
	}

	req := authenticatedRequest(t, "DELETE", "/api/admin/comments/1", nil)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	AdminDeleteComment(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// Verify deleted
	_, err = db.GetComment(comment.ID)
	if err == nil {
		t.Error("expected comment to be deleted")
	}
}

func TestAdminDeleteComment_InvalidID(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := authenticatedRequest(t, "DELETE", "/api/admin/comments/abc", nil)
	req.SetPathValue("id", "abc")
	resp := httptest.NewRecorder()
	AdminDeleteComment(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestAdminDeleteComment_Nonexistent(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := authenticatedRequest(t, "DELETE", "/api/admin/comments/999", nil)
	req.SetPathValue("id", "999")
	resp := httptest.NewRecorder()
	AdminDeleteComment(resp, req)

	// Should still return 200 (idempotent delete)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}
