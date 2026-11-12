package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"redchef/db"
)

// ── Event construction ──

// NewPostNotificationEvent builds the event every transport carries for a
// published post. User-facing copy is Dutch, matching the rest of the app.
func NewPostNotificationEvent(post db.Post) NotificationEvent {
	return NotificationEvent{
		ID:        fmt.Sprintf("np_%d_%d", post.ID, time.Now().Unix()),
		Type:      "new_post",
		Title:     "Nieuwe post",
		Body:      fmt.Sprintf("\"%s\" is gepubliceerd", post.Title),
		URL:       fmt.Sprintf("/posts/%d", post.ID),
		PostID:    post.ID,
		CreatedAt: time.Now().UTC(),
	}
}

// PushNewPostNotification fans a new-post event out to every member except the
// creator, over the in-app hub, Web Push and FCM. It is designed to be called
// from a goroutine: nothing here may block post creation, and each user is
// handled independently.
func PushNewPostNotification(post db.Post, creatorUserID int64) {
	settings, err := db.GetNotificationSettings()
	if err != nil {
		log.Printf("[notifications] load settings: %v", err)
		return
	}
	if !settings.Enabled || !settings.NewPostEnabled {
		return
	}

	ev := NewPostNotificationEvent(post)

	users, err := db.ListUsers()
	if err != nil {
		log.Printf("[notifications] list users: %v", err)
		return
	}

	for _, u := range users {
		if u.ID == creatorUserID {
			continue
		}
		if isDemoNotificationTarget(u.Email) {
			continue
		}
		go notifyUser(settings, u.ID, ev)
	}
}

// notifyUser delivers one event to one user according to their preferences.
func notifyUser(settings *db.NotificationSettings, userID int64, ev NotificationEvent) {
	prefs, err := db.GetNotificationPrefs(userID)
	if err != nil {
		log.Printf("[notifications] prefs for user %d: %v", userID, err)
		return
	}

	if prefs.NewPosts {
		GlobalNotificationHub().Publish(userID, ev)
	}
	if !prefs.Push {
		return
	}
	if settings.WebPushEnabled {
		sendWebPushToUser(settings, userID, ev)
	}
	if settings.FCMEnabled {
		sendFCMToUser(settings, userID, ev)
	}
}

// ── Public config ──

// GetNotificationConfig exposes the feature switches and the VAPID public key.
// It never returns secrets, so it is safe to call without authentication.
func GetNotificationConfig(w http.ResponseWriter, r *http.Request) {
	settings, err := db.GetNotificationSettings()
	if err != nil {
		jsonError(w, "failed to get notification settings", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled":          settings.Enabled,
		"new_post_enabled": settings.NewPostEnabled,
		"web_push_enabled": settings.WebPushEnabled,
		"fcm_enabled":      settings.FCMEnabled,
		"vapid_public_key": settings.VAPIDPublicKey,
	})
}

// ── Per-user preferences ──

func GetNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	prefs, err := db.GetNotificationPrefs(getUserID(r))
	if err != nil {
		jsonError(w, "failed to get preferences", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prefs)
}

func UpdateNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NewPosts *bool `json:"new_posts"`
		Email    *bool `json:"email"`
		Push     *bool `json:"push"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	userID := getUserID(r)
	prefs, err := db.GetNotificationPrefs(userID)
	if err != nil {
		jsonError(w, "failed to get preferences", http.StatusInternalServerError)
		return
	}
	// PUT is a full replace, but absent fields keep their current value so a
	// partial client cannot accidentally switch everything off.
	if req.NewPosts != nil {
		prefs.NewPosts = *req.NewPosts
	}
	if req.Email != nil {
		prefs.Email = *req.Email
	}
	if req.Push != nil {
		prefs.Push = *req.Push
	}
	if err := db.UpsertNotificationPrefs(prefs); err != nil {
		jsonError(w, "failed to save preferences", http.StatusInternalServerError)
		return
	}

	saved, err := db.GetNotificationPrefs(userID)
	if err != nil {
		jsonError(w, "failed to get preferences", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(saved)
}

// ── SSE stream ──

// NotificationStream keeps a text/event-stream connection open for the signed
// in user and forwards hub events as they happen.
func NotificationStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	events, unsubscribe := GlobalNotificationHub().Subscribe(getUserID(r))
	defer unsubscribe()

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-events:
			data, err := json.Marshal(ev)
			if err != nil {
				log.Printf("[notifications] marshal event: %v", err)
				continue
			}
			fmt.Fprintf(w, "event: notification\ndata: %s\n\n", data)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(w, "event: ping\ndata: {}\n\n")
			flusher.Flush()
		}
	}
}

// ── Device tokens ──

func RegisterDeviceToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Platform string `json:"platform"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	req.Token = strings.TrimSpace(req.Token)
	if req.Token == "" {
		jsonError(w, "token is required", http.StatusBadRequest)
		return
	}
	if err := db.SaveDeviceToken(getUserID(r), req.Token, req.Platform); err != nil {
		jsonError(w, "failed to save device token", http.StatusInternalServerError)
		return
	}
	writeOK(w)
}

func UnregisterDeviceToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	req.Token = strings.TrimSpace(req.Token)
	if req.Token == "" {
		jsonError(w, "token is required", http.StatusBadRequest)
		return
	}
	if err := db.DeleteDeviceToken(getUserID(r), req.Token); err != nil {
		jsonError(w, "failed to delete device token", http.StatusInternalServerError)
		return
	}
	writeOK(w)
}

// ── Web Push subscriptions ──

