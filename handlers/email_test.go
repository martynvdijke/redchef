package handlers

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"redchef/db"
)

func enableDemoAccount(t *testing.T) {
	t.Helper()
	t.Setenv("DEMO_USER_ENABLED", "true")
	t.Setenv("DEMO_USER_EMAIL", "demo@redchef.local")
	t.Setenv("DEMO_USER_PASSWORD", "redchef-demo")
	if _, err := db.EnsureDemoUser("demo@redchef.local", "redchef-demo"); err != nil {
		t.Fatalf("seed demo account: %v", err)
	}
}

func TestIsDemoNotificationTarget(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()
	enableDemoAccount(t)

	if !isDemoNotificationTarget(" Demo@RedChef.Local ") {
		t.Fatal("configured demo email should be muted")
	}
	if isDemoNotificationTarget("someone@example.com") {
		t.Fatal("ordinary email should not be muted")
	}
	if isDemoNotificationTarget("   ") {
		t.Fatal("empty email should not be muted")
	}

	// A stored demo-role account stays muted even when the feature is disabled
	// or the configured address changes later.
	t.Setenv("DEMO_USER_ENABLED", "false")
	t.Setenv("DEMO_USER_EMAIL", "renamed@redchef.local")
	if !isDemoNotificationTarget("demo@redchef.local") {
		t.Fatal("stored demo account should stay muted")
	}
}

func TestNotificationsSkipDemoAccount(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()
	enableDemoAccount(t)

	var gotifyCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotifyCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := db.UpdateEmailSettings("", 0, "", "", "", "", srv.URL, "test-token"); err != nil {
		t.Fatalf("configure email settings: %v", err)
	}

	SendTipNotification("demo@redchef.local", "Soep", 500)
	if n := gotifyCalls.Load(); n != 0 {
		t.Fatalf("demo tip produced %d Gotify notifications, want 0", n)
	}

	SendTipNotification("fan@example.com", "Soep", 500)
	if n := gotifyCalls.Load(); n != 1 {
		t.Fatalf("member tip produced %d Gotify notifications, want 1", n)
	}
}
