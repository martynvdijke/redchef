package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"redchef/db"
)

// uploadResponse mirrors the AdminUpload response
type uploadResponse struct {
	ID         int64     `json:"id"`
	Title      string    `json:"title"`
	MediaType  string    `json:"media_type"`
	Locked     bool      `json:"locked"`
	Processing bool      `json:"processing"`
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"created_at"`
}

func setupHandlerTest(t *testing.T) (cleanup func()) {
	t.Helper()
	err := db.Init("/tmp/redchef_handler_test.db")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}

	uploadDir = "/tmp/redchef_handler_test_media"
	os.MkdirAll(uploadDir, 0755)

	_, err = db.CreateUser("admin", "test1234")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	return func() {
		ProcessWG.Wait() // Wait for any in-flight media processing goroutines
		db.DB.Close()
		// Don't set db.DB = nil — background notification goroutines
		// may still try to read email settings and nil DB would panic.
		os.Remove("/tmp/redchef_handler_test.db")
		os.Remove("/tmp/redchef_handler_test.db-wal")
		os.Remove("/tmp/redchef_handler_test.db-shm")
		os.RemoveAll(uploadDir)
	}
}

func authenticatedRequest(t *testing.T, method, target string, body io.Reader) *http.Request {
	t.Helper()

	// Wait for any in-flight media processing to finish before creating a new session
	ProcessWG.Wait()

	req := httptest.NewRequest(method, target, body)

	// Create user session
	user, err := db.GetUserByUsername("admin")
	if err != nil {
		// Use email lookup if username fails
		user, err = db.GetUserByEmail("admin")
		if err != nil {
			t.Fatalf("get admin user: %v", err)
		}
	}
	session, err := db.CreateUserSession(user.ID)
	if err != nil {
		t.Fatalf("create user session: %v", err)
	}
	req.AddCookie(&http.Cookie{
		Name:  "session_token",
		Value: session.Token,
	})
	// Set X-User-ID header (normally done by AuthMiddleware) so handlers can identify the user
	req.Header.Set("X-User-ID", fmt.Sprintf("%d", user.ID))
	req.Header.Set("X-User-Role", user.Role)
	return req
}

func createTestImage(t *testing.T) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.NRGBA{255, 0, 0, 255})
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode test image: %v", err)
	}
	return buf.Bytes()
}

func buildUploadRequest(t *testing.T, title, description, locked string, fileData []byte, filename string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if fileData != nil {
		part, err := w.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		part.Write(fileData)
	}

	w.WriteField("title", title)
	if description != "" {
		w.WriteField("description", description)
	}
	if locked != "" {
		w.WriteField("locked", locked)
	}
	w.Close()

	req := authenticatedRequest(t, "POST", "/api/admin/upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestAdminUpload_Success(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	imgData := createTestImage(t)
	req := buildUploadRequest(t, "My Upload", "A description", "true", imgData, "test.jpg")
	resp := httptest.NewRecorder()

	AdminUpload(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", resp.Code, resp.Body.String())
	}

	var result uploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.Title != "My Upload" {
		t.Errorf("expected title 'My Upload', got %q", result.Title)
	}
	if result.MediaType != "photo" {
		t.Errorf("expected media_type 'photo', got %q", result.MediaType)
	}
	if !result.Locked {
		t.Error("expected locked=true")
	}
	if !result.Processing {
		t.Error("expected processing=true")
	}
	if result.CreatedAt.IsZero() {
		t.Error("created_at is zero time")
	}
}

func TestAdminUpload_LockedField(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	imgData := createTestImage(t)

	tests := []struct {
		name       string
		lockedVal  string
		wantLocked bool
	}{
		{"unchecked (no field)", "", false},
		{"explicit true", "true", true},
		{"false string", "false", false},
		{"on value (HTML checkbox)", "on", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := buildUploadRequest(t, "Locked Test", "", tt.lockedVal, imgData, "test.jpg")
			resp := httptest.NewRecorder()
			AdminUpload(resp, req)

			if resp.Code != http.StatusCreated {
				t.Fatalf("expected 201, got %d: %s", resp.Code, resp.Body.String())
			}

			var result uploadResponse
			json.NewDecoder(resp.Body).Decode(&result)
			if result.Locked != tt.wantLocked {
				t.Errorf("locked=%v, want %v", result.Locked, tt.wantLocked)
			}
		})
	}
}

func createPostForTest(t *testing.T) *uploadResponse {
	t.Helper()
	imgData := createTestImage(t)
	req := buildUploadRequest(t, "Test Post", "desc", "true", imgData, "test.jpg")
	resp := httptest.NewRecorder()
	AdminUpload(resp, req)
	var result uploadResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return &result
}

func TestAdminUpdatePost_Lock(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	upload := createPostForTest(t)
	defer db.DeletePost(upload.ID)

	if !upload.Locked {
		t.Fatal("expected post to be locked initially")
	}

	// Unlock
	idStr := fmt.Sprintf("%d", upload.ID)
	body := bytes.NewReader([]byte(`{"locked":false}`))
	req := authenticatedRequest(t, "PATCH", "/api/admin/posts/"+idStr, body)
	req.SetPathValue("id", idStr)
	resp := httptest.NewRecorder()
	AdminUpdatePost(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var updated db.Post
	json.NewDecoder(resp.Body).Decode(&updated)
	if updated.Locked {
		t.Error("expected locked=false after update")
	}

	// Lock again
	body = bytes.NewReader([]byte(`{"locked":true}`))
	req = authenticatedRequest(t, "PATCH", "/api/admin/posts/"+idStr, body)
	req.SetPathValue("id", idStr)
	resp = httptest.NewRecorder()
	AdminUpdatePost(resp, req)

	json.NewDecoder(resp.Body).Decode(&updated)
	if !updated.Locked {
		t.Error("expected locked=true after second update")
	}
}

func TestAdminUpdatePost_NotFound(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"locked":false}`))
	req := authenticatedRequest(t, "PATCH", "/api/admin/posts/99999", body)
	req.SetPathValue("id", "99999")
	resp := httptest.NewRecorder()
	AdminUpdatePost(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestAdminUpdatePost_MissingField(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{}`))
	req := authenticatedRequest(t, "PATCH", "/api/admin/posts/1", body)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	AdminUpdatePost(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestAdminUpdatePost_InvalidID(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"locked":true}`))
	req := authenticatedRequest(t, "PATCH", "/api/admin/posts/abc", body)
	req.SetPathValue("id", "abc")
	resp := httptest.NewRecorder()
	AdminUpdatePost(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestAdminUpload_AutoTitle(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	imgData := createTestImage(t)
	req := buildUploadRequest(t, "", "", "true", imgData, "test.jpg")
	resp := httptest.NewRecorder()

	AdminUpload(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.Code, resp.Body.String())
	}

	var result uploadResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Title == "" {
		t.Error("expected auto-generated title, got empty")
	}
	today := time.Now().Format("January 2, 2006")
	if result.Title != today {
		t.Errorf("expected title %q, got %q", today, result.Title)
	}
}

