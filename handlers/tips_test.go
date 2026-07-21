package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"redchef/db"
)

func TestCreateTip_Success(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	_, err := db.CreatePost("Test", "", "photo", "test.jpg", "test.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	body := bytes.NewReader([]byte(`{"amount_cents":500}`))
	req := authenticatedRequest(t, "POST", "/api/posts/1/tip", body)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	CreateTip(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["ok"] != true {
		t.Error("expected ok=true")
	}
	if result["tip_count"] == nil {
		t.Error("expected tip_count")
	}
	if result["formatted"] != "€5,00" {
		t.Errorf("expected formatted '€5,00', got %v", result["formatted"])
	}
}

func TestCreateTip_MinimumAmount(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	_, err := db.CreatePost("Test", "", "photo", "test.jpg", "test.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	body := bytes.NewReader([]byte(`{"amount_cents":1}`))
	req := authenticatedRequest(t, "POST", "/api/posts/1/tip", body)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	CreateTip(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 for 1 cent, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestCreateTip_ZeroAmount(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"amount_cents":0}`))
	req := authenticatedRequest(t, "POST", "/api/posts/1/tip", body)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	CreateTip(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for zero amount, got %d", resp.Code)
	}
}

func TestCreateTip_NegativeAmount(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"amount_cents":-50}`))
	req := authenticatedRequest(t, "POST", "/api/posts/1/tip", body)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	CreateTip(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for negative amount, got %d", resp.Code)
	}
}

func TestCreateTip_Unauthenticated(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"amount_cents":100}`))
	req := httptest.NewRequest("POST", "/api/posts/1/tip", body)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	CreateTip(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestCreateTip_PostNotFound(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"amount_cents":100}`))
	req := authenticatedRequest(t, "POST", "/api/posts/999/tip", body)
	req.SetPathValue("id", "999")
	resp := httptest.NewRecorder()
	CreateTip(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}
}

func TestCreateTip_InvalidPostID(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"amount_cents":100}`))
	req := authenticatedRequest(t, "POST", "/api/posts/abc/tip", body)
	req.SetPathValue("id", "abc")
	resp := httptest.NewRecorder()
	CreateTip(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestCreateTip_Accumulates(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	_, err := db.CreatePost("Test", "", "photo", "test.jpg", "test.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	// Tip once
	body := bytes.NewReader([]byte(`{"amount_cents":200}`))
	req := authenticatedRequest(t, "POST", "/api/posts/1/tip", body)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	CreateTip(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("first tip: expected 201, got %d", resp.Code)
	}

	// Tip again
	body = bytes.NewReader([]byte(`{"amount_cents":300}`))
	req = authenticatedRequest(t, "POST", "/api/posts/1/tip", body)
	req.SetPathValue("id", "1")
	resp = httptest.NewRecorder()
	CreateTip(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("second tip: expected 201, got %d", resp.Code)
	}

	// Verify tip count and total
	count, _ := db.GetTipCount(1)
	total, _ := db.GetTotalTipAmount(1)
	if count != 2 {
		t.Errorf("expected 2 tips, got %d", count)
	}
	if total != 500 {
		t.Errorf("expected total 500 cents, got %d", total)
	}
}
