package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler(called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		w.WriteHeader(http.StatusOK)
	})
}

func TestCORSDefaultsToSameOrigin(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	called := false
	h := withMiddleware(okHandler(&called))

	req := httptest.NewRequest("GET", "http://redchef.example/api/posts", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !called || rec.Code != http.StatusOK {
		t.Fatalf("non-CORS read should still work: called=%v code=%d", called, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("unlisted origin must not receive CORS headers, got %q", got)
	}
}

func TestCORSSameOriginAndAllowlist(t *testing.T) {
	called := false
	h := withMiddleware(okHandler(&called))

	req := httptest.NewRequest("GET", "http://redchef.example/api/posts", nil)
	req.Header.Set("Origin", "http://redchef.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://redchef.example" {
		t.Errorf("same-origin ACAO = %q", got)
	}

	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example, https://other.example")
	req = httptest.NewRequest("GET", "http://redchef.example/api/posts", nil)
	req.Header.Set("Origin", "https://app.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example" {
		t.Errorf("allowlisted ACAO = %q", got)
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Errorf("expected Vary: Origin, got %q", got)
	}
}

func TestCORSRejectsUnlistedPreflight(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example")
	called := false
	h := withMiddleware(okHandler(&called))

	req := httptest.NewRequest("OPTIONS", "http://redchef.example/api/posts", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden || called {
		t.Fatalf("unlisted preflight should be rejected: code=%d called=%v", rec.Code, called)
	}

	req = httptest.NewRequest("OPTIONS", "http://redchef.example/api/posts", nil)
	req.Header.Set("Origin", "https://app.example")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || called {
		t.Fatalf("allowed preflight should be 204 without invoking handler: code=%d called=%v", rec.Code, called)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example" {
		t.Errorf("preflight ACAO = %q", got)
	}
}

func TestCookieMutationsRequireSameOrigin(t *testing.T) {
	called := false
	h := withMiddleware(okHandler(&called))

	// Cookie-authenticated cross-origin POST is CSRF and must be rejected.
	req := httptest.NewRequest("POST", "http://redchef.example/api/posts", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.AddCookie(&http.Cookie{Name: "session_token", Value: "abc"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || called {
		t.Fatalf("cross-origin cookie POST should be 403: code=%d called=%v", rec.Code, called)
	}

	// Same-origin browser POST works.
	req = httptest.NewRequest("POST", "http://redchef.example/api/posts", nil)
	req.Header.Set("Origin", "http://redchef.example")
	req.AddCookie(&http.Cookie{Name: "session_token", Value: "abc"})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !called {
		t.Fatalf("same-origin POST should pass: code=%d called=%v", rec.Code, called)
	}

	// Referer fallback is validated when Origin is absent.
	called = false
	req = httptest.NewRequest("POST", "http://redchef.example/api/posts", nil)
	req.Header.Set("Referer", "https://evil.example/page")
	req.AddCookie(&http.Cookie{Name: "admin_token", Value: "abc"})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || called {
		t.Fatalf("cross-origin referer should be 403: code=%d called=%v", rec.Code, called)
	}

	// Non-browser clients (no Origin/Referer) keep working.
	called = false
	req = httptest.NewRequest("POST", "http://redchef.example/api/posts", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: "abc"})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !called {
		t.Fatalf("non-browser cookie client should pass: code=%d called=%v", rec.Code, called)
	}

	// Cross-origin request without auth cookie (pure bearer/API client) passes.
	called = false
	req = httptest.NewRequest("POST", "http://redchef.example/api/posts", nil)
	req.Header.Set("Origin", "https://partner.example")
	req.Header.Set("Authorization", "Bearer rc_x")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !called {
		t.Fatalf("cookie-less API client should pass: code=%d called=%v", rec.Code, called)
	}
}
