package handlers

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"strings"

	"redchef/db"
)

// ── Gotify ──

// SendGotifyNotification sends a push notification via Gotify.
// Silently skips when Gotify is not configured.
func SendGotifyNotification(title, message string) {
	s, err := db.GetEmailSettings()
	if err != nil {
		log.Printf("[gotify] settings error: %v", err)
		return
	}
	if strings.TrimSpace(s.GotifyURL) == "" || strings.TrimSpace(s.GotifyToken) == "" {
		return
	}

	url := strings.TrimRight(s.GotifyURL, "/") + "/message?token=" + s.GotifyToken
	payload, _ := json.Marshal(map[string]interface{}{
		"title":    title,
		"message":  message,
		"priority": 5,
	})

	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Printf("[gotify] send failed: %v", err)
		return
	}
	resp.Body.Close()
	log.Printf("[gotify] Sent %q", title)
}

// SendEmail delivers an email using the admin-configured SMTP settings.
// Returns nil without sending when SMTP is not configured (empty host).
func SendEmail(to, subject, body string) error {
	s, err := db.GetEmailSettings()
	if err != nil {
		return fmt.Errorf("email settings: %w", err)
	}
	if strings.TrimSpace(s.SMTPHost) == "" {
		log.Printf("[email] SMTP not configured, skipping email to %s", to)
		return nil
	}

	from := s.FromAddr
	if from == "" {
		from = s.Username
	}
	if from == "" {
		return fmt.Errorf("no from address configured")
	}

	msg := strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	addr := fmt.Sprintf("%s:%d", s.SMTPHost, s.SMTPPort)

	var client *smtp.Client
	switch s.Encryption {
	case "ssl":
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: s.SMTPHost})
		if err != nil {
			return fmt.Errorf("ssl dial: %w", err)
		}
		client, err = smtp.NewClient(conn, s.SMTPHost)
		if err != nil {
			return fmt.Errorf("smtp client: %w", err)
		}
	default: // "tls" (STARTTLS) or "none"
		client, err = smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("dial: %w", err)
		}
		if s.Encryption == "tls" {
			if err := client.StartTLS(&tls.Config{ServerName: s.SMTPHost}); err != nil {
				client.Close()
				return fmt.Errorf("starttls: %w", err)
			}
		}
	}
	defer client.Close()

	// Authenticate when credentials are provided (not for plain "none" encryption)
	if s.Username != "" && s.Encryption != "none" {
		auth := smtp.PlainAuth("", s.Username, s.Password, s.SMTPHost)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("rcpt to: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close data: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("quit: %w", err)
	}

	log.Printf("[email] Sent %q to %s", subject, to)
	return nil
}

// SendWelcomeEmail notifies a new member that their account was created.
// Failures are logged but never block registration.
func SendWelcomeEmail(email string) {
	subject := "Welkom bij Red Copper Chef — je account is aangemaakt! 🍳"
	body := fmt.Sprintf(`Hoi!

Je account bij Red Copper Chef is zojuist aangemaakt. Welkom in de keuken!

Je kunt nu inloggen met dit e-mailadres (%s), reageren op posts, favorieten opslaan en met een lidmaatschap alle content ontgrendelen.

Tot in de keuken!
— Red Copper Chef 🍳

(Dit is een automatisch bericht, antwoorden heeft geen zin — de Chef leest alleen rooksignalen.)`, email)

	if err := SendEmail(email, subject, body); err != nil {
		log.Printf("[email] Failed to send welcome email to %s: %v", email, err)
	}
	SendGotifyNotification("Nieuwe gebruiker", fmt.Sprintf("%s heeft een account aangemaakt", email))
}

