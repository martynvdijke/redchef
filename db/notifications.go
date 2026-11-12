package db

import (
	"database/sql"
	"time"
)

// NotificationSettings is the singleton admin configuration row for the
// notification feature. Boolean columns are stored as INTEGER 0/1.
type NotificationSettings struct {
	ID              int64     `json:"id"`
	Enabled         bool      `json:"enabled"`
	NewPostEnabled  bool      `json:"new_post_enabled"`
	WebPushEnabled  bool      `json:"web_push_enabled"`
	FCMEnabled      bool      `json:"fcm_enabled"`
	VAPIDPublicKey  string    `json:"vapid_public_key"`
	VAPIDPrivateKey string    `json:"vapid_private_key"`
	VAPIDSubject    string    `json:"vapid_subject"`
	FCMProjectID    string    `json:"fcm_project_id"`
	FCMClientEmail  string    `json:"fcm_client_email"`
	FCMPrivateKey   string    `json:"fcm_private_key"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// DefaultNotificationSettings is what a fresh install exposes before an admin
// changes anything: in-app notifications on, external transports off.
func DefaultNotificationSettings() NotificationSettings {
	return NotificationSettings{ID: 1, Enabled: true, NewPostEnabled: true}
}

// GetNotificationSettings returns the singleton settings row. When the row is
// missing (for example a database created before this feature) it returns the
// defaults instead of failing, so the public config endpoint keeps working.
func GetNotificationSettings() (*NotificationSettings, error) {
	s := &NotificationSettings{}
	var enabled, newPost, webPush, fcm int
	var updatedAt string
	err := DB.QueryRow(
		`SELECT id, enabled, new_post_enabled, web_push_enabled, fcm_enabled,
		        vapid_public_key, vapid_private_key, vapid_subject,
		        fcm_project_id, fcm_client_email, fcm_private_key, updated_at
		 FROM notification_settings WHERE id = 1`,
	).Scan(&s.ID, &enabled, &newPost, &webPush, &fcm,
		&s.VAPIDPublicKey, &s.VAPIDPrivateKey, &s.VAPIDSubject,
		&s.FCMProjectID, &s.FCMClientEmail, &s.FCMPrivateKey, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			d := DefaultNotificationSettings()
			return &d, nil
		}
		return nil, err
	}
	s.Enabled = intToBool(enabled)
	s.NewPostEnabled = intToBool(newPost)
	s.WebPushEnabled = intToBool(webPush)
	s.FCMEnabled = intToBool(fcm)
	s.UpdatedAt, _ = parseTime(updatedAt)
	return s, nil
}

// UpdateNotificationSettings persists the singleton row.
func UpdateNotificationSettings(s NotificationSettings) error {
	_, err := DB.Exec(
		`UPDATE notification_settings SET
			enabled = ?, new_post_enabled = ?, web_push_enabled = ?, fcm_enabled = ?,
			vapid_public_key = ?, vapid_private_key = ?, vapid_subject = ?,
			fcm_project_id = ?, fcm_client_email = ?, fcm_private_key = ?,
			updated_at = datetime('now')
		 WHERE id = 1`,
		boolToInt(s.Enabled), boolToInt(s.NewPostEnabled),
		boolToInt(s.WebPushEnabled), boolToInt(s.FCMEnabled),
		s.VAPIDPublicKey, s.VAPIDPrivateKey, s.VAPIDSubject,
		s.FCMProjectID, s.FCMClientEmail, s.FCMPrivateKey,
	)
	return err
}

// NotificationPrefs are a single user's opt-in choices.
type NotificationPrefs struct {
	UserID    int64     `json:"user_id"`
	NewPosts  bool      `json:"new_posts"`
	Email     bool      `json:"email"`
	Push      bool      `json:"push"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DefaultNotificationPrefs is the all-on default used when a user has never
// saved preferences.
func DefaultNotificationPrefs(userID int64) NotificationPrefs {
	return NotificationPrefs{UserID: userID, NewPosts: true, Email: true, Push: true}
}

// GetNotificationPrefs returns a user's preferences, falling back to the
// all-on defaults when no row exists.
func GetNotificationPrefs(userID int64) (*NotificationPrefs, error) {
	p := &NotificationPrefs{UserID: userID}
	var newPosts, email, push int
	var updatedAt string
	err := DB.QueryRow(
		"SELECT user_id, new_posts, email, push, updated_at FROM notification_prefs WHERE user_id = ?",
		userID,
	).Scan(&p.UserID, &newPosts, &email, &push, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			d := DefaultNotificationPrefs(userID)
			return &d, nil
		}
		return nil, err
	}
	p.NewPosts = intToBool(newPosts)
	p.Email = intToBool(email)
	p.Push = intToBool(push)
	p.UpdatedAt, _ = parseTime(updatedAt)
	return p, nil
}

