package db

import (
	"testing"
)

func TestNotificationSettingsRoundTrip(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	got, err := GetNotificationSettings()
	if err != nil {
		t.Fatalf("GetNotificationSettings: %v", err)
	}
	if !got.Enabled || !got.NewPostEnabled {
		t.Fatalf("fresh settings should have in-app notifications on, got %+v", got)
	}
	if got.WebPushEnabled || got.FCMEnabled {
		t.Fatalf("fresh settings should have external transports off, got %+v", got)
	}

	want := NotificationSettings{
		ID:              1,
		Enabled:         true,
		NewPostEnabled:  false,
		WebPushEnabled:  true,
		FCMEnabled:      true,
		VAPIDPublicKey:  "pub-key",
		VAPIDPrivateKey: "priv-key",
		VAPIDSubject:    "mailto:chef@example.com",
		FCMProjectID:    "project-1",
		FCMClientEmail:  "svc@project-1.iam.gserviceaccount.com",
		FCMPrivateKey:   "-----BEGIN PRIVATE KEY-----\\nabc\\n-----END PRIVATE KEY-----",
	}
	if err := UpdateNotificationSettings(want); err != nil {
		t.Fatalf("UpdateNotificationSettings: %v", err)
	}

	got, err = GetNotificationSettings()
	if err != nil {
		t.Fatalf("GetNotificationSettings after update: %v", err)
	}
	if got.ID != want.ID ||
		got.Enabled != want.Enabled ||
		got.NewPostEnabled != want.NewPostEnabled ||
		got.WebPushEnabled != want.WebPushEnabled ||
		got.FCMEnabled != want.FCMEnabled ||
		got.VAPIDPublicKey != want.VAPIDPublicKey ||
		got.VAPIDPrivateKey != want.VAPIDPrivateKey ||
		got.VAPIDSubject != want.VAPIDSubject ||
		got.FCMProjectID != want.FCMProjectID ||
		got.FCMClientEmail != want.FCMClientEmail ||
		got.FCMPrivateKey != want.FCMPrivateKey {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", got, want)
	}
}

func TestNotificationSettingsDefaultsWhenRowMissing(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	if _, err := DB.Exec("DELETE FROM notification_settings"); err != nil {
		t.Fatalf("delete settings row: %v", err)
	}

	got, err := GetNotificationSettings()
	if err != nil {
		t.Fatalf("GetNotificationSettings without row: %v", err)
	}
	if got == nil || got.ID != 1 || !got.Enabled || !got.NewPostEnabled {
		t.Fatalf("expected defaults when the row is missing, got %+v", got)
	}
}

func TestNotificationPrefsDefaults(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	userID, err := CreateUser("prefs@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	prefs, err := GetNotificationPrefs(userID)
	if err != nil {
		t.Fatalf("GetNotificationPrefs: %v", err)
	}
	if prefs.UserID != userID {
		t.Fatalf("user_id = %d, want %d", prefs.UserID, userID)
	}
	if !prefs.NewPosts || !prefs.Email || !prefs.Push {
		t.Fatalf("defaults should be all true, got %+v", prefs)
	}

	// Reading defaults must not create a row.
	var count int
	if err := DB.QueryRow("SELECT COUNT(*) FROM notification_prefs WHERE user_id = ?", userID).Scan(&count); err != nil {
		t.Fatalf("count prefs: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no stored prefs, got %d rows", count)
	}
}

func TestNotificationPrefsUpsert(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	userID, err := CreateUser("prefs-upsert@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	tests := []struct {
		name     string
		newPosts bool
		email    bool
		push     bool
	}{
		{"all off", false, false, false},
		{"mixed", true, false, true},
		{"all on", true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := UpsertNotificationPrefs(&NotificationPrefs{
				UserID: userID, NewPosts: tt.newPosts, Email: tt.email, Push: tt.push,
			}); err != nil {
				t.Fatalf("UpsertNotificationPrefs: %v", err)
			}

			got, err := GetNotificationPrefs(userID)
			if err != nil {
				t.Fatalf("GetNotificationPrefs: %v", err)
			}
			if got.NewPosts != tt.newPosts || got.Email != tt.email || got.Push != tt.push {
				t.Fatalf("got %+v, want new_posts=%v email=%v push=%v",
					got, tt.newPosts, tt.email, tt.push)
			}

			var count int
			if err := DB.QueryRow("SELECT COUNT(*) FROM notification_prefs WHERE user_id = ?", userID).Scan(&count); err != nil {
				t.Fatalf("count prefs: %v", err)
			}
			if count != 1 {
				t.Fatalf("upsert created %d rows, want 1", count)
			}
		})
	}
}

