package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"redchef/db"
)

type authResponse struct {
	Token string `json:"token"`
	Role  string `json:"role"`
	Error string `json:"error"`
}

// ── Register ──

func TestRegister_Success(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"email":"new@user.com","password":"password123","confirm_password":"password123"}`))
	req := httptest.NewRequest("POST", "/api/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	Register(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["token"] == "" {
		t.Error("expected non-empty token")
	}
	if result["role"] != "normal" {
		t.Errorf("expected role 'normal', got %v", result["role"])
	}
}

func TestRegister_InvalidEmail(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"email":"invalid","password":"password123","confirm_password":"password123"}`))
	req := httptest.NewRequest("POST", "/api/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	Register(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestRegister_EmptyEmail(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"email":"","password":"password123","confirm_password":"password123"}`))
	req := httptest.NewRequest("POST", "/api/auth/register", body)
	resp := httptest.NewRecorder()
	Register(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"email":"user@test.com","password":"12345","confirm_password":"12345"}`))
	req := httptest.NewRequest("POST", "/api/auth/register", body)
	resp := httptest.NewRecorder()
	Register(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestRegister_PasswordMismatch(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"email":"user@test.com","password":"password123","confirm_password":"different"}`))
	req := httptest.NewRequest("POST", "/api/auth/register", body)
	resp := httptest.NewRecorder()
	Register(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	// Register the first time
	body := bytes.NewReader([]byte(`{"email":"dup@test.com","password":"password123","confirm_password":"password123"}`))
	req := httptest.NewRequest("POST", "/api/auth/register", body)
	resp := httptest.NewRecorder()
	Register(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201 first time, got %d", resp.Code)
	}

	// Register again with same email
	body = bytes.NewReader([]byte(`{"email":"dup@test.com","password":"password123","confirm_password":"password123"}`))
	req = httptest.NewRequest("POST", "/api/auth/register", body)
	resp = httptest.NewRecorder()
	Register(resp, req)

	if resp.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate, got %d", resp.Code)
	}
}

func TestRegister_InvalidJSON(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`not json`))
	req := httptest.NewRequest("POST", "/api/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	Register(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

// ── Login ──

func TestLogin_Success(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"email":"admin","password":"test1234"}`))
	req := httptest.NewRequest("POST", "/api/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	Login(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["token"] == "" {
		t.Error("expected non-empty token")
	}
	if result["role"] != "admin" {
		t.Errorf("expected role 'admin', got %v", result["role"])
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"email":"admin","password":"wrongpassword"}`))
	req := httptest.NewRequest("POST", "/api/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	Login(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestLogin_NonexistentUser(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"email":"nonexistent@test.com","password":"password"}`))
	req := httptest.NewRequest("POST", "/api/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	Login(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestLogin_UsernameFallback(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	// user2 is created to test login with a non-admin account
	_, err := db.CreateUser("user@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Login with email
	body := bytes.NewReader([]byte(`{"email":"user@test.com","password":"password123"}`))
	req := httptest.NewRequest("POST", "/api/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	Login(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 with correct email, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestLogin_EmptyFields(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := bytes.NewReader([]byte(`{"email":"","password":""}`))
	req := httptest.NewRequest("POST", "/api/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	Login(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

// ── Logout ──

func TestLogout_ClearsCookies(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	// Login first
	body := bytes.NewReader([]byte(`{"email":"admin","password":"test1234"}`))
	loginReq := httptest.NewRequest("POST", "/api/auth/login", body)
	loginResp := httptest.NewRecorder()
	Login(loginResp, loginReq)

	// Extract session cookie
	cookies := loginResp.Result().Cookies()
	var sessionToken string
	for _, c := range cookies {
		if c.Name == "session_token" {
			sessionToken = c.Value
		}
	}

	// Logout
	logoutReq := httptest.NewRequest("POST", "/api/auth/logout", nil)
	logoutReq.AddCookie(&http.Cookie{Name: "session_token", Value: sessionToken})
	logoutResp := httptest.NewRecorder()
	Logout(logoutResp, logoutReq)

	if logoutResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", logoutResp.Code)
	}

	// Verify session was deleted
	_, err := db.GetUserSession(sessionToken)
	if err == nil {
		t.Error("expected session to be deleted after logout")
	}
}

func TestLogout_WithoutSession(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	resp := httptest.NewRecorder()
	Logout(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}

// ── Me ──

func TestMe_Authenticated(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	_, err := db.CreateUserSession(1) // admin user
	if err != nil {
		t.Fatalf("CreateUserSession: %v", err)
	}

	// Create request with auth middleware context
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("X-User-ID", "1")
	resp := httptest.NewRecorder()
	Me(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["authenticated"] != true {
		t.Error("expected authenticated=true")
	}
	if result["email"] != "admin" {
		t.Errorf("expected email 'admin', got %v", result["email"])
	}
}

func TestMe_Unauthenticated(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	resp := httptest.NewRecorder()
	Me(resp, req)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["authenticated"] != false {
		t.Error("expected authenticated=false for unauthenticated request")
	}
}

func TestMe_UserNotFound(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("X-User-ID", "999")
	resp := httptest.NewRecorder()
	Me(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}
}

// ── AuthMiddleware ──

func TestAuthMiddleware_SetsUserHeaders(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	session, err := db.CreateUserSession(1)
	if err != nil {
		t.Fatalf("CreateUserSession: %v", err)
	}

	handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		role := r.Header.Get("X-User-Role")
		if userID == "" || userID == "0" {
			t.Error("expected X-User-ID to be set")
		}
		if role != "admin" {
			t.Errorf("expected X-User-Role 'admin', got %q", role)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: session.Token})
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}

func TestAuthMiddleware_NoSession(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID != "" {
			t.Error("expected no X-User-ID for unauthenticated request")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}

// ── RequireAuth ──

func TestRequireAuth_Authenticated(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	handler := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-User-ID", "1")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}

func TestRequireAuth_Unauthenticated(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	handler := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

// ── AdminAuth ──

func TestAdminAuth_AdminUser(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	session, err := db.CreateUserSession(1) // user 1 is admin
	if err != nil {
		t.Fatalf("CreateUserSession: %v", err)
	}

	handler := AdminAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role := r.Header.Get("X-User-Role")
		if role != "admin" {
			t.Errorf("expected role 'admin', got %q", role)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: session.Token})
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}

func TestAdminAuth_NonAdminUser(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	// Create normal user
	userID, err := db.CreateUser("normal@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	session, err := db.CreateUserSession(userID)
	if err != nil {
		t.Fatalf("CreateUserSession: %v", err)
	}

	handler := AdminAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for non-admin")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: session.Token})
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestAdminAuth_Unauthenticated(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	handler := AdminAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestAdminAuth_LegacyToken(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	// Create legacy session
	session, err := db.CreateSession(1) // user 1 is admin
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	handler := AdminAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "admin_token", Value: session.Token})
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 for legacy token, got %d", resp.Code)
	}
}
