package db

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
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
	CreatedAt   time.Time `json:"created_at"`
}

type Session struct {
	ID        int64     `json:"id"`
	Token     string    `json:"token"`
	UserID    int64     `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
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
			password_hash TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			description TEXT DEFAULT '',
			media_type TEXT NOT NULL,
			filename TEXT NOT NULL,
			thumbnail TEXT DEFAULT '',
			locked INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token TEXT UNIQUE NOT NULL,
			user_id INTEGER NOT NULL,
			expires_at DATETIME NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);
	`)
	return err
}

func SeedAdmin(username, password string) error {
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	hash, err := hashPassword(password)
	if err != nil {
		return err
	}

	_, err = DB.Exec("INSERT INTO users (username, password_hash) VALUES (?, ?)", username, hash)
	return err
}

func GetUserByUsername(username string) (int64, string, error) {
	var id int64
	var hash string
	err := DB.QueryRow("SELECT id, password_hash FROM users WHERE username = ?", username).Scan(&id, &hash)
	if err != nil {
		return 0, "", err
	}
	return id, hash, nil
}

// Sessions

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

// Posts

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

func GetPosts() ([]Post, error) {
	rows, err := DB.Query(
		"SELECT id, title, description, media_type, filename, thumbnail, locked, created_at FROM posts ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		var lockedInt int
		var createdAt string
		err := rows.Scan(&p.ID, &p.Title, &p.Description, &p.MediaType, &p.Filename, &p.Thumbnail, &lockedInt, &createdAt)
		if err != nil {
			return nil, err
		}
		p.Locked = lockedInt == 1
		p.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		posts = append(posts, p)
	}
	return posts, nil
}

func GetPost(id int64) (*Post, error) {
	p := &Post{}
	var lockedInt int
	var createdAt string
	err := DB.QueryRow(
		"SELECT id, title, description, media_type, filename, thumbnail, locked, created_at FROM posts WHERE id = ?",
		id,
	).Scan(&p.ID, &p.Title, &p.Description, &p.MediaType, &p.Filename, &p.Thumbnail, &lockedInt, &createdAt)
	if err != nil {
		return nil, err
	}
	p.Locked = lockedInt == 1
	p.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return p, nil
}

func DeletePost(id int64) error {
	_, err := DB.Exec("DELETE FROM posts WHERE id = ?", id)
	return err
}

// Helpers

func generateToken(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func hashPassword(password string) (string, error) {
	// bcrypt-like approach: use SHA-256 + salt for simplicity (no CGO)
	salt := generateToken(16)
	hash := sha256Hash(salt + password)
	return salt + ":" + hash, nil
}

func CheckPassword(password, stored string) bool {
	if len(stored) < 49 { // salt(32) + ":" + hash(64) = 97 chars min
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
