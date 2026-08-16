package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"redchef/db"
)

// ── Forgot Password ──

// ForgotPassword accepts an email, issues a single-use reset token and emails
// a reset link. It always returns 200 for both known and unknown emails so
// the endpoint can't be used to enumerate accounts.
func ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" || !strings.Contains(email, "@") {
		jsonError(w, "valid email is required", http.StatusBadRequest)
		return
	}

	if user, err := db.GetUserByEmail(email); err == nil {
		rawToken, tokenHash := db.GenerateResetToken()
		if err := db.CreatePasswordReset(user.ID, tokenHash, time.Now().Add(1*time.Hour)); err != nil {
			jsonError(w, "internal error", http.StatusInternalServerError)
			return
		}
		resetURL := baseURL(r) + "/reset?token=" + rawToken
		go SendPasswordResetEmail(user.Email, resetURL)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "If an account exists for that email, a reset link is on its way.",
	})
}

// ── Reset Password ──

// ResetPassword validates a single-use reset token and sets a new password.
// All of the user's sessions are invalidated so they must sign in again.
func ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token           string `json:"token"`
		Password        string `json:"password"`
		ConfirmPassword string `json:"confirm_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	token := strings.TrimSpace(req.Token)
	if token == "" {
		jsonError(w, "reset token is required", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 6 {
		jsonError(w, "password must be at least 6 characters", http.StatusBadRequest)
		return
	}
	if req.ConfirmPassword != "" && req.Password != req.ConfirmPassword {
		jsonError(w, "passwords do not match", http.StatusBadRequest)
		return
	}

	tokenHash := db.HashToken(token)
	reset, err := db.GetPasswordReset(tokenHash)
	if err != nil {
		jsonError(w, "invalid or expired reset token", http.StatusBadRequest)
		return
	}

	if err := db.UpdateUserPassword(reset.UserID, req.Password); err != nil {
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := db.ConsumePasswordReset(tokenHash); err != nil {
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Invalidate existing sessions so the old session can't outlive the change.
	db.DeleteUserSessionsForUser(reset.UserID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Password updated. You can now sign in.",
	})
}

// ── Helpers ──

// baseURL returns the public base URL used to build absolute links.
// Override with the BASE_URL env var (recommended behind a proxy); otherwise
// derive from the incoming request.
func baseURL(r *http.Request) string {
	if v := os.Getenv("BASE_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
