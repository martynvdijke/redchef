package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"redchef/db"
)

func TestRemoveFavourite(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			"unauthenticated -> 401",
			func(t *testing.T) {
				cleanup := setupHandlerTest(t)
				defer cleanup()
				db.CreatePost("P", "", "photo", "a.jpg", "a.jpg", false)
				handler := AuthMiddleware(RequireAuth(http.HandlerFunc(RemoveFavourite)))
				req := httptest.NewRequest("DELETE", "/api/favourites/1", nil)
				req.SetPathValue("id", "1")
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				if rec.Code != http.StatusUnauthorized {
					t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
				}
			},
		},
		{
			"authenticated remove -> 200 with favourited false and decremented count",
			func(t *testing.T) {
				cleanup := setupHandlerTest(t)
				defer cleanup()
				_, err := db.CreatePost("P", "", "photo", "a.jpg", "a.jpg", false)
				if err != nil {
					t.Fatalf("create: %v", err)
				}
				// favourite first
				db.ToggleFavourite(1, 1)
				countBefore, _ := db.GetFavouriteCount(1)
				if countBefore != 1 {
					t.Fatalf("expected count 1 before, got %d", countBefore)
				}
				req := authenticatedRequest(t, "DELETE", "/api/favourites/1", nil)
				req.SetPathValue("id", "1")
				rec := httptest.NewRecorder()
				RemoveFavourite(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
				}
				var res map[string]interface{}
				json.NewDecoder(rec.Body).Decode(&res)
				if res["favourited"] != false {
					t.Errorf("expected favourited false, got %v", res["favourited"])
				}
				if int(res["favourite_count"].(float64)) != 0 {
					t.Errorf("expected count 0, got %v", res["favourite_count"])
				}
			},
		},
		{
			"idempotent second call still 200",
			func(t *testing.T) {
				cleanup := setupHandlerTest(t)
				defer cleanup()
				db.CreatePost("P", "", "photo", "a.jpg", "a.jpg", false)
				db.ToggleFavourite(1, 1)
				// first remove
				req := authenticatedRequest(t, "DELETE", "/api/favourites/1", nil)
				req.SetPathValue("id", "1")
				rec := httptest.NewRecorder()
				RemoveFavourite(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("first remove: %d", rec.Code)
				}
				// second remove idempotent
				req2 := authenticatedRequest(t, "DELETE", "/api/favourites/1", nil)
				req2.SetPathValue("id", "1")
				rec2 := httptest.NewRecorder()
				RemoveFavourite(rec2, req2)
				if rec2.Code != http.StatusOK {
					t.Fatalf("second remove expected 200, got %d: %s", rec2.Code, rec2.Body.String())
				}
				var res map[string]interface{}
				json.NewDecoder(rec2.Body).Decode(&res)
				if res["favourited"] != false {
					t.Errorf("expected favourited false on second call")
				}
				if int(res["favourite_count"].(float64)) != 0 {
					t.Errorf("expected count 0 on second call, got %v", res["favourite_count"])
				}
			},
		},
		{
			"unknown post -> 404",
			func(t *testing.T) {
				cleanup := setupHandlerTest(t)
				defer cleanup()
				req := authenticatedRequest(t, "DELETE", "/api/favourites/999", nil)
				req.SetPathValue("id", "999")
				rec := httptest.NewRecorder()
				RemoveFavourite(rec, req)
				if rec.Code != http.StatusNotFound {
					t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
				}
			},
		},
		{
			"hidden draft post -> 404",
			func(t *testing.T) {
				cleanup := setupHandlerTest(t)
				defer cleanup()
				post, _ := db.CreatePost("Draft", "", "photo", "a.jpg", "a.jpg", false)
				db.SetPostStatus(post.ID, db.PostStatusDraft, nil)
				req := authenticatedRequest(t, "DELETE", "/api/favourites/1", nil)
				req.SetPathValue("id", "1")
				// need fresh auth after status change (ProcessWG not needed but safe)
				rec := httptest.NewRecorder()
				RemoveFavourite(rec, req)
				if rec.Code != http.StatusNotFound {
					t.Fatalf("expected 404 for draft, got %d: %s", rec.Code, rec.Body.String())
				}
			},
		},
		{
			"scheduled future post -> 404",
			func(t *testing.T) {
				cleanup := setupHandlerTest(t)
				defer cleanup()
				post, _ := db.CreatePost("Sched", "", "photo", "a.jpg", "a.jpg", false)
				future := time.Now().Add(24 * time.Hour)
				db.SetPostStatus(post.ID, db.PostStatusPublished, &future)
				req := authenticatedRequest(t, "DELETE", "/api/favourites/1", nil)
				req.SetPathValue("id", "1")
				rec := httptest.NewRecorder()
				RemoveFavourite(rec, req)
				if rec.Code != http.StatusNotFound {
					t.Fatalf("expected 404 for scheduled future, got %d: %s", rec.Code, rec.Body.String())
				}
			},
		},
		{
			"invalid id -> 400",
			func(t *testing.T) {
				cleanup := setupHandlerTest(t)
				defer cleanup()
				req := authenticatedRequest(t, "DELETE", "/api/favourites/abc", nil)
				req.SetPathValue("id", "abc")
				rec := httptest.NewRecorder()
				RemoveFavourite(rec, req)
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("expected 400, got %d", rec.Code)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func TestRemoveFavouriteViaMiddleware_Unauthenticated(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()
	db.CreatePost("P", "", "photo", "a.jpg", "a.jpg", false)
	handler := AuthMiddleware(RequireAuth(http.HandlerFunc(RemoveFavourite)))
	req := httptest.NewRequest("DELETE", "/api/favourites/1", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 via middleware, got %d", rec.Code)
	}
}
