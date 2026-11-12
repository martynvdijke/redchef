package handlers

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"redchef/db"
)

// FCM HTTP v1 is called directly: a service-account JWT is signed with
// crypto/rsa and exchanged for a cached OAuth access token. No Firebase SDK is
// involved, which keeps the binary small and the dependency tree unchanged.

const (
	fcmScope            = "https://www.googleapis.com/auth/firebase.messaging"
	fcmTokenCacheWindow = 50 * time.Minute
)

// Variables so tests can point the client at a local server.
var (
	fcmTokenEndpoint = "https://oauth2.googleapis.com/token"
	fcmSendEndpoint  = "https://fcm.googleapis.com/v1/projects/%s/messages:send"
	fcmHTTPClient    = &http.Client{Timeout: 15 * time.Second}
)

// fcmAccessTokenCache stores the last exchanged OAuth token. The fingerprint
// ties a cached token to the service account that produced it, so changing the
// admin settings cannot reuse a token for the previous account.
type fcmAccessTokenCache struct {
	mu          sync.Mutex
	fingerprint string
	token       string
	expires     time.Time
}

var fcmTokenCache fcmAccessTokenCache

// fcmResult is the per-token outcome of one FCM send.
type fcmResult struct {
	Token string
	OK    bool
	Gone  bool // token is no longer valid and should be pruned
	Err   error
}

type fcmClaims struct {
	Iss   string `json:"iss"`
	Scope string `json:"scope"`
	Aud   string `json:"aud"`
	Iat   int64  `json:"iat"`
	Exp   int64  `json:"exp"`
}

type fcmMessage struct {
	Message struct {
		Token        string            `json:"token"`
		Notification map[string]string `json:"notification"`
		Data         map[string]string `json:"data"`
		Android      map[string]string `json:"android"`
	} `json:"message"`
}

type fcmErrorResponse struct {
	Error struct {
		Status  string `json:"status"`
		Details []struct {
			ErrorCode string `json:"errorCode"`
		} `json:"details"`
	} `json:"error"`
}

// normalizePEM turns the escaped newlines that end up in JSON/env-stored keys
// into real PEM newlines.
func normalizePEM(key string) string {
	key = strings.TrimSpace(key)
	key = strings.ReplaceAll(key, `\n`, "\n")
	return key
}

// parseServiceAccountKey decodes a PKCS#8 (or PKCS#1) RSA private key.
func parseServiceAccountKey(pemKey string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(normalizePEM(pemKey)))
	if block == nil {
		return nil, errors.New("fcm: private key is not valid PEM")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("fcm: private key is not RSA")
		}
		return rsaKey, nil
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, errors.New("fcm: cannot parse private key")
}

// buildFCMJWT creates the RS256 service-account assertion.
func buildFCMJWT(clientEmail, pemKey string, now time.Time) (string, error) {
	if strings.TrimSpace(clientEmail) == "" {
		return "", errors.New("fcm: client email is not configured")
	}
	key, err := parseServiceAccountKey(pemKey)
	if err != nil {
		return "", err
	}

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims, err := json.Marshal(fcmClaims{
		Iss:   clientEmail,
		Scope: fcmScope,
		Aud:   fcmTokenEndpoint,
		Iat:   now.Unix(),
		Exp:   now.Add(time.Hour).Unix(),
	})
	if err != nil {
		return "", err
	}
	signingInput := header + "." + base64.RawURLEncoding.EncodeToString(claims)

	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// fcmAccessToken returns a cached OAuth access token, exchanging a fresh JWT
// when the cache is empty, stale or belongs to a different service account.
func fcmAccessToken(s *db.NotificationSettings, now time.Time) (string, error) {
	if s == nil {
		return "", errors.New("fcm: settings are not loaded")
	}
	fingerprint := fmt.Sprintf("%s|%x", s.FCMClientEmail, sha256.Sum256([]byte(s.FCMPrivateKey)))

	fcmTokenCache.mu.Lock()
	defer fcmTokenCache.mu.Unlock()

	if fcmTokenCache.token != "" && fcmTokenCache.fingerprint == fingerprint && now.Before(fcmTokenCache.expires) {
		return fcmTokenCache.token, nil
	}

	assertion, err := buildFCMJWT(s.FCMClientEmail, s.FCMPrivateKey, now)
	if err != nil {
		return "", err
	}

	form := bytes.NewBufferString("grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer&assertion=" + assertion)
	req, err := http.NewRequest(http.MethodPost, fcmTokenEndpoint, form)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := fcmHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fcm: token exchange failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("fcm: decode token response: %w", err)
	}
	if payload.AccessToken == "" {
		return "", errors.New("fcm: token response had no access_token")
	}

	ttl := fcmTokenCacheWindow
	if payload.ExpiresIn > 0 {
		if d := time.Duration(payload.ExpiresIn)*time.Second - 10*time.Minute; d > 0 && d < ttl {
			ttl = d
		}
	}
	fcmTokenCache.token = payload.AccessToken
	fcmTokenCache.fingerprint = fingerprint
	fcmTokenCache.expires = now.Add(ttl)
	return payload.AccessToken, nil
}

