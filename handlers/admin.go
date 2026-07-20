package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"redchef/db"
)

type AnalyticsSettingsRequest struct {
	UmamiScriptURL  string `json:"umami_script_url"`
	UmamiWebsiteID  string `json:"umami_website_id"`
	TrackingEnabled bool   `json:"tracking_enabled"`
}

var uploadDir string

func init() {
	uploadDir = os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "uploads"
	}
	os.MkdirAll(uploadDir, 0755)
}

func AdminListPosts(w http.ResponseWriter, r *http.Request) {
	posts, err := db.GetPosts()
	if err != nil {
		http.Error(w, `{"error":"failed to list posts"}`, http.StatusInternalServerError)
		return
	}
	if posts == nil {
		posts = []db.Post{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(posts)
}

func AdminUpload(w http.ResponseWriter, r *http.Request) {
	// Limit upload size to 50MB
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, `{"error":"file too large or invalid form"}`, http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"missing file"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	title := r.FormValue("title")
	description := r.FormValue("description")
	lockedStr := r.FormValue("locked")
	locked := lockedStr != "false"

	if title == "" {
		http.Error(w, `{"error":"title is required"}`, http.StatusBadRequest)
		return
	}

	// Determine media type from extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	var mediaType string
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		mediaType = "photo"
	case ".mp4", ".webm", ".mov":
		mediaType = "video"
	default:
		http.Error(w, `{"error":"unsupported file type: `+ext+`"}`, http.StatusBadRequest)
		return
	}

	// Generate unique filename
	filename := fmt.Sprintf("%d%s", generateID(), ext)
	dst, err := os.Create(filepath.Join(uploadDir, filename))
	if err != nil {
		http.Error(w, `{"error":"failed to save file"}`, http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, `{"error":"failed to write file"}`, http.StatusInternalServerError)
		return
	}

	// Generate thumbnail path (same file for now, frontend can use it directly)
	thumbnail := filename

	post, err := db.CreatePost(title, description, mediaType, filename, thumbnail, locked)
	if err != nil {
		http.Error(w, `{"error":"failed to create post"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(post)
}

func AdminDeletePost(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	// Get post to find filename
	post, err := db.GetPost(id)
	if err != nil {
		http.Error(w, `{"error":"post not found"}`, http.StatusNotFound)
		return
	}

	// Delete file from disk
	filePath := filepath.Join(uploadDir, post.Filename)
	os.Remove(filePath)
	if post.Thumbnail != "" && post.Thumbnail != post.Filename {
		os.Remove(filepath.Join(uploadDir, post.Thumbnail))
	}

	if err := db.DeletePost(id); err != nil {
		http.Error(w, `{"error":"failed to delete post"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"ok":true}`))
}

func AdminGetAnalyticsSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := db.GetAnalyticsSettings()
	if err != nil {
		http.Error(w, `{"error":"failed to get settings"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

func AdminUpdateAnalyticsSettings(w http.ResponseWriter, r *http.Request) {
	var req AnalyticsSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.UmamiScriptURL != "" {
		if !strings.HasPrefix(req.UmamiScriptURL, "https://") && !strings.HasPrefix(req.UmamiScriptURL, "http://") {
			http.Error(w, `{"error":"script URL must start with http:// or https://"}`, http.StatusBadRequest)
			return
		}
	}

	if err := db.UpdateAnalyticsSettings(req.UmamiScriptURL, req.UmamiWebsiteID, req.TrackingEnabled); err != nil {
		http.Error(w, `{"error":"failed to save settings"}`, http.StatusInternalServerError)
		return
	}

	settings, _ := db.GetAnalyticsSettings()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

// ── Setup (first-run admin creation) ──

func Setup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username        string `json:"username"`
		Password        string `json:"password"`
		ConfirmPassword string `json:"confirm_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Check if admin already exists
	hasUsers, err := db.HasUsers()
	if err != nil {
		http.Error(w, `{"error":"failed to check setup status"}`, http.StatusInternalServerError)
		return
	}
	if hasUsers {
		http.Error(w, `{"error":"admin already configured"}`, http.StatusForbidden)
		return
	}

	// Validate
	if req.Username == "" {
		http.Error(w, `{"error":"username is required"}`, http.StatusBadRequest)
		return
	}
	if req.Password == "" {
		http.Error(w, `{"error":"password is required"}`, http.StatusBadRequest)
		return
	}
	if len(req.Password) < 4 {
		http.Error(w, `{"error":"password must be at least 4 characters"}`, http.StatusBadRequest)
		return
	}
	if req.Password != req.ConfirmPassword {
		http.Error(w, `{"error":"passwords do not match"}`, http.StatusBadRequest)
		return
	}

	// Create user
	if err := db.CreateUser(req.Username, req.Password); err != nil {
		http.Error(w, `{"error":"failed to create admin user"}`, http.StatusInternalServerError)
		return
	}

	// Auto-login: get user ID
	userID, _, err := db.GetUserByUsername(req.Username)
	if err != nil {
		http.Error(w, `{"error":"failed to verify admin user"}`, http.StatusInternalServerError)
		return
	}

	// Create session
	session, err := db.CreateSession(userID)
	if err != nil {
		http.Error(w, `{"error":"failed to create session"}`, http.StatusInternalServerError)
		return
	}

	// Set session cookie (same name used by Login handler)
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_token",
		Value:    session.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"token":    session.Token,
		"redirect": "/admin.html",
	})
}

// ── Setup status check ──

func SetupStatus(w http.ResponseWriter, r *http.Request) {
	hasUsers, err := db.HasUsers()
	if err != nil {
		http.Error(w, `{"error":"failed to check"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{
		"needs_setup": !hasUsers,
	})
}

// Simple ID generator (not crypto, just for filenames)
var idCounter int64

func generateID() int64 {
	idCounter++
	return idCounter + int64(os.Getpid())*1000000
}
