package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"redchef/db"
)

func createTokenFor(t *testing.T, name string) (secret string, id int64) {
	t.Helper()
	req := authenticatedRequest(t, "POST", "/api/tokens", strings.NewReader(`{"name":"`+name+`"}`))
	resp := httptest.NewRecorder()
	CreateToken(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("CreateToken: expected 201, got %d: %s", resp.Code, resp.Body.String())
	}
	var out struct {
		ID    int64  `json:"id"`
		Token string `json:"token"`
		Name  string `json:"name"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	return out.Token, out.ID
}

func TestTokenLifecycle(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	secret, _ := createTokenFor(t, "ci")
	if !strings.HasPrefix(secret, apiTokenPrefix) {
		t.Fatalf("token should carry prefix %q, got %q", apiTokenPrefix, secret)
	}

	// List must not contain the secret.
	req := authenticatedRequest(t, "GET", "/api/tokens", nil)
	resp := httptest.NewRecorder()
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

	// Revoke removes it.
	req = authenticatedRequest(t, "DELETE", "/api/tokens/1", nil)
	req.SetPathValue("id", "1")
	resp = httptest.NewRecorder()
	RevokeToken(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("revoke failed: %d", resp.Code)
	}
	if _, err := db.GetApiTokenByHash(hashApiToken(secret)); err == nil {
		t.Fatal("revoked token hash still resolves")
	}
}

func TestTokenSecretsAreHashed(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	secret, _ := createTokenFor(t, "hashed")
	var stored string
	if err := db.DB.QueryRow("SELECT token_hash FROM api_tokens LIMIT 1").Scan(&stored); err != nil {
		t.Fatalf("read token hash: %v", err)
	}
	if stored == secret || strings.Contains(stored, secret) {
		t.Fatal("token must be stored hashed, not plaintext")
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

	if _, err := db.GetApiTokenByHash(hashApiToken(oldSecret)); err == nil {
		t.Fatal("old secret still valid after rotate")
	}
	if _, err := db.GetApiTokenByHash(hashApiToken(out.Token)); err != nil {
		t.Fatal("new secret not stored")
	}
}
