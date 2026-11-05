package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"redchef/db"
)

func TestDemoConfigDefaultsAndEnv(t *testing.T) {
	t.Setenv("DEMO_USER_ENABLED", "")
	t.Setenv("DEMO_USER_EMAIL", "")
	t.Setenv("DEMO_USER_PASSWORD", "")
	t.Setenv("LEGAL_BASE_URL", "https://example.com")
	t.Setenv("BASE_URL", "")

	cfg := demoConfig()
	if cfg.Enabled {
		t.Fatalf("expected demo disabled off the official host, got %+v", cfg)
	}
	if cfg.Email != defaultDemoEmail || cfg.Password != defaultDemoPassword {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}

	t.Setenv("DEMO_USER_ENABLED", "true")
	t.Setenv("DEMO_USER_EMAIL", "Demo@Example.COM")
	t.Setenv("DEMO_USER_PASSWORD", "custom-pass")
	cfg = demoConfig()
	if !cfg.Enabled {
		t.Fatal("expected demo enabled")
	}
	if cfg.Email != "demo@example.com" || cfg.Password != "custom-pass" {
		t.Fatalf("env overrides not applied: %+v", cfg)
	}

	t.Setenv("DEMO_USER_ENABLED", "no")
	if demoConfig().Enabled {
		t.Fatal("expected 'no' to disable demo")
	}

	t.Setenv("DEMO_USER_ENABLED", "0")
	if demoConfig().Enabled {
		t.Fatal("expected '0' to disable demo")
	}
}

func TestDemoConfigAutoEnablesOnOfficialHost(t *testing.T) {
	t.Setenv("DEMO_USER_ENABLED", "")
	t.Setenv("LEGAL_BASE_URL", "https://redchef.vandijke.xyz/path")
	t.Setenv("BASE_URL", "")

	if !demoConfig().Enabled {
		t.Fatal("expected demo to auto-enable on redchef.vandijke.xyz")
	}

	t.Setenv("LEGAL_BASE_URL", "")
	t.Setenv("BASE_URL", "https://redchef.vandijke.xyz")
	if !demoConfig().Enabled {
		t.Fatal("expected demo to auto-enable from BASE_URL")
	}

	t.Setenv("BASE_URL", "https://not-redchef.vandijke.xyz")
	if demoConfig().Enabled {
		t.Fatal("expected different host to keep demo disabled")
	}
}

func TestEnsureDemoUserCreatesAndRepairs(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()
	t.Setenv("DEMO_USER_ENABLED", "true")
	t.Setenv("DEMO_USER_EMAIL", "demo@redchef.local")
	t.Setenv("DEMO_USER_PASSWORD", "redchef-demo")

	EnsureDemoUser()
	user, err := db.GetUserByEmail("demo@redchef.local")
	if err != nil {
		t.Fatalf("demo user not created: %v", err)
	}
	full, err := db.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("get demo by id: %v", err)
	}
	if full.Role != "demo" || !full.Paid || full.Username != "demo@redchef.local" {
		t.Fatalf("unexpected demo account: %+v", full)
	}
	if !db.CheckPassword("redchef-demo", user.PasswordHash) {
		t.Fatal("demo password not set")
	}

	// Calling again must be idempotent.
	EnsureDemoUser()
	again, err := db.GetUserByEmail("demo@redchef.local")
	if err != nil {
		t.Fatalf("demo user disappeared: %v", err)
	}
	if again.ID != user.ID {
		t.Fatalf("demo user was replaced: %d != %d", again.ID, user.ID)
	}

	// A drifted password or role is repaired on the next boot.
	if err := db.UpdateUserPassword(user.ID, "something-else"); err != nil {
		t.Fatalf("update password: %v", err)
	}
	if _, err := db.DB.Exec("UPDATE users SET username = 'stale-demo', paid = 0 WHERE id = ?", user.ID); err != nil {
		t.Fatalf("corrupt demo row: %v", err)
	}
	EnsureDemoUser()
	repaired, err := db.GetUserByEmail("demo@redchef.local")
	if err != nil {
		t.Fatalf("get repaired demo: %v", err)
	}
	if repaired.Role != "demo" || !repaired.Paid {
		t.Fatalf("demo account not repaired: %+v", repaired)
	}
	if !db.CheckPassword("redchef-demo", repaired.PasswordHash) {
		t.Fatal("demo password not repaired")
	}
}

func TestDeleteMeBlocksDemo(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()
	t.Setenv("DEMO_USER_ENABLED", "true")
	t.Setenv("DEMO_USER_EMAIL", "demo@redchef.local")
	t.Setenv("DEMO_USER_PASSWORD", "redchef-demo")
	EnsureDemoUser()

	demo, err := db.GetUserByEmail("demo@redchef.local")
	if err != nil {
		t.Fatalf("get demo: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/me", strings.NewReader(`{"confirm":"demo@redchef.local"}`))
	req.Header.Set("X-User-ID", fmt.Sprintf("%d", demo.ID))
	rec := httptest.NewRecorder()
	DeleteMe(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for demo deletion, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "demo account cannot be deleted") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
	if _, err := db.GetUserByID(demo.ID); err != nil {
		t.Fatalf("demo account was deleted: %v", err)
	}
}

func TestDemoConfigHandler(t *testing.T) {
	t.Setenv("DEMO_USER_ENABLED", "false")

	rec := httptest.NewRecorder()
	DemoConfigHandler(rec, httptest.NewRequest(http.MethodGet, "/api/auth/config", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var disabled struct {
		Demo DemoConfig `json:"demo"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &disabled); err != nil {
		t.Fatalf("decode disabled config: %v", err)
	}
	if disabled.Demo.Enabled || disabled.Demo.Email != "" || disabled.Demo.Password != "" {
		t.Fatalf("disabled config leaked credentials: %+v", disabled.Demo)
	}

	t.Setenv("DEMO_USER_ENABLED", "true")
	t.Setenv("DEMO_USER_EMAIL", "demo@redchef.local")
	t.Setenv("DEMO_USER_PASSWORD", "redchef-demo")
	rec = httptest.NewRecorder()
	DemoConfigHandler(rec, httptest.NewRequest(http.MethodGet, "/api/auth/config", nil))
	var enabled struct {
		Demo DemoConfig `json:"demo"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &enabled); err != nil {
		t.Fatalf("decode enabled config: %v", err)
	}
	if !enabled.Demo.Enabled || enabled.Demo.Email != "demo@redchef.local" || enabled.Demo.Password != "redchef-demo" {
		t.Fatalf("unexpected enabled config: %+v", enabled.Demo)
	}
}
