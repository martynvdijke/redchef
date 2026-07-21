package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"redchef/db"
)

func TestAdminListUsers_Empty(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := authenticatedRequest(t, "GET", "/api/admin/users", nil)
	resp := httptest.NewRecorder()
	AdminListUsers(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var users []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&users)
	if len(users) == 0 {
		t.Fatal("expected at least 1 user (admin)")
	}
}

func TestAdminListUsers_ShowsAll(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	// Create additional users
	_, err := db.CreateUser("user1@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	_, err = db.CreateUser("user2@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	req := authenticatedRequest(t, "GET", "/api/admin/users", nil)
	resp := httptest.NewRecorder()
	AdminListUsers(resp, req)

	var users []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&users)
	if len(users) != 3 {
		t.Fatalf("expected 3 users, got %d", len(users))
	}
}

func TestAdminUpdateUser_PromoteToAdmin(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	userID, err := db.CreateUser("promote@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	body := bytes.NewReader([]byte(`{"role":"admin"}`))
	req := authenticatedRequest(t, "PATCH", "/api/admin/users/2", body)
	req.SetPathValue("id", "2")
	resp := httptest.NewRecorder()
	AdminUpdateUser(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	user, err := db.GetUserByID(userID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if user.Role != "admin" {
		t.Errorf("expected role 'admin', got %q", user.Role)
	}
}

func TestAdminUpdateUser_DemoteToNormal(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	// Create a second admin user first
	adminID, err := db.CreateUser("admin2@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	db.UpdateUserRole(adminID, "admin")

	// Now demote the second admin
	body := bytes.NewReader([]byte(`{"role":"normal"}`))
	req := authenticatedRequest(t, "PATCH", "/api/admin/users/2", body)
	req.SetPathValue("id", "2")
	resp := httptest.NewRecorder()
	AdminUpdateUser(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	user, err := db.GetUserByID(2)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if user.Role != "normal" {
		t.Errorf("expected role 'normal', got %q", user.Role)
	}
}

func TestAdminUpdateUser_DemoteLastAdmin(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	// Try demoting the only admin (user 1)
	body := bytes.NewReader([]byte(`{"role":"normal"}`))
	req := authenticatedRequest(t, "PATCH", "/api/admin/users/1", body)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	AdminUpdateUser(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when demoting last admin, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] == nil {
		t.Error("expected error message")
	}
}

func TestAdminUpdateUser_InvalidRole(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"role":"superadmin"}`))
	req := authenticatedRequest(t, "PATCH", "/api/admin/users/1", body)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	AdminUpdateUser(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid role, got %d", resp.Code)
	}
}

func TestAdminUpdateUser_InvalidID(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"role":"admin"}`))
	req := authenticatedRequest(t, "PATCH", "/api/admin/users/abc", body)
	req.SetPathValue("id", "abc")
	resp := httptest.NewRecorder()
	AdminUpdateUser(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestAdminUpdateUser_NonexistentUser(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"role":"admin"}`))
	req := authenticatedRequest(t, "PATCH", "/api/admin/users/999", body)
	req.SetPathValue("id", "999")
	resp := httptest.NewRecorder()
	AdminUpdateUser(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.Code)
	}
}

func TestAdminUpdateUser_NoFields(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{}`))
	req := authenticatedRequest(t, "PATCH", "/api/admin/users/1", body)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	AdminUpdateUser(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestAdminDeleteUser_LastAdmin(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := authenticatedRequest(t, "DELETE", "/api/admin/users/1", nil)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	AdminDeleteUser(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when deleting last admin, got %d", resp.Code)
	}
}

func TestAdminDeleteUser_NonAdmin(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	userID, err := db.CreateUser("delete_me@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	req := authenticatedRequest(t, "DELETE", "/api/admin/users/2", nil)
	req.SetPathValue("id", "2")
	resp := httptest.NewRecorder()
	AdminDeleteUser(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	_, err = db.GetUserByID(userID)
	if err == nil {
		t.Error("expected user to be deleted")
	}
}

func TestAdminUpdateUser_InvalidIDFormat(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"paid":true}`))
	req := authenticatedRequest(t, "PATCH", "/api/admin/users/abc", body)
	req.SetPathValue("id", "abc")
	resp := httptest.NewRecorder()
	AdminUpdateUser(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestAdminUpdateUser_PaidAndRole(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	// Create a user to update
	_, err := db.CreateUser("multi@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	body := bytes.NewReader([]byte(`{"paid":true,"role":"admin"}`))
	req := authenticatedRequest(t, "PATCH", "/api/admin/users/2", body)
	req.SetPathValue("id", "2")
	resp := httptest.NewRecorder()
	AdminUpdateUser(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	user, err := db.GetUserByID(2)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if user.Role != "admin" {
		t.Errorf("expected role 'admin', got %q", user.Role)
	}
	if !user.Paid {
		t.Error("expected paid=true")
	}
}
