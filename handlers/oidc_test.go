package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestOIDCConfigValid(t *testing.T) {
	// valid when all required fields present
	cfg := OIDCConfig{Enabled: true, Issuer: "https://authelia.vandijke.xyz", ClientID: "redchef", Secret: "s", RedirectURL: "https://redchef.vandijke.xyz/api/auth/oidc/callback"}
	if !cfg.valid() {
		t.Error("expected valid")
	}
	cfg.Enabled = false
	if cfg.valid() {
		t.Error("expected invalid when disabled")
	}
	cfg.Enabled = true
	cfg.Secret = ""
	if cfg.valid() {
		t.Error("expected invalid when secret empty")
	}
}

func TestOIDCConfigEnvParsing(t *testing.T) {
	t.Setenv("OIDC_ENABLED", "false")
	t.Setenv("OIDC_ISSUER_URL", "https://authelia.vandijke.xyz")
	t.Setenv("OIDC_CLIENT_ID", "redchef")
	t.Setenv("OIDC_CLIENT_SECRET", "secret123")
	t.Setenv("OIDC_REDIRECT_URL", "https://redchef.vandijke.xyz/api/auth/oidc/callback")
	t.Setenv("OIDC_SCOPES", "")
	if cfg := loadOIDCConfig(); cfg.valid() {
		t.Error("expected invalid when OIDC_ENABLED=false")
	}
	t.Setenv("OIDC_ENABLED", "true")
	if cfg := loadOIDCConfig(); !cfg.valid() {
		t.Error("expected valid when enabled and configured")
	}
	// 0 disables
	t.Setenv("OIDC_ENABLED", "0")
	if cfg := loadOIDCConfig(); cfg.valid() {
		t.Error("expected invalid when OIDC_ENABLED=0")
	}
	// secret file fallback: create temp file
	f, _ := os.CreateTemp("", "oidc-secret-*")
	f.WriteString("  file-secret  \n")
	f.Close()
	defer os.Remove(f.Name())
	t.Setenv("OIDC_ENABLED", "true")
	t.Setenv("OIDC_CLIENT_SECRET", "")
	t.Setenv("OIDC_CLIENT_SECRET_FILE", f.Name())
	if cfg := loadOIDCConfig(); cfg.Secret != "file-secret" {
		t.Errorf("expected trimmed file secret, got %q", cfg.Secret)
	}
}

func TestOidcRoleForGroups(t *testing.T) {
	if oidcRoleForGroups([]string{"users"}) != "normal" {
		t.Error("expected normal")
	}
	if oidcRoleForGroups([]string{"admins"}) != "admin" {
		t.Error("expected admin")
	}
	if oidcRoleForGroups([]string{"users", "admins"}) != "admin" {
		t.Error("expected admin when admins present")
	}
	if oidcRoleForGroups(nil) != "normal" {
		t.Error("expected normal for nil")
	}
}

func TestOIDCStatus_Disabled(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()
	t.Setenv("OIDC_ENABLED", "false")
	t.Setenv("OIDC_ISSUER_URL", "")
	req := httptest.NewRequest("GET", "/api/auth/oidc/status", nil)
	w := httptest.NewRecorder()
	OIDCStatus(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if body := w.Body.String(); body != `{"enabled":false}`+"\n" {
		t.Errorf("unexpected body %q", body)
	}
}

func TestOIDCCallback_BadState(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()
	t.Setenv("OIDC_ENABLED", "true")
	t.Setenv("OIDC_ISSUER_URL", "https://authelia.vandijke.xyz")
	t.Setenv("OIDC_CLIENT_ID", "redchef")
	t.Setenv("OIDC_CLIENT_SECRET", "secret")
	t.Setenv("OIDC_REDIRECT_URL", "https://redchef.vandijke.xyz/api/auth/oidc/callback")

	// No cookies -> expired
	req := httptest.NewRequest("GET", "/api/auth/oidc/callback?state=foo&code=bar", nil)
	w := httptest.NewRecorder()
	OIDCCallback(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("expected redirect got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc == "" {
		t.Error("expected redirect location")
	}

	// Cookies present but state mismatch -> rejected
	req = httptest.NewRequest("GET", "/api/auth/oidc/callback?state=wrong&code=bar", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "correct-state"})
	req.AddCookie(&http.Cookie{Name: "oidc_nonce", Value: "nonce"})
	req.AddCookie(&http.Cookie{Name: "oidc_verifier", Value: "verifier"})
	w = httptest.NewRecorder()
	OIDCCallback(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("expected redirect on bad state got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/?error=oidc_state" {
		t.Errorf("expected oidc_state redirect got %q", loc)
	}

	// Missing state query param
	req = httptest.NewRequest("GET", "/api/auth/oidc/callback?code=bar", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "s"})
	req.AddCookie(&http.Cookie{Name: "oidc_nonce", Value: "n"})
	req.AddCookie(&http.Cookie{Name: "oidc_verifier", Value: "v"})
	w = httptest.NewRecorder()
	OIDCCallback(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("expected redirect on absent state got %d", w.Code)
	}
}

func TestOIDCLogin_DisabledReturns404(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()
	t.Setenv("OIDC_ENABLED", "false")
	req := httptest.NewRequest("GET", "/api/auth/oidc/login", nil)
	w := httptest.NewRecorder()
	OIDCLogin(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", w.Code)
	}
}
