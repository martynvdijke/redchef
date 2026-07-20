package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"redchef/db"
)

// ProcessWG tracks in-flight media processing goroutines for test cleanup.
var ProcessWG sync.WaitGroup

type AnalyticsSettingsRequest struct {
	UmamiScriptURL  string `json:"umami_script_url"`
	UmamiWebsiteID  string `json:"umami_website_id"`
	TrackingEnabled bool   `json:"tracking_enabled"`
}

type EmailSettingsRequest struct {
	SMTPHost    string `json:"smtp_host"`
	SMTPPort    int    `json:"smtp_port"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	FromAddr    string `json:"from_addr"`
	Encryption  string `json:"encryption"`
	GotifyURL   string `json:"gotify_url"`
	GotifyToken string `json:"gotify_token"`
}

var uploadDir string

func init() {
	uploadDir = os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "/app/media"
	}
	os.MkdirAll(uploadDir, 0755)
}

// ── Posts ──

func AdminListPosts(w http.ResponseWriter, r *http.Request) {
	posts, err := db.GetPosts(nil)
	if err != nil {
		jsonError(w, "failed to list posts", http.StatusInternalServerError)
		return
	}
	if posts == nil {
		posts = []db.Post{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(posts)
}

func AdminUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		jsonError(w, "file too large or invalid form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	title := r.FormValue("title")
	description := r.FormValue("description")
	lockedStr := r.FormValue("locked")
	locked := lockedStr == "true" || lockedStr == "on"

	if title == "" {
		title = time.Now().Format("January 2, 2006")
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	var mediaType string
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		mediaType = "photo"
	case ".mp4", ".webm", ".mov":
		mediaType = "video"
	default:
		jsonError(w, "unsupported file type: "+ext, http.StatusBadRequest)
		return
	}

	// Save original file temporarily
	filename := fmt.Sprintf("_raw_%d%s", generateID(), ext)
	dst, err := os.Create(filepath.Join(uploadDir, filename))
	if err != nil {
		jsonError(w, "failed to save file", http.StatusInternalServerError)
		return
	}
	io.Copy(dst, file)
	dst.Close()

	// Create post immediately with processing=true
	post, err := db.CreatePost(title, description, mediaType, filename, filename, locked)
	if err != nil {
		os.Remove(filepath.Join(uploadDir, filename))
		jsonError(w, "failed to create post", http.StatusInternalServerError)
		return
	}

	// Notify all users about the new post (async, best-effort)
	go SendNewPostNotification(*post, getUserID(r))

	// Process media asynchronously
	ProcessWG.Add(1)
	go func() {
		defer ProcessWG.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[media] Panic processing post %d: %v", post.ID, r)
			}
		}()
		processMedia(post.ID, filename, mediaType, ext)
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":         post.ID,
		"title":      post.Title,
		"media_type": post.MediaType,
		"locked":     post.Locked,
		"created_at": post.CreatedAt,
		"processing": true,
		"message":    "Upload accepted, processing...",
	})
}

type UpdatePostRequest struct {
	Locked      *bool   `json:"locked"`
	Title       *string `json:"title"`
	Description *string `json:"description"`
}

func AdminUpdatePost(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	var req UpdatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Locked == nil && req.Title == nil && req.Description == nil {
		jsonError(w, "at least one field (locked, title, description) is required", http.StatusBadRequest)
		return
	}

	// Verify post exists first
	if _, err := db.GetPost(id); err != nil {
		jsonError(w, "post not found", http.StatusNotFound)
		return
	}

	if req.Locked != nil {
		if _, err := db.UpdatePostLock(id, *req.Locked); err != nil {
			jsonError(w, "failed to update lock state", http.StatusInternalServerError)
			return
		}
	}

	if req.Title != nil || req.Description != nil {
		post, _ := db.GetPost(id)
		title := post.Title
		description := post.Description
		if req.Title != nil {
			title = strings.TrimSpace(*req.Title)
			if title == "" {
				jsonError(w, "title cannot be empty", http.StatusBadRequest)
				return
			}
		}
		if req.Description != nil {
			description = *req.Description
		}
		if _, err := db.UpdatePostDetails(id, title, description); err != nil {
			jsonError(w, "failed to update post", http.StatusInternalServerError)
			return
		}
	}

	post, err := db.GetPost(id)
	if err != nil {
		jsonError(w, "post not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(post)
}

func AdminDeletePost(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	post, err := db.GetPost(id)
	if err != nil {
		jsonError(w, "post not found", http.StatusNotFound)
		return
	}

	// Delete files from disk
	filePath := filepath.Join(uploadDir, post.Filename)
	os.Remove(filePath)
	if post.Thumbnail != "" && post.Thumbnail != post.Filename {
		os.Remove(filepath.Join(uploadDir, post.Thumbnail))
	}
	// Also remove raw file if exists
	os.Remove(filepath.Join(uploadDir, fmt.Sprintf("_raw_%d*", id)))

	if err := db.DeletePost(id); err != nil {
		jsonError(w, "failed to delete post", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"ok":true}`))
}

