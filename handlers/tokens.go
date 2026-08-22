package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"redchef/db"
)

// Token secrets are shown exactly once at creation; only the SHA-256 hash is stored.
const apiTokenPrefix = "rc_"

func generateApiTokenSecret() (secret string, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	secret = apiTokenPrefix + base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(secret))
	hash = hex.EncodeToString(sum[:])
	return secret, hash, nil
}

func hashApiToken(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// bearerToken extracts "Authorization: Bearer <token>".
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

// RequireMutationToken enforces the second credential for mutations: a valid
// bearer API token owned by the authenticated session user. It must be
// composed after AuthMiddleware + RequireAuth. Failures are uniform 401s so
// token existence is never disclosed.
func RequireMutationToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)
		if userID == 0 {
			jsonError(w, "valid API token required", http.StatusUnauthorized)
			return
		}
		secret := bearerToken(r)
		if secret == "" {
			jsonError(w, "valid API token required", http.StatusUnauthorized)
			return
		}
		token, err := db.GetApiTokenByHash(hashApiToken(secret))
		if err != nil || token.UserID != userID {
			jsonError(w, "valid API token required", http.StatusUnauthorized)
			return
		}
		// Constant-time confirmation of the hash match (defense in depth).
		if subtle.ConstantTimeCompare([]byte(token.TokenHash), []byte(hashApiToken(secret))) != 1 {
			jsonError(w, "valid API token required", http.StatusUnauthorized)
			return
		}
		db.TouchApiToken(token.ID)
		next.ServeHTTP(w, r)
	})
}

// CreateToken issues a new API token for the session user; the secret is
// returned once in this response and never again.
func CreateToken(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = "unnamed"
	}
	if len(name) > 100 {
		jsonError(w, "name too long (max 100 chars)", http.StatusBadRequest)
		return
	}
	secret, hash, err := generateApiTokenSecret()
	if err != nil {
		jsonError(w, "failed to generate token", http.StatusInternalServerError)
		return
	}
	id, err := db.CreateApiToken(userID, name, hash)
	if err != nil {
		jsonError(w, "failed to create token", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":         id,
		"name":       name,
		"token":      secret,
		"created_at": time.Now().UTC(),
	})
}

// ListTokens returns metadata for the session user's tokens (no secrets).
func ListTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := db.ListApiTokens(getUserID(r))
	if err != nil {
		jsonError(w, "failed to list tokens", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokens)
}

// RevokeToken revokes one of the session user's tokens by id.
func RevokeToken(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid token id", http.StatusBadRequest)
		return
	}
	if !db.RevokeApiToken(id, getUserID(r)) {
		jsonError(w, "token not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"revoked": true})
}

// RotateToken revokes an existing token and issues a replacement; the new
// secret is returned once.
func RotateToken(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid token id", http.StatusBadRequest)
		return
	}
	tokens, err := db.ListApiTokens(userID)
	if err != nil {
		jsonError(w, "failed to rotate token", http.StatusInternalServerError)
		return
	}
	var target *db.ApiToken
	for i := range tokens {
		if tokens[i].ID == id && tokens[i].RevokedAt == nil {
			target = &tokens[i]
			break
		}
	}
	if target == nil {
		jsonError(w, "token not found", http.StatusNotFound)
		return
	}
	db.RevokeApiToken(id, userID)
	secret, hash, err := generateApiTokenSecret()
	if err != nil {
		jsonError(w, "failed to rotate token", http.StatusInternalServerError)
		return
	}
	newID, err := db.CreateApiToken(userID, target.Name, hash)
	if err != nil {
		jsonError(w, "failed to rotate token", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":         newID,
		"name":       target.Name,
		"token":      secret,
		"created_at": time.Now().UTC(),
	})
}
