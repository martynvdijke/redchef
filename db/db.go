package db

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
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
	Username     string    `json:"-"`
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

type EmailSettings struct {
	ID          int64     `json:"id"`
	SMTPHost    string    `json:"smtp_host"`
	SMTPPort    int       `json:"smtp_port"`
	Username    string    `json:"username"`
	Password    string    `json:"password"`
	FromAddr    string    `json:"from_addr"`
	Encryption  string    `json:"encryption"`
	GotifyURL   string    `json:"gotify_url"`
	GotifyToken string    `json:"gotify_token"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Comment struct {
	ID        int64     `json:"id"`
	PostID    int64     `json:"post_id"`
	UserID    int64     `json:"user_id"`
	Username  string    `json:"username"`
	ParentID  *int64    `json:"parent_id,omitempty"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type Favourite struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	PostID    int64     `json:"post_id"`
	CreatedAt time.Time `json:"created_at"`
}

type Tip struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	PostID      int64     `json:"post_id"`
	AmountCents int       `json:"amount_cents"`
	CreatedAt   time.Time `json:"created_at"`
}

type PostLink struct {
	ID           int64  `json:"id"`
	PostID       int64  `json:"post_id"`
	LinkedPostID int64  `json:"linked_post_id"`
	LinkedTitle  string `json:"linked_title,omitempty"`
	OrderIndex   int    `json:"order_index"`
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

type PasswordReset struct {
	ID        int64
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
	Used      bool
	CreatedAt time.Time
}

var DB *sql.DB

func Init(dbPath string) error {
	var err error
	// Use DSN with pragmas so they apply to every connection from the pool
	dsn := dbPath + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	DB, err = sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}

	// SQLite is not designed for concurrent writes from multiple connections.
	// Limit the pool to 1 to prevent WAL visibility races across connections.
	DB.SetMaxOpenConns(1)

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

		CREATE TABLE IF NOT EXISTS comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			post_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			parent_id INTEGER,
			body TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (post_id) REFERENCES posts(id),
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (parent_id) REFERENCES comments(id)
		);

		CREATE TABLE IF NOT EXISTS favourites (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			post_id INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, post_id),
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (post_id) REFERENCES posts(id)
		);

		CREATE TABLE IF NOT EXISTS tips (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			post_id INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (post_id) REFERENCES posts(id)
		);

		CREATE TABLE IF NOT EXISTS post_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			post_id INTEGER NOT NULL,
			linked_post_id INTEGER NOT NULL,
			order_index INTEGER DEFAULT 0,
			FOREIGN KEY (post_id) REFERENCES posts(id),
			FOREIGN KEY (linked_post_id) REFERENCES posts(id)
		);

		CREATE TABLE IF NOT EXISTS purchases (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			post_id INTEGER NOT NULL,
			price_cents INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, post_id),
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (post_id) REFERENCES posts(id)
		);

		CREATE TABLE IF NOT EXISTS email_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			smtp_host TEXT NOT NULL DEFAULT '',
			smtp_port INTEGER NOT NULL DEFAULT 587,
			username TEXT NOT NULL DEFAULT '',
			password TEXT NOT NULL DEFAULT '',
			from_addr TEXT NOT NULL DEFAULT '',
			encryption TEXT NOT NULL DEFAULT 'tls',
			gotify_url TEXT NOT NULL DEFAULT '',
			gotify_token TEXT NOT NULL DEFAULT '',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS password_resets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			token_hash TEXT UNIQUE NOT NULL,
			expires_at DATETIME NOT NULL,
			used INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);
	`)

	// Migrate existing tables: add columns that may be missing on upgraded DBs
	migrateAddColumn("users", "email", "TEXT DEFAULT ''")
	migrateAddColumn("users", "role", "TEXT DEFAULT 'normal'")
	migrateAddColumn("users", "paid", "INTEGER DEFAULT 0")
	migrateAddColumn("users", "created_at", "DATETIME DEFAULT CURRENT_TIMESTAMP")
	migrateAddColumn("tips", "amount_cents", "INTEGER NOT NULL DEFAULT 0")
	migrateAddColumn("email_settings", "gotify_url", "TEXT NOT NULL DEFAULT ''")
	migrateAddColumn("email_settings", "gotify_token", "TEXT NOT NULL DEFAULT ''")

	// Seed default email settings row if table is empty
	var emailCount int
	DB.QueryRow("SELECT COUNT(*) FROM email_settings").Scan(&emailCount)
	if emailCount == 0 {
		DB.Exec("INSERT INTO email_settings (smtp_host, smtp_port, username, password, from_addr, encryption, gotify_url, gotify_token) VALUES ('', 587, '', '', '', 'tls', '', '')")
	}

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
		"SELECT id, username, email, password_hash, role, paid, created_at FROM users WHERE id = ?",
		id,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &paidInt, &createdAt)
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

func UpdateUserEmail(id int64, email string) error {
	_, err := DB.Exec("UPDATE users SET email = ? WHERE id = ?", email, id)
	return err
}

func UpdateUserRole(id int64, role string) error {
	res, err := DB.Exec("UPDATE users SET role = ? WHERE id = ?", role, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
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

// ── Password Resets ──

// GenerateResetToken returns a fresh raw token plus its SHA-256 hash.
// Only the hash is ever stored, so a DB leak doesn't expose usable tokens.
func GenerateResetToken() (raw, tokenHash string) {
	raw = generateToken(32)
	return raw, sha256Hash(raw)
}

// HashToken hashes a raw reset token for lookup against stored hashes.
func HashToken(token string) string {
	return sha256Hash(token)
}

// CreatePasswordReset stores a reset token hash for a user, discarding any
// previous resets for that user as well as all globally expired resets.
func CreatePasswordReset(userID int64, tokenHash string, expiresAt time.Time) error {
	DB.Exec("DELETE FROM password_resets WHERE user_id = ? OR expires_at < datetime('now')", userID)
	_, err := DB.Exec(
		"INSERT INTO password_resets (user_id, token_hash, expires_at) VALUES (?, ?, ?)",
		userID, tokenHash, expiresAt,
	)
	return err
}

// GetPasswordReset returns a valid (unused, unexpired) reset for a token hash.
// Expired or already-used resets are deleted on sight.
func GetPasswordReset(tokenHash string) (*PasswordReset, error) {
	p := &PasswordReset{}
	var usedInt int
	var expiresAt, createdAt string
	err := DB.QueryRow(
		"SELECT id, user_id, token_hash, expires_at, used, created_at FROM password_resets WHERE token_hash = ?",
		tokenHash,
	).Scan(&p.ID, &p.UserID, &p.TokenHash, &expiresAt, &usedInt, &createdAt)
	if err != nil {
		return nil, err
	}
	p.Used = usedInt == 1
	p.ExpiresAt, _ = parseTime(expiresAt)
	p.CreatedAt, _ = parseTime(createdAt)
	if p.Used || time.Now().After(p.ExpiresAt) {
		DB.Exec("DELETE FROM password_resets WHERE id = ?", p.ID)
		return nil, sql.ErrNoRows
	}
	return p, nil
}

// ConsumePasswordReset marks a reset as used so it can't be replayed.
func ConsumePasswordReset(tokenHash string) error {
	_, err := DB.Exec("UPDATE password_resets SET used = 1 WHERE token_hash = ?", tokenHash)
	return err
}

// UpdateUserPassword hashes and stores a new password for a user.
func UpdateUserPassword(userID int64, password string) error {
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	_, err = DB.Exec("UPDATE users SET password_hash = ? WHERE id = ?", hash, userID)
	return err
}

// DeleteUserSessionsForUser invalidates every session a user holds.
func DeleteUserSessionsForUser(userID int64) error {
	_, err := DB.Exec("DELETE FROM user_sessions WHERE user_id = ?", userID)
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

	orderBy := "ORDER BY created_at DESC, id DESC"
	if filter != nil && filter.Sort == "oldest" {
		orderBy = "ORDER BY created_at ASC, id ASC"
	}

	rows, err := DB.Query(query+" "+orderBy, args...)
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

// UpdatePostDetails updates the title and description of a post.
func UpdatePostDetails(id int64, title, description string) (*Post, error) {
	_, err := DB.Exec("UPDATE posts SET title = ?, description = ? WHERE id = ?", title, description, id)
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

// ── Migration Helpers ──

// migrateAddColumn adds a column to a table if it doesn't already exist.
// SQLite < 3.36 doesn't support IF NOT EXISTS for ALTER TABLE, so we
// ignore the "duplicate column name" error.
func migrateAddColumn(table, column, colType string) {
	_, err := DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, colType))
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		log.Printf("[db] migrate add column %s.%s: %v", table, column, err)
	}
}

// ── Email Settings ──

func GetEmailSettings() (*EmailSettings, error) {
	s := &EmailSettings{}
	var updatedAt string
	err := DB.QueryRow(
		"SELECT id, smtp_host, smtp_port, username, password, from_addr, encryption, gotify_url, gotify_token, updated_at FROM email_settings WHERE id = 1",
	).Scan(&s.ID, &s.SMTPHost, &s.SMTPPort, &s.Username, &s.Password, &s.FromAddr, &s.Encryption, &s.GotifyURL, &s.GotifyToken, &updatedAt)
	if err != nil {
		return nil, err
	}
	s.UpdatedAt, _ = parseTime(updatedAt)
	return s, nil
}

func UpdateEmailSettings(host string, port int, username, password, fromAddr, encryption, gotifyURL, gotifyToken string) error {
	_, err := DB.Exec(
		"UPDATE email_settings SET smtp_host = ?, smtp_port = ?, username = ?, password = ?, from_addr = ?, encryption = ?, gotify_url = ?, gotify_token = ?, updated_at = datetime('now') WHERE id = 1",
		host, port, username, password, fromAddr, encryption, gotifyURL, gotifyToken,
	)
	return err
}

// ── Comments ──

func CreateComment(postID, userID int64, parentID *int64, body string) (*Comment, error) {
	res, err := DB.Exec(
		"INSERT INTO comments (post_id, user_id, parent_id, body) VALUES (?, ?, ?, ?)",
		postID, userID, parentID, body,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return GetComment(id)
}

func GetComment(id int64) (*Comment, error) {
	c := &Comment{}
	var parentID sql.NullInt64
	var createdAt string
	err := DB.QueryRow(`
		SELECT c.id, c.post_id, c.user_id, u.email, c.parent_id, c.body, c.created_at
		FROM comments c JOIN users u ON c.user_id = u.id
		WHERE c.id = ?`, id,
	).Scan(&c.ID, &c.PostID, &c.UserID, &c.Username, &parentID, &c.Body, &createdAt)
	if err != nil {
		return nil, err
	}
	if parentID.Valid {
		c.ParentID = &parentID.Int64
	}
	c.CreatedAt, _ = parseTime(createdAt)
	return c, nil
}

func GetCommentsByPost(postID int64) ([]Comment, error) {
	rows, err := DB.Query(`
		SELECT c.id, c.post_id, c.user_id, u.email, c.parent_id, c.body, c.created_at
		FROM comments c JOIN users u ON c.user_id = u.id
		WHERE c.post_id = ? ORDER BY c.created_at ASC`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		var parentID sql.NullInt64
		var createdAt string
		if err := rows.Scan(&c.ID, &c.PostID, &c.UserID, &c.Username, &parentID, &c.Body, &createdAt); err != nil {
			return nil, err
		}
		if parentID.Valid {
			c.ParentID = &parentID.Int64
		}
		c.CreatedAt, _ = parseTime(createdAt)
		comments = append(comments, c)
	}
	return comments, nil
}

func DeleteComment(id int64) error {
	_, err := DB.Exec("DELETE FROM comments WHERE id = ?", id)
	return err
}

// AdminComment is a comment enriched with its post title for admin listing.
type AdminComment struct {
	Comment
	PostTitle string `json:"post_title"`
}

// GetAllComments returns every comment across all posts, newest first.
func GetAllComments() ([]AdminComment, error) {
	rows, err := DB.Query(`
		SELECT c.id, c.post_id, c.user_id, u.email, c.parent_id, c.body, c.created_at, p.title
		FROM comments c
		JOIN users u ON c.user_id = u.id
		JOIN posts p ON c.post_id = p.id
		ORDER BY c.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []AdminComment
	for rows.Next() {
		var c AdminComment
		var parentID sql.NullInt64
		var createdAt string
		if err := rows.Scan(&c.ID, &c.PostID, &c.UserID, &c.Username, &parentID, &c.Body, &createdAt, &c.PostTitle); err != nil {
			return nil, err
		}
		if parentID.Valid {
			c.ParentID = &parentID.Int64
		}
		c.CreatedAt, _ = parseTime(createdAt)
		comments = append(comments, c)
	}
	return comments, nil
}