// UpsertNotificationPrefs creates or replaces a user's preferences.
func UpsertNotificationPrefs(p *NotificationPrefs) error {
	if p == nil {
		return sql.ErrNoRows
	}
	_, err := DB.Exec(
		`INSERT INTO notification_prefs (user_id, new_posts, email, push, updated_at)
		 VALUES (?, ?, ?, ?, datetime('now'))
		 ON CONFLICT(user_id) DO UPDATE SET
			new_posts = excluded.new_posts,
			email = excluded.email,
			push = excluded.push,
			updated_at = datetime('now')`,
		p.UserID, boolToInt(p.NewPosts), boolToInt(p.Email), boolToInt(p.Push),
	)
	return err
}

// SaveDeviceToken records an FCM/APNS device token for a user. A token is
// unique across the table: saving it again reassigns it to the newest user,
// which is what happens when a device switches accounts.
func SaveDeviceToken(userID int64, token, platform string) error {
	if platform == "" {
		platform = "android"
	}
	_, err := DB.Exec(
		`INSERT INTO device_tokens (user_id, token, platform, updated_at)
		 VALUES (?, ?, ?, datetime('now'))
		 ON CONFLICT(token) DO UPDATE SET
			user_id = excluded.user_id,
			platform = excluded.platform,
			updated_at = datetime('now')`,
		userID, token, platform,
	)
	return err
}

// DeleteDeviceToken removes one token, but only when it belongs to userID.
func DeleteDeviceToken(userID int64, token string) error {
	_, err := DB.Exec("DELETE FROM device_tokens WHERE user_id = ? AND token = ?", userID, token)
	return err
}

// DeleteDeviceTokensForUser removes every device token of a user.
func DeleteDeviceTokensForUser(userID int64) error {
	_, err := DB.Exec("DELETE FROM device_tokens WHERE user_id = ?", userID)
	return err
}

// ListDeviceTokens returns the device tokens registered for a user.
func ListDeviceTokens(userID int64) ([]string, error) {
	rows, err := DB.Query("SELECT token FROM device_tokens WHERE user_id = ? ORDER BY id", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}

// WebPushSubscription is one browser push endpoint.
type WebPushSubscription struct {
	ID       int64  `json:"id"`
	UserID   int64  `json:"user_id"`
	Endpoint string `json:"endpoint"`
	P256dh   string `json:"p256dh"`
	Auth     string `json:"auth"`
}

// SaveWebPushSubscription records an endpoint. Endpoints are unique: the same
// browser re-subscribing refreshes its keys instead of creating duplicates.
func SaveWebPushSubscription(userID int64, endpoint, p256dh, auth, userAgent string) error {
	_, err := DB.Exec(
		`INSERT INTO web_push_subscriptions (user_id, endpoint, p256dh, auth, user_agent, updated_at)
		 VALUES (?, ?, ?, ?, ?, datetime('now'))
		 ON CONFLICT(endpoint) DO UPDATE SET
			user_id = excluded.user_id,
			p256dh = excluded.p256dh,
			auth = excluded.auth,
			user_agent = excluded.user_agent,
			updated_at = datetime('now')`,
		userID, endpoint, p256dh, auth, userAgent,
	)
	return err
}

// DeleteWebPushSubscription removes one endpoint, but only when it belongs to
// userID.
func DeleteWebPushSubscription(userID int64, endpoint string) error {
	_, err := DB.Exec("DELETE FROM web_push_subscriptions WHERE user_id = ? AND endpoint = ?", userID, endpoint)
	return err
}

// ListWebPushSubscriptions returns a user's push endpoints.
func ListWebPushSubscriptions(userID int64) ([]WebPushSubscription, error) {
	rows, err := DB.Query(
		"SELECT id, user_id, endpoint, p256dh, auth FROM web_push_subscriptions WHERE user_id = ? ORDER BY id",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []WebPushSubscription
	for rows.Next() {
		var s WebPushSubscription
		if err := rows.Scan(&s.ID, &s.UserID, &s.Endpoint, &s.P256dh, &s.Auth); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

// WebPushSubscriptionByEndpoint looks a subscription up without knowing its
// owner. Returns sql.ErrNoRows when the endpoint is unknown.
func WebPushSubscriptionByEndpoint(endpoint string) (*WebPushSubscription, error) {
	s := &WebPushSubscription{}
	err := DB.QueryRow(
		"SELECT id, user_id, endpoint, p256dh, auth FROM web_push_subscriptions WHERE endpoint = ?",
		endpoint,
	).Scan(&s.ID, &s.UserID, &s.Endpoint, &s.P256dh, &s.Auth)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool {
	return i != 0
}
