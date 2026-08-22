package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"redchef/db"
)

// chain builds the same middleware stack main.go uses for mutations.
func mutationChain(next http.Handler) http.Handler {
	return AuthMiddleware(RequireAuth(RequireMutationToken(next)))
}

func createTokenFor(t *testing.T, name string) (secret string, id int64) {
	t.Helper()
	req := authenticatedRequest(t, "POST", "/api/tokens", strings.NewReader(`{"name":"`+name+`"}`))
	resp := httptest.NewRecorder()
	CreateToken(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("CreateToken: expected 201, got %d: %s", resp.Code, resp.Body.String())
	}
	var out struct {
		ID     int64  `json:"id"`
		Token  string `json:"token"`
		Name   string `json:"name"`
		Secret string `json:"-"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	return out.Token, out.ID
}

func TestMutationRequiresToken(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	// Session but no bearer → 401, handler not called
	req := authenticatedRequest(t, "POST", "/api/posts/1/favourite", nil)
	resp := httptest.NewRecorder()
	mutationChain(next).ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", resp.Code)
	}
	if called {
		t.Error("handler must not run without token")
	}

	// Anonymous with only a bearer → 401
	req2 := httptest.NewRequest("POST", "/api/posts/1/favourite", nil)
	req2.Header.Set("Authorization", "Bearer rc_sometoken")
	resp2 := httptest.NewRecorder()
	mutationChain(next).ServeHTTP(resp2, req2)
	if resp2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for anonymous, got %d", resp2.Code)
	}
}

func TestTokenLifecycleAndMutation(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	secret, id := createTokenFor(t, "ci")

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	// Valid session + valid token → allowed
	req := authenticatedRequest(t, "POST", "/api/posts/1/favourite", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	resp := httptest.NewRecorder()
	mutationChain(next).ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !called {
		t.Fatalf("valid token rejected: %d %s", resp.Code, resp.Body.String())
	}

	// List must not contain the secret
	req = authenticatedRequest(t, "GET", "/api/tokens", nil)
	resp = httptest.NewRecorder()
	ListTokens(resp, req)
	body := resp.Body.String()
	if strings.Contains(body, secret) {
		t.Fatal("token secret leaked in list response")
	}
	var tokens []db.ApiToken
	json.NewDecoder(resp.Body).Decode(&tokens)
	if len(tokens) != 1 || tokens[0].Name != "ci" {
		t.Fatalf("unexpected token list: %+v", tokens)
	}

	// Revoke → mutation rejected
	req = authenticatedRequest(t, "DELETE", "/api/tokens/1", nil)
	req.SetPathValue("id", "1")
	resp = httptest.NewRecorder()
	RevokeToken(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("revoke failed: %d", resp.Code)
	}

	req = authenticatedRequest(t, "POST", "/api/posts/1/favourite", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	resp = httptest.NewRecorder()
	mutationChain(next).ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token accepted, got %d", resp.Code)
	}
	_ = id
}

func TestForeignAndMalformedTokensRejected(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	secret, _ := createTokenFor(t, "mine")

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	// Malformed header
	req := authenticatedRequest(t, "POST", "/x", nil)
	req.Header.Set("Authorization", "Bearer")
	resp := httptest.NewRecorder()
	mutationChain(next).ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized || called {
		t.Fatalf("malformed bearer accepted: %d", resp.Code)
	}

	// Unknown token — response must be identical to revoked case (uniform 401 body)
	req = authenticatedRequest(t, "POST", "/x", nil)
	req.Header.Set("Authorization", "Bearer rc_totallyunknownvalue9999")
	resp = httptest.NewRecorder()
	mutationChain(next).ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized || called {
		t.Fatalf("unknown token accepted: %d", resp.Code)
	}
	if strings.Contains(resp.Body.String(), "rc_") {
		t.Log("uniform error body:", resp.Body.String())
	}

	// Valid secret still works afterwards (no state was mutated by failures)
	req = authenticatedRequest(t, "POST", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	resp = httptest.NewRecorder()
	mutationChain(next).ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !called {
		t.Fatalf("valid token broke after failures: %d", resp.Code)
	}
}

func TestRotateToken(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	oldSecret, oldID := createTokenFor(t, "rotate-me")

	req := authenticatedRequest(t, "POST", "/api/tokens/1/rotate", nil)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	RotateToken(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("rotate failed: %d %s", resp.Code, resp.Body.String())
	}
	var out struct {
		Token string `json:"token"`
		ID    int64  `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if out.Token == "" || out.Token == oldSecret || out.ID == oldID {
		t.Fatal("rotate did not produce a fresh secret/id")
	}

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	// Old secret dead
	req = authenticatedRequest(t, "POST", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+oldSecret)
	resp = httptest.NewRecorder()
	mutationChain(next).ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized || called {
		t.Fatal("old secret still valid after rotate")
	}

	// New secret works
	req = authenticatedRequest(t, "POST", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+out.Token)
	resp = httptest.NewRecorder()
	mutationChain(next).ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !called {
		t.Fatal("rotated secret rejected")
	}
}
