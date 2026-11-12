package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"redchef/db"
)

// sseRecorder is a race-free ResponseWriter for the streaming handler: the
// handler writes from one goroutine while the test reads from another.
type sseRecorder struct {
	mu   sync.Mutex
	h    http.Header
	body bytes.Buffer
	code int
}

func (r *sseRecorder) Header() http.Header {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.h == nil {
		r.h = make(http.Header)
	}
	return r.h
}

func (r *sseRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.Write(p)
}

func (r *sseRecorder) WriteHeader(code int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.code = code
}

func (r *sseRecorder) Flush() {}

func (r *sseRecorder) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.String()
}

func waitForStream(t *testing.T, rec *sseRecorder, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(rec.String(), want) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("stream never contained %q; body so far:\n%s", want, rec.String())
}

func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder, dst interface{}) {
	t.Helper()
	if err := json.Unmarshal(rr.Body.Bytes(), dst); err != nil {
		t.Fatalf("decode response %q: %v", rr.Body.String(), err)
	}
}

func TestGetNotificationConfig(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	tests := []struct {
		name       string
		settings   db.NotificationSettings
		wantEnable bool
	}{
		{
			name:       "defaults",
			settings:   db.DefaultNotificationSettings(),
			wantEnable: true,
		},
		{
			name: "disabled master switch with web push on",
			settings: db.NotificationSettings{
				ID: 1, Enabled: false, NewPostEnabled: false,
				WebPushEnabled: true, VAPIDPublicKey: "public",
			},
			wantEnable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := db.UpdateNotificationSettings(tt.settings); err != nil {
				t.Fatalf("UpdateNotificationSettings: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, "/api/notifications/config", nil)
			rr := httptest.NewRecorder()
			GetNotificationConfig(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rr.Code)
			}
			var got struct {
				Enabled         bool   `json:"enabled"`
				NewPostEnabled  bool   `json:"new_post_enabled"`
				WebPushEnabled  bool   `json:"web_push_enabled"`
				FCMEnabled      bool   `json:"fcm_enabled"`
				VAPIDPublicKey  string `json:"vapid_public_key"`
				VAPIDPrivateKey string `json:"vapid_private_key"`
			}
			decodeJSON(t, rr, &got)
			if got.Enabled != tt.wantEnable {
				t.Fatalf("enabled = %v, want %v", got.Enabled, tt.wantEnable)
			}
			if got.VAPIDPublicKey != tt.settings.VAPIDPublicKey {
				t.Fatalf("vapid_public_key = %q, want %q", got.VAPIDPublicKey, tt.settings.VAPIDPublicKey)
			}
			if got.VAPIDPrivateKey != "" {
				t.Fatalf("public config leaked the VAPID private key: %q", got.VAPIDPrivateKey)
			}
		})
	}
}

func TestNotificationPreferencesHandlers(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	// A user without a stored row gets all-on defaults.
	req := authenticatedRequest(t, http.MethodGet, "/api/notifications/preferences", nil)
	rr := httptest.NewRecorder()
	GetNotificationPreferences(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", rr.Code)
	}
	var prefs db.NotificationPrefs
	decodeJSON(t, rr, &prefs)
	if !prefs.NewPosts || !prefs.Email || !prefs.Push {
		t.Fatalf("defaults should be all true, got %+v", prefs)
	}

	// Turning push off keeps the other values.
	req = authenticatedRequest(t, http.MethodPut, "/api/notifications/preferences",
		strings.NewReader(`{"push":false}`))
	rr = httptest.NewRecorder()
	UpdateNotificationPreferences(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200 (%s)", rr.Code, rr.Body.String())
	}
	decodeJSON(t, rr, &prefs)
	if prefs.Push || !prefs.NewPosts || !prefs.Email {
		t.Fatalf("partial PUT should only change push, got %+v", prefs)
	}

	// A follow-up GET returns the stored values.
	req = authenticatedRequest(t, http.MethodGet, "/api/notifications/preferences", nil)
	rr = httptest.NewRecorder()
	GetNotificationPreferences(rr, req)
	decodeJSON(t, rr, &prefs)
	if prefs.Push || !prefs.NewPosts || !prefs.Email {
		t.Fatalf("GET after PUT = %+v, want push=false", prefs)
	}

	// Invalid body is rejected.
	req = authenticatedRequest(t, http.MethodPut, "/api/notifications/preferences",
		strings.NewReader(`not json`))
	rr = httptest.NewRecorder()
	UpdateNotificationPreferences(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid body status = %d, want 400", rr.Code)
	}
}

