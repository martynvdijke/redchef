package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"redchef/db"
)

func TestPrivacyPageGDPRContent(t *testing.T) {
	t.Setenv("LEGAL_SUPERVISORY_AUTHORITY", "your local supervisory authority")
	t.Setenv("LEGAL_DPO_EMAIL", "dpo@example.com")
	t.Setenv("LEGAL_EU_REPRESENTATIVE", "Example EU Rep GmbH")

	rec := httptest.NewRecorder()
	PrivacyPage(rec, httptest.NewRequest(http.MethodGet, "/privacy", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Data deletion",
		"supervisory authority",
		"dpo@example.com",
		"Example EU Rep GmbH",
		"/account/delete",
		"/api/me/export",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("privacy page missing %q", want)
		}
	}
}

func TestAccountDeletePageHasPublicForm(t *testing.T) {
	t.Setenv("DEMO_USER_ENABLED", "false")

	rec := httptest.NewRecorder()
	AccountDeletePage(rec, httptest.NewRequest(http.MethodGet, "/account/delete", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `action="/account/delete"`) {
		t.Fatal("deletion page has no POST form")
	}
	if !strings.Contains(body, `method="post"`) {
		t.Fatal("deletion page form is not a POST")
	}
	if !strings.Contains(body, "Delete without signing in") {
		t.Fatal("deletion page missing public pathway heading")
	}
}

func TestRequestAccountDeletionWrongPassword(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()
	t.Setenv("DEMO_USER_ENABLED", "false")

	form := url.Values{
		"email":    {"admin"},
		"password": {"wrong-password"},
		"confirm":  {"admin"},
	}
	req := httptest.NewRequest(http.MethodPost, "/account/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	RequestAccountDeletion(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "email or password is incorrect") {
		t.Fatalf("expected generic error, got %s", rec.Body.String())
	}
	if _, err := db.GetUserByEmail("admin"); err != nil {
		t.Fatalf("user should still exist: %v", err)
	}
}

func TestRequestAccountDeletionDeletesAndRedirects(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()
	t.Setenv("DEMO_USER_ENABLED", "false")

	if _, err := db.CreateUser("user@example.com", "secret12"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	form := url.Values{
		"email":    {"user@example.com"},
		"password": {"secret12"},
		"confirm":  {"User@Example.com"},
	}
	req := httptest.NewRequest(http.MethodPost, "/account/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	RequestAccountDeletion(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/account/delete?deleted=1" {
		t.Fatalf("unexpected redirect location %q", loc)
	}
	if _, err := db.GetUserByEmail("user@example.com"); err == nil {
		t.Fatal("user still exists after deletion")
	}
}

func TestRequestAccountDeletionSignedInConfirmOnly(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()
	t.Setenv("DEMO_USER_ENABLED", "false")

	if _, err := db.CreateUser("member@example.com", "secret12"); err != nil {
		t.Fatalf("create member: %v", err)
	}
	member, err := db.GetUserByEmail("member@example.com")
	if err != nil {
		t.Fatalf("get member: %v", err)
	}

	form := url.Values{"confirm": {"member@example.com"}}
	req := httptest.NewRequest(http.MethodPost, "/account/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-User-ID", fmt.Sprintf("%d", member.ID))
	rec := httptest.NewRecorder()
	RequestAccountDeletion(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := db.GetUserByEmail("member@example.com"); err == nil {
		t.Fatal("signed-in member still exists after deletion")
	}
}

func TestRequestAccountDeletionBlocksDemo(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()
	t.Setenv("DEMO_USER_ENABLED", "true")
	t.Setenv("DEMO_USER_EMAIL", "demo@redchef.local")
	t.Setenv("DEMO_USER_PASSWORD", "redchef-demo")
	EnsureDemoUser()

	form := url.Values{
		"email":    {"demo@redchef.local"},
		"password": {"redchef-demo"},
		"confirm":  {"demo@redchef.local"},
	}
	req := httptest.NewRequest(http.MethodPost, "/account/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	RequestAccountDeletion(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for demo deletion, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := db.GetUserByEmail("demo@redchef.local"); err != nil {
		t.Fatalf("demo account should still exist: %v", err)
	}
}
