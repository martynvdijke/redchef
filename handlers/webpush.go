package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	webpush "github.com/SherClockHolmes/webpush-go"

	"redchef/db"
)

// webPushSender sends one encrypted payload to one subscription and returns the
// HTTP status code of the push service. Declared as a variable so tests can
// substitute a fake service.
type webPushSender func(sub db.WebPushSubscription, payload []byte, settings *db.NotificationSettings) (int, error)

var sendWebPush webPushSender = sendWebPushNotification

// GenerateVAPIDKeys creates a fresh P-256 VAPID keypair. The private key is
// returned first, matching the library.
func GenerateVAPIDKeys() (privateKey, publicKey string, err error) {
	return webpush.GenerateVAPIDKeys()
}

// vapidSubscriber resolves the "sub" claim for the VAPID JWT. The webpush
// library prefixes plain addresses with "mailto:", so the stored value must be
// bare (or an https URL).
func vapidSubscriber(s *db.NotificationSettings) string {
	subject := ""
	if s != nil {
		subject = strings.TrimSpace(s.VAPIDSubject)
	}
	if subject == "" {
		if email, err := db.GetEmailSettings(); err == nil {
			subject = strings.TrimSpace(email.FromAddr)
		}
	}
	if subject == "" {
		subject = "admin@redchef.local"
	}
	// Only an e-mail address gets the mailto: prefix; an https URL is a valid
	// VAPID subject on its own.
	if !strings.HasPrefix(subject, "https://") && !strings.HasPrefix(subject, "http://") {
		subject = strings.TrimPrefix(subject, "mailto:")
	}
	return subject
}

// sendWebPushNotification performs the actual encryption and POST.
func sendWebPushNotification(sub db.WebPushSubscription, payload []byte, s *db.NotificationSettings) (int, error) {
	if s == nil || strings.TrimSpace(s.VAPIDPublicKey) == "" || strings.TrimSpace(s.VAPIDPrivateKey) == "" {
		return 0, errors.New("web push: VAPID keys are not configured")
	}

	resp, err := webpush.SendNotification(payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.P256dh,
			Auth:   sub.Auth,
		},
	}, &webpush.Options{
		Subscriber:      vapidSubscriber(s),
		VAPIDPublicKey:  s.VAPIDPublicKey,
		VAPIDPrivateKey: s.VAPIDPrivateKey,
		TTL:             86400,
	})
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	// Drain so the connection can be reused; the body carries no useful data.
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

// webPushSubscriptionGone reports the statuses a push service uses to say the
// endpoint will never work again.
func webPushSubscriptionGone(status int) bool {
	return status == http.StatusNotFound || status == http.StatusGone
}

// sendWebPushToUser pushes ev to every subscription of userID and deletes the
// endpoints the push service reports as gone.
func sendWebPushToUser(s *db.NotificationSettings, userID int64, ev NotificationEvent) int {
	subs, err := db.ListWebPushSubscriptions(userID)
	if err != nil {
		log.Printf("[webpush] list subscriptions for user %d: %v", userID, err)
		return 0
	}
	if len(subs) == 0 {
		return 0
	}

	payload, err := json.Marshal(ev)
	if err != nil {
		log.Printf("[webpush] marshal event: %v", err)
		return 0
	}

	sent := 0
	for _, sub := range subs {
		status, err := sendWebPush(sub, payload, s)
		if err != nil {
			log.Printf("[webpush] user %d: %v", userID, err)
			continue
		}
		if status >= 200 && status < 300 {
			sent++
			continue
		}
		if webPushSubscriptionGone(status) {
			if err := db.DeleteWebPushSubscription(userID, sub.Endpoint); err != nil {
				log.Printf("[webpush] prune subscription for user %d: %v", userID, err)
			}
			continue
		}
		log.Printf("[webpush] user %d: push service returned %d", userID, status)
	}
	return sent
}
