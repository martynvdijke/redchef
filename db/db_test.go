package db

import (
	"os"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	err := Init("/tmp/redchef_test.db")
	if err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
}

func cleanupTestDB(t *testing.T) {
	t.Helper()
	if DB != nil {
		DB.Close()
		DB = nil
	}
	os.Remove("/tmp/redchef_test.db")
	os.Remove("/tmp/redchef_test.db-wal")
	os.Remove("/tmp/redchef_test.db-shm")
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"RFC3339", "2026-07-20T15:11:24Z", false},
		{"space format", "2026-07-20 15:11:24", false},
		{"RFC3339 no Z", "2026-07-20T15:11:24", false},
		{"with trailing space", "  2026-07-20T15:11:24Z  ", false},
		{"empty string", "", true},
		{"garbage", "not a date", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTime(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseTime(%q) expected error, got %v", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("parseTime(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got.IsZero() {
				t.Errorf("parseTime(%q) returned zero time", tt.input)
			}
		})
	}
}

func TestCreateAndGetPost(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	// Create a post
	post, err := CreatePost("Test Title", "Test Description", "photo", "test.jpg", "test.jpg", true)
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}

	if post.Title != "Test Title" {
		t.Errorf("expected title 'Test Title', got %q", post.Title)
	}
	if post.Description != "Test Description" {
		t.Errorf("expected description 'Test Description', got %q", post.Description)
	}
	if post.MediaType != "photo" {
		t.Errorf("expected media_type 'photo', got %q", post.MediaType)
	}
	if post.Filename != "test.jpg" {
		t.Errorf("expected filename 'test.jpg', got %q", post.Filename)
	}
	if post.Thumbnail != "test.jpg" {
		t.Errorf("expected thumbnail 'test.jpg', got %q", post.Thumbnail)
	}
	if !post.Locked {
		t.Error("expected locked=true")
	}
	if post.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}
	if post.ID == 0 {
		t.Error("expected non-zero post ID")
	}
}

func TestCreatePostLocked(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	// Test with locked=false
	post, err := CreatePost("Unlocked", "", "photo", "unlocked.jpg", "unlocked.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}
	if post.Locked {
		t.Error("expected locked=false")
	}

	// Test with locked=true
	post, err = CreatePost("Locked", "", "photo", "locked.jpg", "locked.jpg", true)
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}
	if !post.Locked {
		t.Error("expected locked=true")
	}
}

func TestGetPosts(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	// Create multiple posts
	_, err := CreatePost("First", "desc", "photo", "first.jpg", "first.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}
	_, err = CreatePost("Second", "desc", "video", "second.mp4", "second.mp4", true)
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}

	posts, err := GetPosts()
	if err != nil {
		t.Fatalf("GetPosts failed: %v", err)
	}

	if len(posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(posts))
	}

	// Verify both posts are present
	titles := map[string]bool{}
	for _, p := range posts {
		titles[p.Title] = true
	}
	if !titles["First"] {
		t.Error("missing post 'First'")
	}
	if !titles["Second"] {
		t.Error("missing post 'Second'")
	}
}

func TestDeletePost(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	post, err := CreatePost("Delete Me", "", "photo", "delete.jpg", "delete.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}

	err = DeletePost(post.ID)
	if err != nil {
		t.Fatalf("DeletePost failed: %v", err)
	}

	// Verify it's gone
	_, err = GetPost(post.ID)
	if err == nil {
		t.Error("expected error when getting deleted post")
	}
}

func TestHasUsers(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	has, err := HasUsers()
	if err != nil {
		t.Fatalf("HasUsers failed: %v", err)
	}
	if has {
		t.Error("expected no users initially")
	}

	err = CreateUser("admin", "password123")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	has, err = HasUsers()
	if err != nil {
		t.Fatalf("HasUsers failed: %v", err)
	}
	if !has {
		t.Error("expected users to exist")
	}
}

func TestUserPassword(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	err := CreateUser("chef", "s3cret!")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	_, hash, err := GetUserByUsername("chef")
	if err != nil {
		t.Fatalf("GetUserByUsername failed: %v", err)
	}

	if !CheckPassword("s3cret!", hash) {
		t.Error("CheckPassword returned false for correct password")
	}

	if CheckPassword("wrong", hash) {
		t.Error("CheckPassword returned true for wrong password")
	}
}

func TestAnalyticsSettings(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	// Should have default settings from migration
	settings, err := GetAnalyticsSettings()
	if err != nil {
		t.Fatalf("GetAnalyticsSettings failed: %v", err)
	}

	if settings.UmamiScriptURL != "" {
		t.Errorf("expected empty script URL, got %q", settings.UmamiScriptURL)
	}

	// Update
	err = UpdateAnalyticsSettings("https://analytics.example.com/script.js", "test-id", true)
	if err != nil {
		t.Fatalf("UpdateAnalyticsSettings failed: %v", err)
	}

	settings, err = GetAnalyticsSettings()
	if err != nil {
		t.Fatalf("GetAnalyticsSettings failed: %v", err)
	}

	if settings.UmamiScriptURL != "https://analytics.example.com/script.js" {
		t.Errorf("expected script URL, got %q", settings.UmamiScriptURL)
	}
	if !settings.TrackingEnabled {
		t.Error("expected tracking enabled")
	}
}

func TestDatePersistence(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	post, err := CreatePost("Date Test", "", "photo", "date.jpg", "date.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}

	if post.CreatedAt.IsZero() {
		t.Fatal("created_at is zero time — date parsing broken")
	}

	// Verify it persists through a read
	got, err := GetPost(post.ID)
	if err != nil {
		t.Fatalf("GetPost failed: %v", err)
	}

	if got.CreatedAt.IsZero() {
		t.Fatal("created_at is zero time after re-read — date parsing broken")
	}

	// Verify it's a reasonable time (within 5 minutes of now)
	if diff := time.Since(got.CreatedAt); diff < 0 || diff > 5*time.Minute {
		t.Errorf("created_at outside expected range: %v (diff: %v)", got.CreatedAt, diff)
	}
}

func TestCreatePostEmptyDescription(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	post, err := CreatePost("No Desc", "", "video", "no-desc.mp4", "no-desc.mp4", false)
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}
	if post.Description != "" {
		t.Errorf("expected empty description, got %q", post.Description)
	}
}