// ── Users (admin) ──

func AdminListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := db.ListUsers()
	if err != nil {
		jsonError(w, "failed to list users", http.StatusInternalServerError)
		return
	}
	if users == nil {
		users = []db.User{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

type UpdateUserRequest struct {
	Paid *bool `json:"paid"`
}

func AdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Paid == nil {
		jsonError(w, "paid field is required", http.StatusBadRequest)
		return
	}

	if err := db.UpdateUserPaid(id, *req.Paid); err != nil {
		jsonError(w, "failed to update user", http.StatusInternalServerError)
		return
	}

	user, _ := db.GetUserByID(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func AdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := db.DeleteUser(id); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"ok":true}`))
}

// ── Analytics Settings ──

func AdminGetAnalyticsSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := db.GetAnalyticsSettings()
	if err != nil {
		jsonError(w, "failed to get settings", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

func AdminUpdateAnalyticsSettings(w http.ResponseWriter, r *http.Request) {
	var req AnalyticsSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.UmamiScriptURL != "" {
		if !strings.HasPrefix(req.UmamiScriptURL, "https://") && !strings.HasPrefix(req.UmamiScriptURL, "http://") {
			jsonError(w, "script URL must start with http:// or https://", http.StatusBadRequest)
			return
		}
	}

	if err := db.UpdateAnalyticsSettings(req.UmamiScriptURL, req.UmamiWebsiteID, req.TrackingEnabled); err != nil {
		jsonError(w, "failed to save settings", http.StatusInternalServerError)
		return
	}

	settings, _ := db.GetAnalyticsSettings()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

// ── Email Settings (admin) ──

func AdminGetEmailSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := db.GetEmailSettings()
	if err != nil {
		jsonError(w, "failed to get email settings", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

func AdminUpdateEmailSettings(w http.ResponseWriter, r *http.Request) {
	var req EmailSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.SMTPPort == 0 {
		req.SMTPPort = 587
	}
	if req.Encryption == "" {
		req.Encryption = "tls"
	}

	if err := db.UpdateEmailSettings(req.SMTPHost, req.SMTPPort, req.Username, req.Password, req.FromAddr, req.Encryption, req.GotifyURL, req.GotifyToken); err != nil {
		jsonError(w, "failed to save email settings", http.StatusInternalServerError)
		return
	}

	settings, _ := db.GetEmailSettings()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

func AdminTestEmail(w http.ResponseWriter, r *http.Request) {
	s, err := db.GetEmailSettings()
	if err != nil {
		jsonError(w, "failed to get email settings", http.StatusInternalServerError)
		return
	}

	to := r.FormValue("to")
	if to == "" {
		// Get admin email
		userID := getUserID(r)
		user, err := db.GetUserByID(userID)
		if err == nil {
			to = user.Email
		}
	}
	if to == "" {
		jsonError(w, "no recipient email available", http.StatusBadRequest)
		return
	}

	err = SendEmail(to, "Test e-mail — Red Copper Chef 🍳",
		`Dit is een test-e-mail van Red Copper Chef.

Als je dit leest, werkt de SMTP-configuratie correct!

— Red Copper Chef 🍳`)
	if err != nil {
		jsonError(w, fmt.Sprintf("email test failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Also test Gotify if configured
	if s.GotifyURL != "" && s.GotifyToken != "" {
		SendGotifyNotification("Test notificatie", "Gotify werkt correct! ✅")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":    true,
		"email": to,
	})
}

// ── Setup ──

func Setup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username        string `json:"username"`
		Password        string `json:"password"`
		ConfirmPassword string `json:"confirm_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	hasUsers, err := db.HasUsers()
	if err != nil {
		jsonError(w, "failed to check setup status", http.StatusInternalServerError)
		return
	}
	if hasUsers {
		jsonError(w, "admin already configured", http.StatusForbidden)
		return
	}

	if req.Username == "" {
		jsonError(w, "username is required", http.StatusBadRequest)
		return
	}
	if req.Password == "" {
		jsonError(w, "password is required", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 4 {
		jsonError(w, "password must be at least 4 characters", http.StatusBadRequest)
		return
	}
	if req.Password != req.ConfirmPassword {
		jsonError(w, "passwords do not match", http.StatusBadRequest)
		return
	}

	// Use username as email too (backward compat)
	userID, err := db.CreateUser(req.Username, req.Password)
	if err != nil {
		jsonError(w, "failed to create admin user", http.StatusInternalServerError)
		return
	}

	session, err := db.CreateUserSession(userID)
	if err != nil {
		jsonError(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, session.Token)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"token":    session.Token,
		"redirect": "/admin.html",
	})
}

func SetupStatus(w http.ResponseWriter, r *http.Request) {
	hasUsers, err := db.HasUsers()
	if err != nil {
		jsonError(w, "failed to check", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{
		"needs_setup": !hasUsers,
	})
}

// ── Comments (admin) ──

func AdminListComments(w http.ResponseWriter, r *http.Request) {
	comments, err := db.GetAllComments()
	if err != nil {
		jsonError(w, "failed to list comments", http.StatusInternalServerError)
		return
	}
	if comments == nil {
		comments = []db.AdminComment{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comments)
}

// ── Linked Posts (admin) ──

func AdminGetPostLinks(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	postID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid post id", http.StatusBadRequest)
		return
	}

	links, err := db.GetPostLinks(postID)
	if err != nil {
		jsonError(w, "failed to get links", http.StatusInternalServerError)
		return
	}
	if links == nil {
		links = []db.PostLink{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(links)
}

func AdminListAllPostsSimple(w http.ResponseWriter, r *http.Request) {
	posts, err := db.GetPosts(nil)
	if err != nil {
		jsonError(w, "failed to list posts", http.StatusInternalServerError)
		return
	}
	// Return lightweight list for the link picker
	type simplePost struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
	}
	result := make([]simplePost, len(posts))
	for i, p := range posts {
		result[i] = simplePost{ID: p.ID, Title: p.Title}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

type SetLinksRequest struct {
	LinkedIDs []int64 `json:"linked_ids"`
}

func AdminSetPostLinks(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	postID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid post id", http.StatusBadRequest)
		return
	}

	var req SetLinksRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := db.SetPostLinks(postID, req.LinkedIDs); err != nil {
		jsonError(w, "failed to set links", http.StatusInternalServerError)
		return
	}

	links, _ := db.GetPostLinks(postID)
	if links == nil {
		links = []db.PostLink{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(links)
}

// ── ID Generator ──

var idCounter int64

func generateID() int64 {
	idCounter++
	return idCounter + int64(os.Getpid())*1000000
}

// ── Media Processing ──

func processMedia(postID int64, rawFilename, mediaType, ext string) {
	log.Printf("[media] Processing post %d (%s)...", postID, mediaType)

	srcPath := filepath.Join(uploadDir, rawFilename)
	defer os.Remove(srcPath) // Always clean up raw file

	var processedFilename, thumbnailFilename string
	var err error

	switch mediaType {
	case "photo":
		processedFilename, err = processImage(srcPath, ext)
	case "video":
		processedFilename, thumbnailFilename, err = processVideo(srcPath)
	default:
		err = fmt.Errorf("unknown media type: %s", mediaType)
	}

	if err != nil {
		log.Printf("[media] Error processing post %d: %v", postID, err)
		// Mark post with original file so it's at least viewable
		db.UpdatePostProcessing(postID, rawFilename, rawFilename)
		return
	}

	if err := db.UpdatePostProcessing(postID, processedFilename, thumbnailFilename); err != nil {
		log.Printf("[media] Error updating post %d after processing: %v", postID, err)
	}

	log.Printf("[media] Post %d processed: %s", postID, processedFilename)
}