func TestDeviceTokenHandlers(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := authenticatedRequest(t, http.MethodPost, "/api/notifications/devices",
		strings.NewReader(`{"token":" device-token-1 ","platform":"android"}`))
	rr := httptest.NewRecorder()
	RegisterDeviceToken(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST status = %d, want 200 (%s)", rr.Code, rr.Body.String())
	}
	if strings.TrimSpace(rr.Body.String()) != `{"ok":true}` {
		t.Fatalf("body = %q, want ok", rr.Body.String())
	}

	tokens, err := db.ListDeviceTokens(getUserID(req))
	if err != nil {
		t.Fatalf("ListDeviceTokens: %v", err)
	}
	if len(tokens) != 1 || tokens[0] != "device-token-1" {
		t.Fatalf("stored tokens = %v, want [device-token-1]", tokens)
	}

	// Empty token is a client error.
	req = authenticatedRequest(t, http.MethodPost, "/api/notifications/devices",
		strings.NewReader(`{"token":""}`))
	rr = httptest.NewRecorder()
	RegisterDeviceToken(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("empty token status = %d, want 400", rr.Code)
	}

	req = authenticatedRequest(t, http.MethodDelete, "/api/notifications/devices",
		strings.NewReader(`{"token":"device-token-1"}`))
	rr = httptest.NewRecorder()
	UnregisterDeviceToken(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("DELETE status = %d, want 200 (%s)", rr.Code, rr.Body.String())
	}
	if tokens, _ := db.ListDeviceTokens(getUserID(req)); len(tokens) != 0 {
		t.Fatalf("tokens after delete = %v, want none", tokens)
	}

	req = authenticatedRequest(t, http.MethodDelete, "/api/notifications/devices",
		strings.NewReader(`{"token":""}`))
	rr = httptest.NewRecorder()
	UnregisterDeviceToken(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("empty token DELETE status = %d, want 400", rr.Code)
	}
}

func TestWebPushSubscriptionHandlers(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	body := `{"endpoint":"https://push.example.com/sub","keys":{"p256dh":"key-p256dh","auth":"key-auth"}}`
	req := authenticatedRequest(t, http.MethodPost, "/api/notifications/web-push", strings.NewReader(body))
	req.Header.Set("User-Agent", "TestAgent/1.0")
	rr := httptest.NewRecorder()
	SubscribeWebPush(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST status = %d, want 200 (%s)", rr.Code, rr.Body.String())
	}

	subs, err := db.ListWebPushSubscriptions(getUserID(req))
	if err != nil {
		t.Fatalf("ListWebPushSubscriptions: %v", err)
	}
	if len(subs) != 1 || subs[0].Endpoint != "https://push.example.com/sub" {
		t.Fatalf("stored subscriptions = %+v, want one endpoint", subs)
	}
	if subs[0].P256dh != "key-p256dh" || subs[0].Auth != "key-auth" {
		t.Fatalf("stored keys = %+v", subs[0])
	}

	// Incomplete payloads are rejected.
	for _, incomplete := range []string{
		`{"endpoint":"https://push.example.com/sub","keys":{"auth":"a"}}`,
		`{"endpoint":"","keys":{"p256dh":"p","auth":"a"}}`,
		`{"endpoint":"https://push.example.com/sub","keys":{"p256dh":"p"}}`,
	} {
		req := authenticatedRequest(t, http.MethodPost, "/api/notifications/web-push", strings.NewReader(incomplete))
		rr := httptest.NewRecorder()
		SubscribeWebPush(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("incomplete payload %s: status = %d, want 400", incomplete, rr.Code)
		}
	}

	req = authenticatedRequest(t, http.MethodDelete, "/api/notifications/web-push",
		strings.NewReader(`{"endpoint":"https://push.example.com/sub"}`))
	rr = httptest.NewRecorder()
	UnsubscribeWebPush(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("DELETE status = %d, want 200 (%s)", rr.Code, rr.Body.String())
	}
	if subs, _ := db.ListWebPushSubscriptions(getUserID(req)); len(subs) != 0 {
		t.Fatalf("subscriptions after delete = %+v, want none", subs)
	}

	req = authenticatedRequest(t, http.MethodDelete, "/api/notifications/web-push",
		strings.NewReader(`{"endpoint":""}`))
	rr = httptest.NewRecorder()
	UnsubscribeWebPush(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("empty endpoint DELETE status = %d, want 400", rr.Code)
	}
}