// SendSubscriptionInvoice emails a mock invoice for the €4,99/maand subscription.
func SendSubscriptionInvoice(email string) {
	subject := "Factuur: Royal Member abonnement — €4,99/maand 🧾"
	body := `Hoi!

Bedankt voor je aankoop bij Red Copper Chef! Hier is je factuur.

──────────────────────────────
FACTUUR (parodie)
──────────────────────────────
Item:      Royal Member abonnement
Prijs:     €4,99 / maand
Betaald:   via iDEAL (nep — er is niks afgeschreven)
──────────────────────────────

Je hebt nu onbeperkt toegang tot alle content. Smakelijk!

— Red Copper Chef 🍳`

	if err := SendEmail(email, subject, body); err != nil {
		log.Printf("[email] Failed to send subscription invoice to %s: %v", email, err)
	}
	SendGotifyNotification("Nieuw abonnement", fmt.Sprintf("%s heeft een Royal Member abonnement genomen (€4,99/maand)", email))
}

// SendItemInvoice emails a mock invoice for a single purchased item.
func SendItemInvoice(email, postTitle, price string) {
	subject := fmt.Sprintf("Factuur: %s — €%s 🧾", postTitle, price)
	body := fmt.Sprintf(`Hoi!

Bedankt voor je aankoop bij Red Copper Chef! Hier is je factuur.

──────────────────────────────
FACTUUR (parodie)
──────────────────────────────
Item:      %s
Prijs:     €%s (eenmalig)
Betaald:   via iDEAL (nep — er is niks afgeschreven)
──────────────────────────────

Deze content is nu voor jou ontgrendeld. Veel kijkplezier!

— Red Copper Chef 🍳`, postTitle, price)

	if err := SendEmail(email, subject, body); err != nil {
		log.Printf("[email] Failed to send item invoice to %s: %v", email, err)
	}
	SendGotifyNotification("Item gekocht", fmt.Sprintf("%s heeft %s gekocht (€%s)", email, postTitle, price))
}

// SendTipNotification emails the tipper a confirmation and notifies admin via Gotify.
func SendTipNotification(email, postTitle string, amountCents int) {
	amount := formatEuros(amountCents)
	subject := fmt.Sprintf("💸 Bedankt voor je tip! — €%s", amount)
	body := fmt.Sprintf(`Hoi!

Bedankt voor je fooi op "%s"!

┌──────────────────────────────
│ Tip: €%s
│ Post: %s
└──────────────────────────────

De Chef waardeert het enorm! 🍳

— Red Copper Chef 🍳`, postTitle, amount, postTitle)

	if err := SendEmail(email, subject, body); err != nil {
		log.Printf("[email] Failed to send tip notification to %s: %v", email, err)
	}
	SendGotifyNotification("Nieuwe fooi", fmt.Sprintf("%s heeft €%s fooi gegeven op \"%s\"", email, amount, postTitle))
}

// SendNewPostNotification emails all registered users about a new post.
// The creator (adminUserID) is skipped so they don't notify themselves.
func SendNewPostNotification(post db.Post, adminUserID int64) {
	users, err := db.ListUsers()
	if err != nil {
		log.Printf("[email] Failed to list users for new-post notification: %v", err)
		return
	}

	postURL := fmt.Sprintf("https://redchef.example.com/posts/%d", post.ID)
	subject := fmt.Sprintf("🍳 Nieuwe post: %s — Red Copper Chef", post.Title)
	description := post.Description
	if description == "" {
		description = "(geen beschrijving)"
	}
	body := fmt.Sprintf(`Hoi!

Er is een nieuwe post verschenen op Red Copper Chef!

──────────────────────────────
%s
──────────────────────────────
%s

Bekijk de post hier:
%s

— Red Copper Chef 🍳`, post.Title, description, postURL)

	for _, u := range users {
		if u.ID == adminUserID {
			continue
		}
		go func(email string) {
			if err := SendEmail(email, subject, body); err != nil {
				log.Printf("[email] Failed to send new-post notification to %s: %v", email, err)
			}
		}(u.Email)
	}
	SendGotifyNotification("Nieuwe post", fmt.Sprintf("\"%s\" is gepubliceerd", post.Title))
}
