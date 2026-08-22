package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"redchef/db"
)

func makeDraftPost(t *testing.T, title string) *db.Post {
	t.Helper()
	post, err := db.CreatePost(title, "desc", "photo", title+".jpg", title+".jpg", false)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	if err := db.SetPostStatus(post.ID, db.PostStatusDraft, nil); err != nil {
		t.Fatalf("SetPostStatus: %v", err)
	}
	return post
}

func TestDrafts_ExcludedFromPublicSurfaces(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	draft := makeDraftPost(t, "Secret Draft")
	db.CreatePost("Live One", "", "photo", "live.jpg", "live.jpg", false)
	// List exclusion
	resp := httptest.NewRecorder()
	ListPosts(resp, httptest.NewRequest("GET", "/api/posts", nil))
	var posts []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&posts)
	if len(posts) != 1 || posts[0]["title"] != "Live One" {
		t.Fatalf("public list must exclude drafts, got %v", posts)
	}

	// Single-post 404 (same as nonexistent)
	req := httptest.NewRequest("GET", "/api/posts/1", nil)
	req.SetPathValue("id", "1")
	resp = httptest.NewRecorder()
	GetPost(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Errorf("draft detail should 404, got %d", resp.Code)
	}

	// Feed exclusion
	resp = httptest.NewRecorder()
	Feed(resp, httptest.NewRequest("GET", "/feed.xml", nil))
	if strings.Contains(resp.Body.String(), "Secret Draft") {
		t.Error("feed must not contain draft")
	}

	// TRMNL exclusion
	req = httptest.NewRequest("GET", "/api/trmnl/latest", nil)
	req.SetPathValue("id", "")
	resp = httptest.NewRecorder()
	TRMNLLatestPost(resp, req)
	if strings.Contains(resp.Body.String(), "Secret Draft") {
		t.Error("TRMNL payload must not contain draft")
	}

	// Engagement mutations rejected as not-found
	mutations := []struct {
		name string
		call func(http.ResponseWriter, *http.Request)
		req  func() *http.Request
	}{
		{"comment", CreateComment, func() *http.Request {
			r := authenticatedRequest(t, "POST", "/api/posts/1/comments", strings.NewReader(`{"body":"hi"}`))
			r.SetPathValue("id", "1")
			return r
		}},
		{"favourite", ToggleFavourite, func() *http.Request {
			r := authenticatedRequest(t, "POST", "/api/posts/1/favourite", nil)
			r.SetPathValue("id", "1")
			return r
		}},
		{"tip", CreateTip, func() *http.Request {
			r := authenticatedRequest(t, "POST", "/api/posts/1/tip", strings.NewReader(`{"amount_cents":100}`))
			r.SetPathValue("id", "1")
			return r
		}},
	}
	for _, m := range mutations {
		resp := httptest.NewRecorder()
		m.call(resp, m.req())
		if resp.Code != http.StatusNotFound {
			t.Errorf("%s on draft should 404, got %d", m.name, resp.Code)
		}
	}
	_ = draft
}

func TestScheduledPost_VisibleOnlyWhenDue(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	db.CreatePost("Timed Drop", "", "photo", "t.jpg", "t.jpg", false)

	future := time.Now().UTC().Add(1 * time.Hour).Format(time.RFC3339)
	body := `{"status":"published","scheduled_at":"` + future + `"}`
	req := authenticatedRequest(t, "PATCH", "/api/admin/posts/1", strings.NewReader(body))
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	AdminUpdatePost(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("schedule PATCH failed: %d %s", resp.Code, resp.Body.String())
	}

	// Not due yet → hidden everywhere
	req = httptest.NewRequest("GET", "/api/posts/1", nil)
	req.SetPathValue("id", "1")
	resp = httptest.NewRecorder()
	GetPost(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Errorf("not-yet-due post should 404 publicly, got %d", resp.Code)
	}

	// Due → visible without any worker
	past := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	body = `{"scheduled_at":"` + past + `"}`
	req = authenticatedRequest(t, "PATCH", "/api/admin/posts/1", strings.NewReader(body))
	req.SetPathValue("id", "1")
	resp = httptest.NewRecorder()
	AdminUpdatePost(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("reschedule PATCH failed: %d", resp.Code)
	}
	req = httptest.NewRequest("GET", "/api/posts/1", nil)
	req.SetPathValue("id", "1")
	resp = httptest.NewRecorder()
	GetPost(resp, req)
	if resp.Code != http.StatusOK {
		t.Errorf("due post should be public, got %d", resp.Code)
	}
}

