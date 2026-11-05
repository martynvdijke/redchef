package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"redchef/db"
)

const (
	defaultDemoEmail    = "demo@redchef.local"
	defaultDemoPassword = "redchef-demo"
	defaultLegalBaseURL = "https://redchef.vandijke.xyz"
	demoHost            = "redchef.vandijke.xyz"
)

// DemoConfig describes the shared demo account. Credentials are only returned
// when the account is enabled, so a disabled instance never leaks them.
type DemoConfig struct {
	Enabled  bool   `json:"enabled"`
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
}

func demoBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("LEGAL_BASE_URL")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("BASE_URL")); v != "" {
		return v
	}
	return defaultLegalBaseURL
}

// demoConfig resolves the demo account settings from the environment. When
// DEMO_USER_ENABLED is unset it auto-enables only on the official public host.
func demoConfig() DemoConfig {
	cfg := DemoConfig{
		Email:    defaultDemoEmail,
		Password: defaultDemoPassword,
	}
	if v := strings.TrimSpace(os.Getenv("DEMO_USER_EMAIL")); v != "" {
		cfg.Email = strings.ToLower(v)
	}
	if v := os.Getenv("DEMO_USER_PASSWORD"); v != "" {
		cfg.Password = v
	}

	switch strings.ToLower(strings.TrimSpace(os.Getenv("DEMO_USER_ENABLED"))) {
	case "true", "1", "yes", "on":
		cfg.Enabled = true
	case "false", "0", "no", "off":
		cfg.Enabled = false
	default:
		if u, err := url.Parse(demoBaseURL()); err == nil {
			cfg.Enabled = strings.EqualFold(u.Hostname(), demoHost)
		}
	}
	return cfg
}

// EnsureDemoUser seeds or repairs the shared demo account on boot. It never
// aborts startup: a failure is logged and the rest of the app keeps working.
func EnsureDemoUser() {
	cfg := demoConfig()
	if !cfg.Enabled {
		log.Printf("demo account disabled")
		return
	}
	created, err := db.EnsureDemoUser(cfg.Email, cfg.Password)
	if err != nil {
		log.Printf("demo account error: %v", err)
		return
	}
	if created {
		log.Printf("demo account created: %s", cfg.Email)
		return
	}
	log.Printf("demo account ready: %s", cfg.Email)
}

// DemoConfigHandler advertises the demo credentials to reviewers and the app.
func DemoConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	cfg := demoConfig()
	if !cfg.Enabled {
		json.NewEncoder(w).Encode(map[string]DemoConfig{
			"demo": {Enabled: false},
		})
		return
	}
	json.NewEncoder(w).Encode(map[string]DemoConfig{"demo": cfg})
}

// isDemoAccount reports whether u is the shared demo account. It is only true
// when the feature is enabled, so a disabled deployment can delete the row.
func isDemoAccount(u *db.User) bool {
	if u == nil {
		return false
	}
	cfg := demoConfig()
	return cfg.Enabled && u.Role == "demo" && strings.EqualFold(strings.TrimSpace(u.Email), cfg.Email)
}

// demoDeletionBlocked writes the appropriate error and returns true when the
// requested account is the protected demo account.
func demoDeletionBlocked(w http.ResponseWriter, u *db.User, html bool) bool {
	if !isDemoAccount(u) {
		return false
	}
	const msg = "the shared demo account cannot be deleted"
	if html {
		http.Error(w, msg, http.StatusBadRequest)
	} else {
		jsonError(w, msg, http.StatusBadRequest)
	}
	return true
}