func TestAdminUpload_MissingFile(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := buildUploadRequest(t, "No File", "desc", "true", nil, "")
	resp := httptest.NewRecorder()

	AdminUpload(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestAdminUpload_UnsupportedType(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := buildUploadRequest(t, "Bad File", "", "true", []byte("not an image"), "test.exe")
	resp := httptest.NewRecorder()

	AdminUpload(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestAdminUpload_VideoType(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := buildUploadRequest(t, "My Video", "desc", "false", []byte("fake video"), "video.mp4")
	resp := httptest.NewRecorder()

	AdminUpload(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.Code, resp.Body.String())
	}

	var result uploadResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.MediaType != "video" {
		t.Errorf("expected media_type 'video', got %q", result.MediaType)
	}
}

func TestAdminGetEmailSettings(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := authenticatedRequest(t, "GET", "/api/admin/settings/email", nil)
	resp := httptest.NewRecorder()
	AdminGetEmailSettings(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var settings db.EmailSettings
	if err := json.NewDecoder(resp.Body).Decode(&settings); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if settings.SMTPPort != 587 {
		t.Errorf("expected default port 587, got %d", settings.SMTPPort)
	}
	if settings.Encryption != "tls" {
		t.Errorf("expected default encryption 'tls', got %q", settings.Encryption)
	}
	if settings.ID != 1 {
		t.Errorf("expected ID 1, got %d", settings.ID)
	}
}

func TestAdminUpdateEmailSettings(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{
		"smtp_host": "smtp.example.com",
		"smtp_port": 465,
		"username": "user",
		"password": "secret",
		"from_addr": "noreply@example.com",
		"encryption": "ssl"
	}`))
	req := authenticatedRequest(t, "PUT", "/api/admin/settings/email", body)
	resp := httptest.NewRecorder()
	AdminUpdateEmailSettings(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var settings db.EmailSettings
	if err := json.NewDecoder(resp.Body).Decode(&settings); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if settings.SMTPHost != "smtp.example.com" {
		t.Errorf("expected host 'smtp.example.com', got %q", settings.SMTPHost)
	}
	if settings.SMTPPort != 465 {
		t.Errorf("expected port 465, got %d", settings.SMTPPort)
	}
	if settings.Encryption != "ssl" {
		t.Errorf("expected encryption 'ssl', got %q", settings.Encryption)
	}
}

func TestAdminUpdateEmailSettings_Defaults(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	// Minimal payload — should get default port and encryption
	body := bytes.NewReader([]byte(`{"smtp_host": "smtp.example.com"}`))
	req := authenticatedRequest(t, "PUT", "/api/admin/settings/email", body)
	resp := httptest.NewRecorder()
	AdminUpdateEmailSettings(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var settings db.EmailSettings
	json.NewDecoder(resp.Body).Decode(&settings)

	if settings.SMTPHost != "smtp.example.com" {
		t.Errorf("expected host 'smtp.example.com', got %q", settings.SMTPHost)
	}
	if settings.SMTPPort != 587 {
		t.Errorf("expected default port 587, got %d", settings.SMTPPort)
	}
	if settings.Encryption != "tls" {
		t.Errorf("expected default encryption 'tls', got %q", settings.Encryption)
	}
}

func TestAdminUpload_WebPSupport(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := buildUploadRequest(t, "WebP", "", "false", []byte("webp data"), "image.webp")
	resp := httptest.NewRecorder()

	AdminUpload(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.Code, resp.Body.String())
	}

	var result uploadResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.MediaType != "photo" {
		t.Errorf("expected media_type 'photo', got %q", result.MediaType)
	}
}
