package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"redchef/db"
)

func TestPayUnlock_Success(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	// Create a non-admin, non-paid user
	userID, err := db.CreateUser("user@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	session, err := db.CreateUserSession(userID)
	if err != nil {
		t.Fatalf("CreateUserSession: %v", err)
	}

	body := bytes.NewReader([]byte(`{"bank":"ING"}`))
	req := httptest.NewRequest("POST", "/api/pay/unlock", body)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: session.Token})
	req.Header.Set("X-User-ID", "2")
	resp := httptest.NewRecorder()
	PayUnlock(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["ok"] != true {
		t.Error("expected ok=true")
	}
	if result["paid"] != true {
		t.Error("expected paid=true")
	}

	// Verify user is now paid
	user, err := db.GetUserByID(2)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if !user.Paid {
		t.Error("expected user to be marked as paid")
	}
}

func TestPayUnlock_AlreadyPaid(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	// Admin is already paid
	session, err := db.CreateUserSession(1)
	if err != nil {
		t.Fatalf("CreateUserSession: %v", err)
	}

	body := bytes.NewReader([]byte(`{"bank":"ING"}`))
	req := httptest.NewRequest("POST", "/api/pay/unlock", body)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: session.Token})
	req.Header.Set("X-User-ID", "1")
	resp := httptest.NewRecorder()
	PayUnlock(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["paid"] != true {
		t.Error("expected paid=true for already paid user")
	}
	if result["message"] == "" {
		t.Error("expected message for already paid")
	}
}

func TestPayUnlock_Unauthenticated(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"bank":"ING"}`))
	req := httptest.NewRequest("POST", "/api/pay/unlock", body)
	resp := httptest.NewRecorder()
	PayUnlock(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestPayItem_Success(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	// Create non-paid user
	userID, err := db.CreateUser("user@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Create a locked post
	_, err = db.CreatePost("Locked Item", "", "video", "locked.mp4", "locked.jpg", true)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	session, err := db.CreateUserSession(userID)
	if err != nil {
		t.Fatalf("CreateUserSession: %v", err)
	}

	body := bytes.NewReader([]byte(`{"post_id":1,"bank":"ING"}`))
	req := httptest.NewRequest("POST", "/api/pay/item", body)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: session.Token})
	req.Header.Set("X-User-ID", "2")
	resp := httptest.NewRecorder()
	PayItem(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["ok"] != true {
		t.Error("expected ok=true")
	}

	// Verify purchase was recorded
	purchased, _ := db.HasUserPurchased(2, 1)
	if !purchased {
		t.Error("expected purchase to be recorded")
	}
}

func TestPayItem_AlreadyUnlocked(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	// Admin has automatic access
	session, err := db.CreateUserSession(1)
	if err != nil {
		t.Fatalf("CreateUserSession: %v", err)
	}

	_, err = db.CreatePost("Locked", "", "photo", "locked.jpg", "locked.jpg", true)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	body := bytes.NewReader([]byte(`{"post_id":1,"bank":"ING"}`))
	req := httptest.NewRequest("POST", "/api/pay/item", body)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: session.Token})
	req.Header.Set("X-User-ID", "1")
	resp := httptest.NewRecorder()
	PayItem(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["ok"] != true {
		t.Error("expected ok=true for admin")
	}
}

func TestPayItem_AlreadyPurchased(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	userID, err := db.CreateUser("user@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	_, err = db.CreatePost("Locked", "", "photo", "locked.jpg", "locked.jpg", true)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	// Purchase first
	db.CreatePurchase(userID, 1, 5)

	session, err := db.CreateUserSession(userID)
	if err != nil {
		t.Fatalf("CreateUserSession: %v", err)
	}

	body := bytes.NewReader([]byte(`{"post_id":1,"bank":"ING"}`))
	req := httptest.NewRequest("POST", "/api/pay/item", body)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: session.Token})
	req.Header.Set("X-User-ID", "2")
	resp := httptest.NewRecorder()
	PayItem(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["ok"] != true {
		t.Error("expected ok=true for already purchased")
	}
}

func TestPayItem_MissingPostID(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"invalid":true}`))
	req := authenticatedRequest(t, "POST", "/api/pay/item", body)
	resp := httptest.NewRecorder()
	PayItem(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestPayItem_PostNotFound(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"post_id":999,"bank":"ING"}`))
	req := authenticatedRequest(t, "POST", "/api/pay/item", body)
	resp := httptest.NewRecorder()
	PayItem(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}
}

func TestPayItem_Unauthenticated(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"post_id":1,"bank":"ING"}`))
	req := httptest.NewRequest("POST", "/api/pay/item", body)
	resp := httptest.NewRecorder()
	PayItem(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}
