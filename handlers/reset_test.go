package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"redchef/db"
)

// ── Forgot Password ──

func createTestUser(t *testing.T, email, password string) int64 {
	t.Helper()
	id, err := db.CreateUser(email, password)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return id
}

func TestForgotPassword_ExistingEmail(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	userID := createTestUser(t, "reset@test.com", "password123")

	body := bytes.NewReader([]byte(`{"email":"reset@test.com"}`))
	req := httptest.NewRequest("POST", "/api/auth/forgot", body)
	req.Host = "example.com"
	resp := httptest.NewRecorder()
	ForgotPassword(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var count int
	db.DB.QueryRow("SELECT COUNT(*) FROM password_resets WHERE user_id = ?", userID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 reset row, got %d", count)
	}
}

func TestForgotPassword_UnknownEmail(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"email":"nobody@test.com"}`))
	req := httptest.NewRequest("POST", "/api/auth/forgot", body)
	resp := httptest.NewRecorder()
	ForgotPassword(resp, req)

	// Always 200 so accounts can't be enumerated
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	var count int
	db.DB.QueryRow("SELECT COUNT(*) FROM password_resets").Scan(&count)
	if count != 0 {
		t.Errorf("expected no reset rows for unknown email, got %d", count)
	}
}

func TestForgotPassword_InvalidEmail(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"email":""}`))
	req := httptest.NewRequest("POST", "/api/auth/forgot", body)
	resp := httptest.NewRecorder()
	ForgotPassword(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

// ── Reset Password ──

func TestResetPassword_Success(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	userID := createTestUser(t, "reset@test.com", "password123")

	// Create a session that should be invalidated on reset
	session, err := db.CreateUserSession(userID)
	if err != nil {
		t.Fatalf("CreateUserSession: %v", err)
	}

	rawToken, tokenHash := db.GenerateResetToken()
	if err := db.CreatePasswordReset(userID, tokenHash, time.Now().Add(1*time.Hour)); err != nil {
		t.Fatalf("CreatePasswordReset: %v", err)
	}

	body := bytes.NewReader([]byte(`{"token":"` + rawToken + `","password":"newpassword123","confirm_password":"newpassword123"}`))
	req := httptest.NewRequest("POST", "/api/auth/reset", body)
	resp := httptest.NewRecorder()
	ResetPassword(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// New password verifies, old one doesn't
	user, _ := db.GetUserByID(userID)
	if !db.CheckPassword("newpassword123", user.PasswordHash) {
		t.Error("expected new password to verify")
	}
	if db.CheckPassword("password123", user.PasswordHash) {
		t.Error("expected old password to be rejected")
	}

	// Reset is consumed (single use)
	if _, err := db.GetPasswordReset(tokenHash); err == nil {
		t.Error("expected reset to be consumed")
	}

	// All sessions invalidated
	if _, err := db.GetUserSession(session.Token); err == nil {
		t.Error("expected user sessions to be invalidated")
	}
}

func TestResetPassword_InvalidToken(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"token":"bogus","password":"newpassword123"}`))
	req := httptest.NewRequest("POST", "/api/auth/reset", body)
	resp := httptest.NewRecorder()
	ResetPassword(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestResetPassword_ExpiredToken(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	userID := createTestUser(t, "reset@test.com", "password123")
	rawToken, tokenHash := db.GenerateResetToken()
	if err := db.CreatePasswordReset(userID, tokenHash, time.Now().Add(-1*time.Hour)); err != nil {
		t.Fatalf("CreatePasswordReset: %v", err)
	}

	body := bytes.NewReader([]byte(`{"token":"` + rawToken + `","password":"newpassword123"}`))
	req := httptest.NewRequest("POST", "/api/auth/reset", body)
	resp := httptest.NewRecorder()
	ResetPassword(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}

	// Password unchanged
	user, _ := db.GetUserByID(userID)
	if !db.CheckPassword("password123", user.PasswordHash) {
		t.Error("expected original password to be unchanged")
	}
}

func TestResetPassword_TokenReuse(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	userID := createTestUser(t, "reset@test.com", "password123")
	rawToken, tokenHash := db.GenerateResetToken()
	if err := db.CreatePasswordReset(userID, tokenHash, time.Now().Add(1*time.Hour)); err != nil {
		t.Fatalf("CreatePasswordReset: %v", err)
	}

	resetBody := func() *httptest.ResponseRecorder {
		body := bytes.NewReader([]byte(`{"token":"` + rawToken + `","password":"newpassword123","confirm_password":"newpassword123"}`))
		req := httptest.NewRequest("POST", "/api/auth/reset", body)
		resp := httptest.NewRecorder()
		ResetPassword(resp, req)
		return resp
	}

	if resp := resetBody(); resp.Code != http.StatusOK {
		t.Fatalf("expected 200 on first use, got %d: %s", resp.Code, resp.Body.String())
	}
	if resp := resetBody(); resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on reuse, got %d", resp.Code)
	}
}

func TestResetPassword_ShortPassword(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"token":"abc","password":"12345"}`))
	req := httptest.NewRequest("POST", "/api/auth/reset", body)
	resp := httptest.NewRecorder()
	ResetPassword(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestResetPassword_PasswordMismatch(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"token":"abc","password":"password123","confirm_password":"different"}`))
	req := httptest.NewRequest("POST", "/api/auth/reset", body)
	resp := httptest.NewRecorder()
	ResetPassword(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}
