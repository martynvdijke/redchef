package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"redchef/db"
)

func setupHandlerTest(t *testing.T) (cleanup func()) {
	t.Helper()
	err := db.Init("/tmp/redchef_handler_test.db")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}

	// Set upload dir to temp
	uploadDir = "/tmp/redchef_handler_test_media"
	os.MkdirAll(uploadDir, 0755)

	// Create admin user for auth
	err = db.CreateUser("admin", "test1234")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	return func() {
		db.DB.Close()
		db.DB = nil
		os.Remove("/tmp/redchef_handler_test.db")
		os.Remove("/tmp/redchef_handler_test.db-wal")
		os.Remove("/tmp/redchef_handler_test.db-shm")
		os.RemoveAll(uploadDir)
	}
}

func authenticatedRequest(t *testing.T, method, target string, body io.Reader) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, body)
	// Create session and set cookie
	session, err := db.CreateSession(1)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	req.AddCookie(&http.Cookie{
		Name:  "admin_token",
		Value: session.Token,
	})
	return req
}

func createTestImage(t *testing.T) []byte {
	t.Helper()
	// Minimal valid JPEG (1x1 pixel)
	data := []byte{
		0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0x00, 0x01,
		0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xff, 0xdb, 0x00, 0x43,
		0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09,
		0x09, 0x08, 0x0a, 0x0c, 0x14, 0x0d, 0x0c, 0x0b, 0x0b, 0x0c, 0x19, 0x12,
		0x13, 0x0f, 0x14, 0x1d, 0x1a, 0x1f, 0x1e, 0x1d, 0x1a, 0x1c, 0x1c, 0x20,
		0x24, 0x2e, 0x27, 0x20, 0x22, 0x2c, 0x23, 0x1c, 0x1c, 0x28, 0x37, 0x29,
		0x2c, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1f, 0x27, 0x39, 0x3d, 0x38, 0x32,
		0x3c, 0x2e, 0x33, 0x34, 0x32, 0xff, 0xc0, 0x00, 0x0b, 0x08, 0x00, 0x01,
		0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xff, 0xc4, 0x00, 0x1f, 0x00, 0x00,
		0x01, 0x05, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a,
		0x0b, 0xff, 0xc4, 0x00, 0xb5, 0x10, 0x00, 0x02, 0x01, 0x03, 0x03, 0x02,
		0x04, 0x03, 0x05, 0x05, 0x04, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x01, 0x02, 0x03, 0x11, 0x04, 0x12, 0x05, 0x21, 0x31, 0x06, 0x13, 0x41,
		0x51, 0x07, 0x61, 0x71, 0x81, 0x14, 0x91, 0xa1, 0xb1, 0xc1, 0xd1, 0x23,
		0x33, 0x15, 0x52, 0xf0, 0x24, 0x62, 0x72, 0x82, 0x09, 0x0a, 0x16, 0x17,
		0x18, 0x19, 0x1a, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2a, 0x34, 0x35, 0x36,
		0x37, 0x38, 0x39, 0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4a,
		0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5a, 0x63, 0x64, 0x65, 0x66,
		0x67, 0x68, 0x69, 0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79, 0x7a,
		0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89, 0x8a, 0x92, 0x93, 0x94, 0x95,
		0x96, 0x97, 0x98, 0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8,
		0xa9, 0xaa, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba, 0xc2,
		0xc3, 0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2, 0xd3, 0xd4, 0xd5,
		0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xe1, 0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7,
		0xe8, 0xe9, 0xea, 0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8, 0xf9,
		0xff, 0xda, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3f, 0x00, 0xff, 0xd9,
	}
	return data
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

	var post db.Post
	if err := json.NewDecoder(resp.Body).Decode(&post); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if post.Title != "My Upload" {
		t.Errorf("expected title 'My Upload', got %q", post.Title)
	}
	if post.MediaType != "photo" {
		t.Errorf("expected media_type 'photo', got %q", post.MediaType)
	}
	if !post.Locked {
		t.Error("expected locked=true")
	}
	if post.CreatedAt.IsZero() {
		t.Error("created_at is zero time — date parsing broken")
	}

	// Verify file was saved
	filePath := filepath.Join(uploadDir, post.Filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("uploaded file not found at %s", filePath)
	}

	// Clean up test file
	os.Remove(filePath)
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

			var post db.Post
			json.NewDecoder(resp.Body).Decode(&post)
			if post.Locked != tt.wantLocked {
				t.Errorf("locked=%v, want %v", post.Locked, tt.wantLocked)
			}

			// Cleanup
			db.DeletePost(post.ID)
			os.Remove(filepath.Join(uploadDir, post.Filename))
		})
	}
}

func createPostForTest(t *testing.T) *db.Post {
	t.Helper()
	imgData := createTestImage(t)
	req := buildUploadRequest(t, "Test Post", "desc", "true", imgData, "test.jpg")
	resp := httptest.NewRecorder()
	AdminUpload(resp, req)
	var post db.Post
	json.NewDecoder(resp.Body).Decode(&post)
	return &post
}

func TestAdminUpdatePost_Lock(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	post := createPostForTest(t)
	defer db.DeletePost(post.ID)
	defer os.Remove(filepath.Join(uploadDir, post.Filename))

	if !post.Locked {
		t.Fatal("expected post to be locked initially")
	}

	// Unlock
	idStr := fmt.Sprintf("%d", post.ID)
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

	var post db.Post
	json.NewDecoder(resp.Body).Decode(&post)
	if post.Title == "" {
		t.Error("expected auto-generated title, got empty")
	}
	// Title should be today's date, e.g. "July 20, 2026"
	today := time.Now().Format("January 2, 2006")
	if post.Title != today {
		t.Errorf("expected title %q, got %q", today, post.Title)
	}

	db.DeletePost(post.ID)
	os.Remove(filepath.Join(uploadDir, post.Filename))
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

	var post db.Post
	json.NewDecoder(resp.Body).Decode(&post)
	if post.MediaType != "video" {
		t.Errorf("expected media_type 'video', got %q", post.MediaType)
	}

	db.DeletePost(post.ID)
	os.Remove(filepath.Join(uploadDir, post.Filename))
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

	var post db.Post
	json.NewDecoder(resp.Body).Decode(&post)
	if post.MediaType != "photo" {
		t.Errorf("expected media_type 'photo', got %q", post.MediaType)
	}

	db.DeletePost(post.ID)
	os.Remove(filepath.Join(uploadDir, post.Filename))
}