func TestDeviceTokenUpsertAndDelete(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	first, err := CreateUser("device-one@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser first: %v", err)
	}
	second, err := CreateUser("device-two@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser second: %v", err)
	}

	if err := SaveDeviceToken(first, "token-1", "android"); err != nil {
		t.Fatalf("SaveDeviceToken: %v", err)
	}
	if err := SaveDeviceToken(first, "token-2", ""); err != nil {
		t.Fatalf("SaveDeviceToken default platform: %v", err)
	}

	tokens, err := ListDeviceTokens(first)
	if err != nil {
		t.Fatalf("ListDeviceTokens: %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("got %d tokens, want 2 (%v)", len(tokens), tokens)
	}

	// Saving the same token under another user moves it, and updates platform.
	if err := SaveDeviceToken(second, "token-1", "ios"); err != nil {
		t.Fatalf("SaveDeviceToken reassign: %v", err)
	}
	firstTokens, _ := ListDeviceTokens(first)
	if len(firstTokens) != 1 || firstTokens[0] != "token-2" {
		t.Fatalf("first user tokens = %v, want [token-2]", firstTokens)
	}
	secondTokens, _ := ListDeviceTokens(second)
	if len(secondTokens) != 1 || secondTokens[0] != "token-1" {
		t.Fatalf("second user tokens = %v, want [token-1]", secondTokens)
	}
	var platform string
	if err := DB.QueryRow("SELECT platform FROM device_tokens WHERE token = ?", "token-1").Scan(&platform); err != nil {
		t.Fatalf("query platform: %v", err)
	}
	if platform != "ios" {
		t.Fatalf("platform = %q, want ios", platform)
	}

	// A user cannot delete someone else's token.
	if err := DeleteDeviceToken(first, "token-1"); err != nil {
		t.Fatalf("DeleteDeviceToken wrong owner: %v", err)
	}
	if tokens, _ := ListDeviceTokens(second); len(tokens) != 1 {
		t.Fatalf("wrong-owner delete removed the token: %v", tokens)
	}

	if err := DeleteDeviceToken(first, "token-2"); err != nil {
		t.Fatalf("DeleteDeviceToken: %v", err)
	}
	if tokens, _ := ListDeviceTokens(first); len(tokens) != 0 {
		t.Fatalf("tokens after delete = %v, want none", tokens)
	}

	if err := DeleteDeviceTokensForUser(second); err != nil {
		t.Fatalf("DeleteDeviceTokensForUser: %v", err)
	}
	if tokens, _ := ListDeviceTokens(second); len(tokens) != 0 {
		t.Fatalf("tokens after bulk delete = %v, want none", tokens)
	}
}

func TestWebPushSubscriptionUpsertAndDelete(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	first, err := CreateUser("push-one@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser first: %v", err)
	}
	second, err := CreateUser("push-two@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser second: %v", err)
	}

	const endpoint = "https://push.example.com/abc"
	if err := SaveWebPushSubscription(first, endpoint, "p256dh-1", "auth-1", "UA/1"); err != nil {
		t.Fatalf("SaveWebPushSubscription: %v", err)
	}
	// Same endpoint again under another user refreshes the row.
	if err := SaveWebPushSubscription(second, endpoint, "p256dh-2", "auth-2", "UA/2"); err != nil {
		t.Fatalf("SaveWebPushSubscription upsert: %v", err)
	}

	firstSubs, err := ListWebPushSubscriptions(first)
	if err != nil {
		t.Fatalf("ListWebPushSubscriptions: %v", err)
	}
	if len(firstSubs) != 0 {
		t.Fatalf("endpoint should have moved to the second user, got %v", firstSubs)
	}

	secondSubs, err := ListWebPushSubscriptions(second)
	if err != nil {
		t.Fatalf("ListWebPushSubscriptions: %v", err)
	}
	if len(secondSubs) != 1 {
		t.Fatalf("got %d subscriptions, want 1", len(secondSubs))
	}
	if secondSubs[0].P256dh != "p256dh-2" || secondSubs[0].Auth != "auth-2" {
		t.Fatalf("upsert did not refresh keys: %+v", secondSubs[0])
	}

	byEndpoint, err := WebPushSubscriptionByEndpoint(endpoint)
	if err != nil {
		t.Fatalf("WebPushSubscriptionByEndpoint: %v", err)
	}
	if byEndpoint.UserID != second {
		t.Fatalf("subscription user = %d, want %d", byEndpoint.UserID, second)
	}

	// A user cannot delete someone else's subscription.
	if err := DeleteWebPushSubscription(first, endpoint); err != nil {
		t.Fatalf("DeleteWebPushSubscription wrong owner: %v", err)
	}
	if subs, _ := ListWebPushSubscriptions(second); len(subs) != 1 {
		t.Fatalf("wrong-owner delete removed the subscription")
	}

	if err := DeleteWebPushSubscription(second, endpoint); err != nil {
		t.Fatalf("DeleteWebPushSubscription: %v", err)
	}
	if subs, _ := ListWebPushSubscriptions(second); len(subs) != 0 {
		t.Fatalf("subscriptions after delete = %v, want none", subs)
	}
	if _, err := WebPushSubscriptionByEndpoint(endpoint); err == nil {
		t.Fatal("expected endpoint lookup to fail after delete")
	}
}

func TestDeleteUserRemovesNotificationRows(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	adminID, err := CreateUser("admin@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser admin: %v", err)
	}
	if adminID == 0 {
		t.Fatal("admin id should not be zero")
	}
	userID, err := CreateUser("leaving@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if err := UpsertNotificationPrefs(&NotificationPrefs{UserID: userID, NewPosts: true, Email: false, Push: true}); err != nil {
		t.Fatalf("UpsertNotificationPrefs: %v", err)
	}
	if err := SaveDeviceToken(userID, "leaving-token", "android"); err != nil {
		t.Fatalf("SaveDeviceToken: %v", err)
	}
	if err := SaveWebPushSubscription(userID, "https://push.example.com/leaving", "p", "a", ""); err != nil {
		t.Fatalf("SaveWebPushSubscription: %v", err)
	}

	if err := DeleteUser(userID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	for _, table := range []string{"notification_prefs", "device_tokens", "web_push_subscriptions"} {
		var count int
		if err := DB.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE user_id = ?", userID).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Errorf("%s still has %d rows for the deleted user", table, count)
		}
	}
}