func TestAdmin_LifecycleTransitions(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	// Upload as draft via form field
	var buf strings.Builder
	boundary := "----redchefb2"
	buf.WriteString("--" + boundary + "\r\n")
	writeField := func(name, value string) {
		buf.WriteString(`Content-Disposition: form-data; name="` + name + `"` + "\r\n\r\n")
		buf.WriteString(value + "\r\n")
		buf.WriteString("--" + boundary + "\r\n")
	}
	writeField("title", "Work In Progress")
	writeField("status", "draft")
	buf.WriteString(`Content-Disposition: form-data; name="file"; filename="wip.jpg"` + "\r\n\r\n\x00\x01\x02\r\n")
	buf.WriteString("--" + boundary + "--\r\n")

	req := authenticatedRequest(t, "POST", "/api/admin/upload", strings.NewReader(buf.String()))
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	resp := httptest.NewRecorder()
	AdminUpload(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("draft upload: %d %s", resp.Code, resp.Body.String())
	}
	ProcessWG.Wait()

	got, _ := db.GetPost(1)
	if got.Status != db.PostStatusDraft {
		t.Fatalf("uploaded post should be draft, got %q", got.Status)
	}

	// Publish it
	req = authenticatedRequest(t, "PATCH", "/api/admin/posts/1", strings.NewReader(`{"status":"published"}`))
	req.SetPathValue("id", "1")
	resp = httptest.NewRecorder()
	AdminUpdatePost(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("publish: %d", resp.Code)
	}

	// Unpublish again
	req = authenticatedRequest(t, "PATCH", "/api/admin/posts/1", strings.NewReader(`{"status":"draft"}`))
	req.SetPathValue("id", "1")
	resp = httptest.NewRecorder()
	AdminUpdatePost(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unpublish: %d", resp.Code)
	}

	// Invalid status rejected
	req = authenticatedRequest(t, "PATCH", "/api/admin/posts/1", strings.NewReader(`{"status":"archived"}`))
	req.SetPathValue("id", "1")
	resp = httptest.NewRecorder()
	AdminUpdatePost(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Errorf("invalid status should 400, got %d", resp.Code)
	}

	// Admin list shows drafts and supports the status filter
	req = authenticatedRequest(t, "GET", "/api/admin/posts?status=draft", nil)
	resp = httptest.NewRecorder()
	AdminListPosts(resp, req)
	var posts []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&posts)
	if len(posts) != 1 || posts[0]["status"] != "draft" {
		t.Errorf("admin status filter should return the draft, got %v", posts)
	}
}

func TestAdminExport_ZipBackup(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	// A media file to ride along
	mediaPath := filepath.Join(uploadDir, "backupme.jpg")
	os.WriteFile(mediaPath, []byte("fakejpeg"), 0644)

	req := authenticatedRequest(t, "GET", "/api/admin/export", nil)
	resp := httptest.NewRecorder()
	AdminExport(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("export: %d %s", resp.Code, resp.Body.String())
	}
	if ct := resp.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("expected zip content type, got %q", ct)
	}

	zr, err := zip.NewReader(bytes.NewReader(resp.Body.Bytes()), int64(resp.Body.Len()))
	if err != nil {
		t.Fatalf("output is not a valid zip: %v", err)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	if !names["redchef.db"] {
		t.Error("archive must contain redchef.db snapshot")
	}
	if !names["media/backupme.jpg"] {
		t.Errorf("archive must contain media files, got %v", names)
	}

	// No temp snapshot left behind
	leftovers, _ := filepath.Glob(filepath.Join(os.TempDir(), "redchef_export_*.db"))
	if len(leftovers) != 0 {
		t.Errorf("temp snapshot not cleaned up: %v", leftovers)
	}
}
