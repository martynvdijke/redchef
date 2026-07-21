package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"redchef/db"
)

// ── Types ──

type AuthRequest struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password,omitempty"`
}

type AuthResponse struct {
	Token string `json:"token"`
	Role  string `json:"role"`
}

// ── Register ──

func Register(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		jsonError(w, "valid email is required", http.StatusBadRequest)
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

	// Check if email already taken
	if existing, _ := db.GetUserByEmail(req.Email); existing != nil {
		jsonError(w, "email already registered", http.StatusConflict)
		return
	}

	userID, err := db.CreateUser(req.Email, req.Password)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			jsonError(w, "email already registered", http.StatusConflict)
		} else {
			jsonError(w, "failed to create user", http.StatusInternalServerError)
		}
		return
	}

	// Notify the new member their account was created (async, best-effort)
	go SendWelcomeEmail(req.Email)

	// Auto-login: create session
	session, err := db.CreateUserSession(userID)
	if err != nil {
		jsonError(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	// Get user to determine role
	user, _ := db.GetUserByID(userID)

	setSessionCookie(w, session.Token)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(AuthResponse{Token: session.Token, Role: user.Role})
}

// ── Login ──

func Login(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		jsonError(w, "email and password are required", http.StatusBadRequest)
		return
	}

	// Try email first, fall back to username (for existing admin users)
	user, err := db.GetUserByEmail(req.Email)
	if err != nil {
		// Fallback: try username lookup
		user, err = db.GetUserByUsername(req.Email)
		if err != nil {
			jsonError(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
	}

	if !db.CheckPassword(req.Password, user.PasswordHash) {
		jsonError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	session, err := db.CreateUserSession(user.ID)
	if err != nil {
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, session.Token)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{Token: session.Token, Role: user.Role})
}

// ── Logout ──

func Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		db.DeleteUserSession(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	// Also clear old admin_token if present
	if oldCookie, err := r.Cookie("admin_token"); err == nil {
		db.DeleteSession(oldCookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:   "admin_token",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

// ── Me (current user) ──

func Me(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"authenticated": false,
		})
		return
	}

	user, err := db.GetUserByID(userID)
	if err != nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"authenticated": true,
		"id":            user.ID,
		"email":         user.Email,
		"role":          user.Role,
		"paid":          user.Paid,
	})
}

// ── Middleware ──

// SessionInfo extracts user info from the request context (set by AuthMiddleware or AdminAuth).
type contextKey string

const (
	ctxUserID contextKey = "user_id"
	ctxRole   contextKey = "role"
)

// AuthMiddleware adds user context for every request, doesn't block unauthenticated users.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := getSessionToken(r)
		if token != "" {
			if session, err := db.GetUserSession(token); err == nil {
				user, err := db.GetUserByID(session.UserID)
				if err == nil {
					r.Header.Set("X-User-ID", fmt.Sprintf("%d", user.ID))
					r.Header.Set("X-User-Role", user.Role)
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAuth blocks unauthenticated users.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)
		if userID == 0 {
			jsonError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin blocks non-admin users.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role := r.Header.Get("X-User-Role")
		if role != "admin" {
			jsonError(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ── Legacy AdminAuth (for backward compatibility during migration) ──
// Tries session_token first, falls back to admin_token.

func AdminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := getSessionToken(r)
		if token != "" {
			if session, err := db.GetUserSession(token); err == nil {
				user, err := db.GetUserByID(session.UserID)
				if err == nil && user.Role == "admin" {
					r.Header.Set("X-User-ID", fmt.Sprintf("%d", user.ID))
					r.Header.Set("X-User-Role", "admin")
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		// Fallback: try admin_token cookie
		cookie, err := r.Cookie("admin_token")
		if err != nil {
			jsonError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		session, err := db.GetSession(cookie.Value)
		if err != nil {
			jsonError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		r.Header.Set("X-User-ID", fmt.Sprintf("%d", session.UserID))
		next.ServeHTTP(w, r)
	})
}

// ── Helpers ──

func getUserID(r *http.Request) int64 {
	idStr := r.Header.Get("X-User-ID")
	if idStr == "" {
		return 0
	}
	id, _ := strconv.ParseInt(idStr, 10, 64)
	return id
}

func getSessionToken(r *http.Request) string {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		return ""
	}
	return cookie.Value
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 86400, // 30 days
	})
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