func TestAdminNotificationSettingsRoundTrip(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	// Enabling a transport without credentials is normalized back to off.
	body := `{"enabled":true,"new_post_enabled":true,"web_push_enabled":true,"fcm_enabled":true}`
	req := authenticatedRequest(t, http.MethodPut, "/api/admin/settings/notifications", strings.NewReader(body))
	rr := httptest.NewRecorder()
	AdminUpdateNotificationSettings(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200 (%s)", rr.Code, rr.Body.String())
	}
	var saved db.NotificationSettings
	decodeJSON(t, rr, &saved)
	if saved.WebPushEnabled || saved.FCMEnabled {
		t.Fatalf("transports without credentials should stay off, got %+v", saved)
	}

	body = `{
		"enabled": true,
		"new_post_enabled": false,
		"web_push_enabled": true,
		"fcm_enabled": true,
		"vapid_public_key": " pub ",
		"vapid_private_key": "priv",
		"vapid_subject": "chef@example.com",
		"fcm_project_id": "project-1",
		"fcm_client_email": "svc@project-1.iam.gserviceaccount.com",
		"fcm_private_key": "-----BEGIN PRIVATE KEY-----\\nabc\\n-----END PRIVATE KEY-----"
	}`
	req = authenticatedRequest(t, http.MethodPut, "/api/admin/settings/notifications", strings.NewReader(body))
	rr = httptest.NewRecorder()
	AdminUpdateNotificationSettings(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200 (%s)", rr.Code, rr.Body.String())
	}
	decodeJSON(t, rr, &saved)
	if !saved.Enabled || saved.NewPostEnabled || !saved.WebPushEnabled || !saved.FCMEnabled {
		t.Fatalf("unexpected saved switches: %+v", saved)
	}
	if saved.VAPIDPublicKey != "pub" {
		t.Fatalf("vapid_public_key = %q, want trimmed pub", saved.VAPIDPublicKey)
	}

	req = authenticatedRequest(t, http.MethodGet, "/api/admin/settings/notifications", nil)
	rr = httptest.NewRecorder()
	AdminGetNotificationSettings(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", rr.Code)
	}
	var fetched db.NotificationSettings
	decodeJSON(t, rr, &fetched)
	if fetched.FCMProjectID != "project-1" || fetched.VAPIDPrivateKey != "priv" {
		t.Fatalf("GET did not return the stored settings: %+v", fetched)
	}
	t.Logf("updated_at = %v", fetched.UpdatedAt)
}

func TestAdminGenerateVAPIDKeys(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := authenticatedRequest(t, http.MethodPost, "/api/admin/settings/notifications/vapid", nil)
	rr := httptest.NewRecorder()
	AdminGenerateVAPIDKeys(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rr.Code, rr.Body.String())
	}

	var keys struct {
		Public  string `json:"vapid_public_key"`
		Private string `json:"vapid_private_key"`
	}
	decodeJSON(t, rr, &keys)
	if keys.Public == "" || keys.Private == "" {
		t.Fatalf("generated keys are empty: %+v", keys)
	}
	if keys.Public == keys.Private {
		t.Fatal("public and private key are identical")
	}

	settings, err := db.GetNotificationSettings()
	if err != nil {
		t.Fatalf("GetNotificationSettings: %v", err)
	}
	if settings.VAPIDPublicKey != keys.Public || settings.VAPIDPrivateKey != keys.Private {
		t.Fatalf("keys were not persisted: %+v", settings)
	}
}