func SubscribeWebPush(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	req.Endpoint = strings.TrimSpace(req.Endpoint)
	p256dh := strings.TrimSpace(req.Keys.P256dh)
	auth := strings.TrimSpace(req.Keys.Auth)
	if req.Endpoint == "" || p256dh == "" || auth == "" {
		jsonError(w, "endpoint, p256dh and auth are required", http.StatusBadRequest)
		return
	}
	if err := db.SaveWebPushSubscription(getUserID(r), req.Endpoint, p256dh, auth, r.UserAgent()); err != nil {
		jsonError(w, "failed to save subscription", http.StatusInternalServerError)
		return
	}
	writeOK(w)
}

func UnsubscribeWebPush(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	req.Endpoint = strings.TrimSpace(req.Endpoint)
	if req.Endpoint == "" {
		jsonError(w, "endpoint is required", http.StatusBadRequest)
		return
	}
	if err := db.DeleteWebPushSubscription(getUserID(r), req.Endpoint); err != nil {
		jsonError(w, "failed to delete subscription", http.StatusInternalServerError)
		return
	}
	writeOK(w)
}

// ── Admin settings ──

// NotificationSettingsRequest mirrors db.NotificationSettings without the ID
// and timestamp, which the server owns.
type NotificationSettingsRequest struct {
	Enabled         bool   `json:"enabled"`
	NewPostEnabled  bool   `json:"new_post_enabled"`
	WebPushEnabled  bool   `json:"web_push_enabled"`
	FCMEnabled      bool   `json:"fcm_enabled"`
	VAPIDPublicKey  string `json:"vapid_public_key"`
	VAPIDPrivateKey string `json:"vapid_private_key"`
	VAPIDSubject    string `json:"vapid_subject"`
	FCMProjectID    string `json:"fcm_project_id"`
	FCMClientEmail  string `json:"fcm_client_email"`
	FCMPrivateKey   string `json:"fcm_private_key"`
}

func AdminGetNotificationSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := db.GetNotificationSettings()
	if err != nil {
		jsonError(w, "failed to get notification settings", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

func AdminUpdateNotificationSettings(w http.ResponseWriter, r *http.Request) {
	var req NotificationSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	settings := db.NotificationSettings{
		ID:              1,
		Enabled:         req.Enabled,
		NewPostEnabled:  req.NewPostEnabled,
		WebPushEnabled:  req.WebPushEnabled,
		FCMEnabled:      req.FCMEnabled,
		VAPIDPublicKey:  strings.TrimSpace(req.VAPIDPublicKey),
		VAPIDPrivateKey: strings.TrimSpace(req.VAPIDPrivateKey),
		VAPIDSubject:    strings.TrimSpace(req.VAPIDSubject),
		FCMProjectID:    strings.TrimSpace(req.FCMProjectID),
		FCMClientEmail:  strings.TrimSpace(req.FCMClientEmail),
		FCMPrivateKey:   strings.TrimSpace(req.FCMPrivateKey),
	}
	// A transport cannot be enabled without the credentials it needs; refusing
	// to store an impossible combination keeps the fanout code honest.
	if settings.VAPIDPublicKey == "" || settings.VAPIDPrivateKey == "" {
		settings.WebPushEnabled = false
	}
	if settings.FCMProjectID == "" || settings.FCMClientEmail == "" || settings.FCMPrivateKey == "" {
		settings.FCMEnabled = false
	}

	if err := db.UpdateNotificationSettings(settings); err != nil {
		jsonError(w, "failed to save notification settings", http.StatusInternalServerError)
		return
	}

	saved, err := db.GetNotificationSettings()
	if err != nil {
		jsonError(w, "failed to get notification settings", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(saved)
}

// AdminGenerateVAPIDKeys creates and stores a keypair, replacing any previous
// one. Existing browser subscriptions keep working because the VAPID keypair
// only signs requests, it is not part of the subscription.
func AdminGenerateVAPIDKeys(w http.ResponseWriter, r *http.Request) {
	privateKey, publicKey, err := GenerateVAPIDKeys()
	if err != nil {
		jsonError(w, fmt.Sprintf("failed to generate VAPID keys: %v", err), http.StatusInternalServerError)
		return
	}

	settings, err := db.GetNotificationSettings()
	if err != nil {
		jsonError(w, "failed to get notification settings", http.StatusInternalServerError)
		return
	}
	settings.VAPIDPublicKey = publicKey
	settings.VAPIDPrivateKey = privateKey
	if err := db.UpdateNotificationSettings(*settings); err != nil {
		jsonError(w, "failed to save VAPID keys", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"vapid_public_key":  publicKey,
		"vapid_private_key": privateKey,
	})
}

// AdminTestNotification delivers a test event over every configured transport
// to the admin's own subscriptions and returns how many sends succeeded.
func AdminTestNotification(w http.ResponseWriter, r *http.Request) {
	settings, err := db.GetNotificationSettings()
	if err != nil {
		jsonError(w, "failed to get notification settings", http.StatusInternalServerError)
		return
	}

	userID := getUserID(r)
	ev := NotificationEvent{
		ID:        fmt.Sprintf("test_%d", time.Now().Unix()),
		Type:      "test",
		Title:     "Testnotificatie",
		Body:      "Dit is een testnotificatie van Red Copper Chef.",
		URL:       "/",
		CreatedAt: time.Now().UTC(),
	}

	// The in-app event is always "sent": publishing is non-blocking and a
	// browser with no open stream simply misses it.
	GlobalNotificationHub().Publish(userID, ev)

	webPush := 0
	if settings.WebPushEnabled {
		webPush = sendWebPushToUser(settings, userID, ev)
	}
	android := 0
	if settings.FCMEnabled {
		android = sendFCMToUser(settings, userID, ev)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"realtime": 1,
		"web_push": webPush,
		"android":  android,
	})
}

func writeOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}