// sendFCMToToken delivers one notification to one device token using an
// already-obtained access token.
func sendFCMToToken(s *db.NotificationSettings, accessToken, token string, ev NotificationEvent) fcmResult {
	res := fcmResult{Token: token}
	if s == nil || strings.TrimSpace(s.FCMProjectID) == "" {
		res.Err = errors.New("fcm: project id is not configured")
		return res
	}

	var msg fcmMessage
	msg.Message.Token = token
	msg.Message.Notification = map[string]string{"title": ev.Title, "body": ev.Body}
	msg.Message.Data = map[string]string{
		"type":    ev.Type,
		"post_id": fmt.Sprintf("%d", ev.PostID),
		"url":     ev.URL,
	}
	msg.Message.Android = map[string]string{"priority": "high"}

	body, err := json.Marshal(msg)
	if err != nil {
		res.Err = err
		return res
	}

	url := fmt.Sprintf(fcmSendEndpoint, s.FCMProjectID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		res.Err = err
		return res
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := fcmHTTPClient.Do(req)
	if err != nil {
		res.Err = err
		return res
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		res.OK = true
		return res
	}

	var apiErr fcmErrorResponse
	_ = json.Unmarshal(respBody, &apiErr)
	res.Gone = fcmIsGone(resp.StatusCode, apiErr)
	res.Err = fmt.Errorf("fcm: send failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	return res
}

// fcmIsGone reports whether the failure means the token will never work again.
func fcmIsGone(status int, apiErr fcmErrorResponse) bool {
	if status == http.StatusNotFound || status == http.StatusGone {
		return true
	}
	if apiErr.Error.Status == "NOT_FOUND" || apiErr.Error.Status == "UNREGISTERED" {
		return true
	}
	for _, d := range apiErr.Error.Details {
		if d.ErrorCode == "UNREGISTERED" || d.ErrorCode == "NOT_FOUND" {
			return true
		}
	}
	return false
}

// sendFCMToUser sends ev to every device token of userID and prunes tokens
// FCM reports as unregistered.
func sendFCMToUser(s *db.NotificationSettings, userID int64, ev NotificationEvent) int {
	tokens, err := db.ListDeviceTokens(userID)
	if err != nil {
		log.Printf("[fcm] list tokens for user %d: %v", userID, err)
		return 0
	}
	if len(tokens) == 0 {
		return 0
	}

	accessToken, err := fcmAccessToken(s, time.Now())
	if err != nil {
		log.Printf("[fcm] access token: %v", err)
		return 0
	}

	sent := 0
	for _, token := range tokens {
		res := sendFCMToToken(s, accessToken, token, ev)
		if res.OK {
			sent++
			continue
		}
		if res.Gone {
			if err := db.DeleteDeviceToken(userID, token); err != nil {
				log.Printf("[fcm] prune token for user %d: %v", userID, err)
			}
		}
		log.Printf("[fcm] user %d: %v", userID, res.Err)
	}
	return sent
}