// ── Favourites ──

func GetUserFavourited(userID, postID int64) (bool, error) {
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM favourites WHERE user_id = ? AND post_id = ?", userID, postID).Scan(&count)
	return count > 0, err
}

func GetFavouriteCount(postID int64) (int, error) {
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM favourites WHERE post_id = ?", postID).Scan(&count)
	return count, err
}

func ToggleFavourite(userID, postID int64) (bool, error) {
	// Check if already favourited
	var id int64
	err := DB.QueryRow("SELECT id FROM favourites WHERE user_id = ? AND post_id = ?", userID, postID).Scan(&id)
	if err == nil {
		// Already favourited — remove
		_, err = DB.Exec("DELETE FROM favourites WHERE id = ?", id)
		return false, err
	}
	// Not favourited — add
	_, err = DB.Exec("INSERT INTO favourites (user_id, post_id) VALUES (?, ?)", userID, postID)
	return true, err
}

func ListFavourites(userID int64) ([]Post, error) {
	rows, err := DB.Query(`
		SELECT p.id, p.title, p.description, p.media_type, p.filename, p.thumbnail, p.locked, p.processing, p.created_at
		FROM posts p JOIN favourites f ON p.id = f.post_id
		WHERE f.user_id = ? ORDER BY f.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		var lockedInt, processingInt int
		var createdAt string
		if err := rows.Scan(&p.ID, &p.Title, &p.Description, &p.MediaType, &p.Filename, &p.Thumbnail, &lockedInt, &processingInt, &createdAt); err != nil {
			return nil, err
		}
		p.Locked = lockedInt == 1
		p.Processing = processingInt == 1
		p.CreatedAt, _ = parseTime(createdAt)
		posts = append(posts, p)
	}
	return posts, nil
}

// ── Tips ──

func GetTipCount(postID int64) (int, error) {
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM tips WHERE post_id = ?", postID).Scan(&count)
	return count, err
}

func HasUserTipped(userID, postID int64) (bool, error) {
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM tips WHERE user_id = ? AND post_id = ?", userID, postID).Scan(&count)
	return count > 0, err
}

func CreateTip(userID, postID int64, amountCents int) error {
	_, err := DB.Exec("INSERT INTO tips (user_id, post_id, amount_cents) VALUES (?, ?, ?)", userID, postID, amountCents)
	return err
}

func GetTotalTipAmount(postID int64) (int, error) {
	var total int
	err := DB.QueryRow("SELECT COALESCE(SUM(amount_cents), 0) FROM tips WHERE post_id = ?", postID).Scan(&total)
	return total, err
}

// ── Purchases (per-item mock payments) ──

// CreatePurchase records a per-item unlock. Duplicate purchases are ignored.
func CreatePurchase(userID, postID int64, priceCents int) error {
	_, err := DB.Exec(
		"INSERT OR IGNORE INTO purchases (user_id, post_id, price_cents) VALUES (?, ?, ?)",
		userID, postID, priceCents,
	)
	return err
}

func HasUserPurchased(userID, postID int64) (bool, error) {
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM purchases WHERE user_id = ? AND post_id = ?", userID, postID).Scan(&count)
	return count > 0, err
}

// ── Linked Posts ──

func SetPostLinks(postID int64, linkedIDs []int64) error {
	// Remove existing links for this post
	if _, err := DB.Exec("DELETE FROM post_links WHERE post_id = ?", postID); err != nil {
		return err
	}

	for i, linkedID := range linkedIDs {
		_, err := DB.Exec(
			"INSERT INTO post_links (post_id, linked_post_id, order_index) VALUES (?, ?, ?)",
			postID, linkedID, i,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func GetPostLinks(postID int64) ([]PostLink, error) {
	rows, err := DB.Query(`
		SELECT pl.id, pl.post_id, pl.linked_post_id, p.title, pl.order_index
		FROM post_links pl JOIN posts p ON pl.linked_post_id = p.id
		WHERE pl.post_id = ? ORDER BY pl.order_index ASC`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []PostLink
	for rows.Next() {
		var l PostLink
		if err := rows.Scan(&l.ID, &l.PostID, &l.LinkedPostID, &l.LinkedTitle, &l.OrderIndex); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, nil
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