func TestNotificationHubPublishSubscribe(t *testing.T) {
	hub := NewNotificationHub()

	ch, cancel := hub.Subscribe(42)
	if got := hub.SubscriberCount(42); got != 1 {
		t.Fatalf("subscriber count = %d, want 1", got)
	}

	ev := NotificationEvent{ID: "np_1_1", Type: "new_post", Title: "Nieuwe post", Body: "body", PostID: 1}
	hub.Publish(42, ev)

	select {
	case got := <-ch:
		if got.ID != ev.ID || got.PostID != ev.PostID {
			t.Fatalf("received %+v, want %+v", got, ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the published event")
	}

	// Other users must not see it.
	other, cancelOther := hub.Subscribe(7)
	defer cancelOther()
	hub.Publish(42, ev)
	select {
	case got := <-other:
		t.Fatalf("user 7 received user 42's event: %+v", got)
	case <-time.After(50 * time.Millisecond):
	}

	// Publishing without a subscriber is a no-op.
	hub.Publish(9999, ev)

	cancel()
	if got := hub.SubscriberCount(42); got != 0 {
		t.Fatalf("subscriber count after cancel = %d, want 0", got)
	}
	// Cancelling twice is safe.
	cancel()
}

func TestNotificationHubDropsWhenFull(t *testing.T) {
	hub := NewNotificationHub()
	ch, cancel := hub.Subscribe(5)
	defer cancel()

	for i := 0; i < hubBufferSize+10; i++ {
		hub.Publish(5, NotificationEvent{ID: fmt.Sprintf("ev-%d", i)})
	}
	if got := len(ch); got != hubBufferSize {
		t.Fatalf("buffered events = %d, want %d", got, hubBufferSize)
	}
}

func TestNotificationStream(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	req := authenticatedRequest(t, http.MethodGet, "/api/notifications/stream", nil)
	userID := getUserID(req)
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	req = req.WithContext(ctx)

	rec := &sseRecorder{}
	done := make(chan struct{})
	go func() {
		NotificationStream(rec, req)
		close(done)
	}()

	waitForStream(t, rec, ": connected\n\n", time.Second)

	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", cc)
	}

	// Wait until the handler has actually subscribed.
	deadline := time.Now().Add(time.Second)
	for GlobalNotificationHub().SubscriberCount(userID) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("handler never subscribed to the hub")
		}
		time.Sleep(5 * time.Millisecond)
	}

	GlobalNotificationHub().Publish(userID, NotificationEvent{
		ID: "np_1_1", Type: "new_post", Title: "Nieuwe post",
		Body: `"Soep" is gepubliceerd`, URL: "/posts/1", PostID: 1,
	})

	waitForStream(t, rec, "event: notification\n", time.Second)
	waitForStream(t, rec, `"post_id":1`, time.Second)
	waitForStream(t, rec, `"title":"Nieuwe post"`, time.Second)

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stream handler did not return after the request context ended")
	}
}

func TestVapidSubscriber(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	tests := []struct {
		name    string
		subject string
		from    string
		want    string
	}{
		{name: "explicit subject", subject: "chef@example.com", want: "chef@example.com"},
		{name: "explicit mailto is stripped", subject: "mailto:chef@example.com", want: "chef@example.com"},
		{name: "https subject kept", subject: "https://example.com/contact", want: "https://example.com/contact"},
		{name: "falls back to from address", from: "smtp@example.com", want: "smtp@example.com"},
		{name: "final fallback", want: "admin@redchef.local"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := db.UpdateEmailSettings("", 587, "", "", tt.from, "tls", "", ""); err != nil {
				t.Fatalf("UpdateEmailSettings: %v", err)
			}
			got := vapidSubscriber(&db.NotificationSettings{VAPIDSubject: tt.subject})
			if got != tt.want {
				t.Fatalf("vapidSubscriber = %q, want %q", got, tt.want)
			}
		})
	}

	// nil settings must not panic.
	if got := vapidSubscriber(nil); got == "" {
		t.Fatal("vapidSubscriber(nil) returned an empty subject")
	}
}

func TestWebPushSendersPruneGoneSubscriptions(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	user, err := db.GetUserByUsername("admin")
	if err != nil {
		user, err = db.GetUserByEmail("admin")
		if err != nil {
			t.Fatalf("get admin: %v", err)
		}
	}

	if err := db.SaveWebPushSubscription(user.ID, "https://push.example.com/gone", "p", "a", ""); err != nil {
		t.Fatalf("SaveWebPushSubscription: %v", err)
	}

	original := sendWebPush
	defer func() { sendWebPush = original }()

	calls := 0
	sendWebPush = func(sub db.WebPushSubscription, payload []byte, settings *db.NotificationSettings) (int, error) {
		calls++
		return http.StatusGone, nil
	}

	settings := &db.NotificationSettings{VAPIDPublicKey: "pub", VAPIDPrivateKey: "priv"}
	sent := sendWebPushToUser(settings, user.ID, NotificationEvent{ID: "x"})
	if sent != 0 {
		t.Fatalf("sent = %d, want 0 for a gone endpoint", sent)
	}
	if calls != 1 {
		t.Fatalf("sender called %d times, want 1", calls)
	}
	if subs, _ := db.ListWebPushSubscriptions(user.ID); len(subs) != 0 {
		t.Fatalf("gone subscription was not pruned: %+v", subs)
	}
}
