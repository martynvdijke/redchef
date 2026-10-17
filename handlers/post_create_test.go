package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"redchef/db"
)

func TestCreatePost_Unauthenticated(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	handler := AuthMiddleware(RequireAuth(http.HandlerFunc(AdminUpload)))
	req := httptest.NewRequest("POST", "/api/posts", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreatePost_Authenticated(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	imgData := createTestImage(t)
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("file", "test.jpg")
	part.Write(imgData)
	w.WriteField("title", "My Post")
	w.Close()

	// Build authenticated request
	req := authenticatedRequest(t, "POST", "/api/posts", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())

	handler := AuthMiddleware(RequireAuth(http.HandlerFunc(AdminUpload)))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var res map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if res["processing"] != true {
		t.Errorf("expected processing true, got %v", res["processing"])
	}
	if res["id"] == nil {
		t.Error("expected id in response")
	}
}

func TestCreatePost_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		authenticated bool
		wantCode   int
	}{
		{"unauthenticated -> 401", false, http.StatusUnauthorized},
		{"authenticated -> 201", true, http.StatusCreated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupHandlerTest(t)
			defer cleanup()

			handler := AuthMiddleware(RequireAuth(http.HandlerFunc(AdminUpload)))

			var req *http.Request
			if tt.authenticated {
				imgData := createTestImage(t)
				var buf bytes.Buffer
				w := multipart.NewWriter(&buf)
				part, _ := w.CreateFormFile("file", "test.jpg")
				part.Write(imgData)
				w.WriteField("title", "Table Post")
				w.Close()
				req = authenticatedRequest(t, "POST", "/api/posts", &buf)
				req.Header.Set("Content-Type", w.FormDataContentType())
			} else {
				req = httptest.NewRequest("POST", "/api/posts", nil)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.wantCode {
				t.Fatalf("expected %d, got %d: %s", tt.wantCode, rec.Code, rec.Body.String())
			}
			if tt.wantCode == http.StatusCreated {
				var res map[string]interface{}
				json.NewDecoder(rec.Body).Decode(&res)
				if res["processing"] != true {
					t.Errorf("expected processing true")
				}
			}
		})
	}
}

// Ensure db import is used
var _ = db.GetPost
var _ = fmt.Sprintf
