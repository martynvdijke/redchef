package db

import (
	"database/sql"
	"time"
)

// ApiToken is a hashed bearer credential owned by a user, used to authorize
// API mutations alongside a session.
type ApiToken struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	Name       string     `json:"name"`
	TokenHash  string     `json:"-"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

// CreateApiToken persists token metadata (hash only) and returns its id.
func CreateApiToken(userID int64, name, tokenHash string) (int64, error) {
	res, err := DB.Exec(
		"INSERT INTO api_tokens (user_id, name, token_hash) VALUES (?, ?, ?)",
		userID, name, tokenHash,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetApiTokenByHash looks up an active (non-revoked, non-expired) token by hash.
func GetApiTokenByHash(hash string) (*ApiToken, error) {
	t := &ApiToken{}
	var lastUsed, expires, revoked sql.NullString
	err := DB.QueryRow(
		`SELECT id, user_id, name, token_hash, created_at, last_used_at, expires_at, revoked_at
		 FROM api_tokens WHERE token_hash = ?`, hash,
	).Scan(&t.ID, &t.UserID, &t.Name, &t.TokenHash, &t.CreatedAt, &lastUsed, &expires, &revoked)
	if err != nil {
		return nil, err
	}
	t.LastUsedAt = parseNullTime(lastUsed)
	t.ExpiresAt = parseNullTime(expires)
	t.RevokedAt = parseNullTime(revoked)
	if t.RevokedAt != nil {
		return nil, sql.ErrNoRows
	}
	if t.ExpiresAt != nil && t.ExpiresAt.Before(time.Now()) {
		return nil, sql.ErrNoRows
	}
	return t, nil
}

// ListApiTokens returns all tokens owned by a user (metadata only).
func ListApiTokens(userID int64) ([]ApiToken, error) {
	rows, err := DB.Query(
		`SELECT id, user_id, name, token_hash, created_at, last_used_at, expires_at, revoked_at
		 FROM api_tokens WHERE user_id = ? ORDER BY id DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tokens := []ApiToken{}
	for rows.Next() {
		t := ApiToken{}
		var lastUsed, expires, revoked sql.NullString
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.TokenHash, &t.CreatedAt, &lastUsed, &expires, &revoked); err != nil {
			return nil, err
		}
		t.LastUsedAt = parseNullTime(lastUsed)
		t.ExpiresAt = parseNullTime(expires)
		t.RevokedAt = parseNullTime(revoked)
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// RevokeApiToken marks a token revoked; returns false if not found or not owned by user.
func RevokeApiToken(id, userID int64) bool {
	res, err := DB.Exec(
		"UPDATE api_tokens SET revoked_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ? AND revoked_at IS NULL",
		id, userID,
	)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}

// TouchApiToken updates last_used_at for a successfully used token.
func TouchApiToken(id int64) {
	DB.Exec("UPDATE api_tokens SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?", id)
}

// parseNullTime converts a SQLite datetime string into a *time.Time.
func parseNullTime(s sql.NullString) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	if t, err := parseTime(s.String); err == nil {
		return &t
	}
	return nil
}
