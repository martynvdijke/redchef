package db

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Post struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	MediaType   string    `json:"media_type"`
	Filename    string    `json:"filename"`
	Thumbnail   string    `json:"thumbnail"`
	Locked      bool      `json:"locked"`
	Processing  bool      `json:"processing"`
	CreatedAt   time.Time `json:"created_at"`
}

type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	Paid         bool      `json:"paid"`
	CreatedAt    time.Time `json:"created_at"`
}

type AnalyticsSettings struct {
	ID              int64     `json:"id"`
	UmamiScriptURL  string    `json:"umami_script_url"`
	UmamiWebsiteID  string    `json:"umami_website_id"`
	TrackingEnabled bool      `json:"tracking_enabled"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Session struct {
	ID        int64     `json:"id"`
	Token     string    `json:"token"`
	UserID    int64     `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

type UserSession struct {
	ID        int64     `json:"id"`
	Token     string    `json:"token"`
	UserID    int64     `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

var DB *sql.DB

func Init(dbPath string) error {
	var err error
	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}

	// Enable WAL mode for better concurrent access
	if _, err := DB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("enable wal: %w", err)
	}

	// Set busy timeout so writes wait instead of immediately failing with SQLITE_BUSY
	if _, err := DB.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return fmt.Errorf("set busy timeout: %w", err)
	}

	if err := migrate(); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	return nil
}

func migrate() error {
	_, err := DB.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			email TEXT DEFAULT '',
			role TEXT DEFAULT 'normal',
			paid INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			description TEXT DEFAULT '',
			media_type TEXT NOT NULL,
			filename TEXT NOT NULL,
			thumbnail TEXT DEFAULT '',
			locked INTEGER DEFAULT 1,
			processing INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token TEXT UNIQUE NOT NULL,
			user_id INTEGER NOT NULL,
			expires_at DATETIME NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);

		CREATE TABLE IF NOT EXISTS user_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token TEXT UNIQUE NOT NULL,
			user_id INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);

		CREATE TABLE IF NOT EXISTS analytics_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			umami_script_url TEXT NOT NULL DEFAULT '',
			umami_website_id TEXT NOT NULL DEFAULT '',
			tracking_enabled INTEGER NOT NULL DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)

	// Seed default analytics settings row if table is empty
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM analytics_settings").Scan(&count)
	if count == 0 {
		_, err = DB.Exec("INSERT INTO analytics_settings (umami_script_url, umami_website_id, tracking_enabled) VALUES ('', '', 0)")
		if err != nil {
			return err
		}
	}

	// Migrate existing users: set first user as admin if no roles set
	var adminCount int
	DB.QueryRow("SELECT COUNT(*) FROM users WHERE role != 'normal'").Scan(&adminCount)
	if adminCount == 0 {
		// Get first user, set as admin + paid
		DB.Exec("UPDATE users SET role = 'admin', paid = 1 WHERE id = (SELECT id FROM users ORDER BY id ASC LIMIT 1)")
		// Set email from username for all users missing email
		DB.Exec("UPDATE users SET email = username || '@local' WHERE email = '' OR email IS NULL")
	}

	return err
}

// ── User Queries ──

func HasUsers() (bool, error) {
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func CreateUser(email, password string) (int64, error) {
	hash, err := hashPassword(password)
	if err != nil {
		return 0, err
	}

	// Determine role: first user = admin, rest = normal
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	role := "normal"
	paid := 0
	if count == 0 {
		role = "admin"
		paid = 1
	}

	res, err := DB.Exec(
		"INSERT INTO users (username, password_hash, email, role, paid) VALUES (?, ?, ?, ?, ?)",
		email, hash, email, role, paid,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func GetUserByEmail(email string) (*User, error) {
	u := &User{}
	var paidInt int
	var createdAt string
	err := DB.QueryRow(
		"SELECT id, email, password_hash, role, paid, created_at FROM users WHERE email = ?",
		email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &paidInt, &createdAt)
	if err != nil {
		return nil, err
	}
	u.Paid = paidInt == 1
	u.CreatedAt, _ = parseTime(createdAt)
	return u, nil
}

func GetUserByUsername(username string) (*User, error) {
	u := &User{}
	var paidInt int
	var createdAt string
	err := DB.QueryRow(
		"SELECT id, email, password_hash, role, paid, created_at FROM users WHERE username = ?",
		username,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &paidInt, &createdAt)
	if err != nil {
		return nil, err
	}
	u.Paid = paidInt == 1
	u.CreatedAt, _ = parseTime(createdAt)
	return u, nil
}

func GetUserByID(id int64) (*User, error) {
	u := &User{}
	var paidInt int
	var createdAt string
	err := DB.QueryRow(
		"SELECT id, email, password_hash, role, paid, created_at FROM users WHERE id = ?",
		id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &paidInt, &createdAt)
	if err != nil {
		return nil, err
	}
	u.Paid = paidInt == 1
	u.CreatedAt, _ = parseTime(createdAt)
	return u, nil
}

func ListUsers() ([]User, error) {
	rows, err := DB.Query("SELECT id, email, role, paid, created_at FROM users ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var paidInt int
		var createdAt string
		err := rows.Scan(&u.ID, &u.Email, &u.Role, &paidInt, &createdAt)
		if err != nil {
			return nil, err
		}
		u.Paid = paidInt == 1
		u.CreatedAt, _ = parseTime(createdAt)
		users = append(users, u)
	}
	return users, nil
}

func UpdateUserPaid(id int64, paid bool) error {
	paidInt := 0
	if paid {
		paidInt = 1
	}
	_, err := DB.Exec("UPDATE users SET paid = ? WHERE id = ?", paidInt, id)
	return err
}

func DeleteUser(id int64) error {
	// Prevent deleting the last admin
	var adminCount int
	DB.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin' AND id != ?", id).Scan(&adminCount)
	if adminCount == 0 {
		return fmt.Errorf("cannot delete the last admin user")
	}
	_, err := DB.Exec("DELETE FROM user_sessions WHERE user_id = ?", id)
	if err != nil {
		return err
	}
	_, err = DB.Exec("DELETE FROM sessions WHERE user_id = ?", id)
	if err != nil {
		return err
	}
	_, err = DB.Exec("DELETE FROM users WHERE id = ?", id)
	return err
}

// ── User Sessions ──

func CreateUserSession(userID int64) (*UserSession, error) {
	token := generateToken(32)
	_, err := DB.Exec(
		"INSERT INTO user_sessions (token, user_id) VALUES (?, ?)",
		token, userID,
	)
	if err != nil {
		return nil, err
	}
	return &UserSession{
		Token:     token,
		UserID:    userID,
		CreatedAt: time.Now(),
	}, nil
}

func GetUserSession(token string) (*UserSession, error) {
	s := &UserSession{}
	var createdAt string
	err := DB.QueryRow(
		"SELECT id, token, user_id, created_at FROM user_sessions WHERE token = ?",
		token,
	).Scan(&s.ID, &s.Token, &s.UserID, &createdAt)
	if err != nil {
		return nil, err
	}
	s.CreatedAt, _ = parseTime(createdAt)
	// Sessions valid for 30 days
	if time.Since(s.CreatedAt) > 30*24*time.Hour {
		DB.Exec("DELETE FROM user_sessions WHERE id = ?", s.ID)
		return nil, sql.ErrNoRows
	}
	return s, nil
}

func DeleteUserSession(token string) error {
	_, err := DB.Exec("DELETE FROM user_sessions WHERE token = ?", token)
	return err
}

// ── Admin Sessions (legacy) ──

func CreateSession(userID int64) (*Session, error) {
	token := generateToken(32)
	expiresAt := time.Now().Add(24 * time.Hour)

	_, err := DB.Exec(
		"INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)",
		token, userID, expiresAt,
	)
	if err != nil {
		return nil, err
	}

	return &Session{
		Token:     token,
		UserID:    userID,
		ExpiresAt: expiresAt,
	}, nil
}

func GetSession(token string) (*Session, error) {
	s := &Session{}
	err := DB.QueryRow(
		"SELECT id, token, user_id, expires_at FROM sessions WHERE token = ? AND expires_at > datetime('now')",
		token,
	).Scan(&s.ID, &s.Token, &s.UserID, &s.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func DeleteSession(token string) error {
	_, err := DB.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
}

// ── Posts ──

type PostFilter struct {
	Sort     string
	Type     string
	DateFrom string
	DateTo   string
}

func CreatePost(title, description, mediaType, filename, thumbnail string, locked bool) (*Post, error) {
	lockedInt := 0
	if locked {
		lockedInt = 1
	}

	res, err := DB.Exec(
		"INSERT INTO posts (title, description, media_type, filename, thumbnail, locked) VALUES (?, ?, ?, ?, ?, ?)",
		title, description, mediaType, filename, thumbnail, lockedInt,
	)
	if err != nil {
		return nil, err
	}

	id, _ := res.LastInsertId()
	return GetPost(id)
}

func GetPosts(filter *PostFilter) ([]Post, error) {
	query := "SELECT id, title, description, media_type, filename, thumbnail, locked, processing, created_at FROM posts"
	var conditions []string
	var args []interface{}

	if filter != nil {
		if filter.Type == "photo" || filter.Type == "video" {
			conditions = append(conditions, "media_type = ?")
			args = append(args, filter.Type)
		}
		if filter.DateFrom != "" {
			conditions = append(conditions, "created_at >= ?")
			args = append(args, filter.DateFrom)
		}
		if filter.DateTo != "" {
			conditions = append(conditions, "created_at <= ?")
			args = append(args, filter.DateTo+" 23:59:59")
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	orderBy := "ORDER BY created_at DESC"
	if filter != nil && filter.Sort == "oldest" {
		orderBy = "ORDER BY created_at ASC"
	}

	rows, err := DB.Query(query + " " + orderBy)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		var lockedInt int
		var processingInt int
		var createdAt string
		err := rows.Scan(&p.ID, &p.Title, &p.Description, &p.MediaType, &p.Filename, &p.Thumbnail, &lockedInt, &processingInt, &createdAt)
		if err != nil {
			return nil, err
		}
		p.Locked = lockedInt == 1
		p.Processing = processingInt == 1
		p.CreatedAt, _ = parseTime(createdAt)
		posts = append(posts, p)
	}
	return posts, nil
}

func GetPost(id int64) (*Post, error) {
	p := &Post{}
	var lockedInt int
	var processingInt int
	var createdAt string
	err := DB.QueryRow(
		"SELECT id, title, description, media_type, filename, thumbnail, locked, processing, created_at FROM posts WHERE id = ?",
		id,
	).Scan(&p.ID, &p.Title, &p.Description, &p.MediaType, &p.Filename, &p.Thumbnail, &lockedInt, &processingInt, &createdAt)
	if err != nil {
		return nil, err
	}
	p.Locked = lockedInt == 1
	p.Processing = processingInt == 1
	p.CreatedAt, _ = parseTime(createdAt)
	return p, nil
}

func UpdatePostLock(id int64, locked bool) (*Post, error) {
	lockedInt := 0
	if locked {
		lockedInt = 1
	}
	_, err := DB.Exec("UPDATE posts SET locked = ? WHERE id = ?", lockedInt, id)
	if err != nil {
		return nil, err
	}
	return GetPost(id)
}

func UpdatePostProcessing(id int64, filename, thumbnail string) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := DB.Exec(
		"UPDATE posts SET filename = ?, thumbnail = ?, processing = 0 WHERE id = ?",
		filename, thumbnail, id,
	)
	return err
}

func DeletePost(id int64) error {
	_, err := DB.Exec("DELETE FROM posts WHERE id = ?", id)
	return err
}

// ── Analytics Settings ──

func GetAnalyticsSettings() (*AnalyticsSettings, error) {
	s := &AnalyticsSettings{}
	var enabledInt int
	var updatedAt string
	err := DB.QueryRow(
		"SELECT id, umami_script_url, umami_website_id, tracking_enabled, updated_at FROM analytics_settings WHERE id = 1",
	).Scan(&s.ID, &s.UmamiScriptURL, &s.UmamiWebsiteID, &enabledInt, &updatedAt)
	if err != nil {
		return nil, err
	}
	s.TrackingEnabled = enabledInt == 1
	s.UpdatedAt, _ = parseTime(updatedAt)
	return s, nil
}

func UpdateAnalyticsSettings(scriptURL, websiteID string, enabled bool) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	_, err := DB.Exec(
		"UPDATE analytics_settings SET umami_script_url = ?, umami_website_id = ?, tracking_enabled = ?, updated_at = datetime('now') WHERE id = 1",
		scriptURL, websiteID, enabledInt,
	)
	return err
}

// ── Helpers ──

func generateToken(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// parseTime tries multiple formats to parse timestamps from SQLite.
func parseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time: %q", s)
}

func hashPassword(password string) (string, error) {
	salt := generateToken(16)
	hash := sha256Hash(salt + password)
	return salt + ":" + hash, nil
}

func CheckPassword(password, stored string) bool {
	if len(stored) < 49 {
		return false
	}
	salt := stored[:32]
	hash := stored[33:]
	return sha256Hash(salt+password) == hash
}

func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
