package handlers

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"redchef/db"
)

// OIDCConfig is the OIDC relying-party config, read from env.
type OIDCConfig struct {
	Enabled     bool
	Issuer      string
	ClientID    string
	Secret      string
	RedirectURL string
	Scopes      []string
	LogoutURL   string
}

func loadOIDCConfig() OIDCConfig {
	enabled := false // OIDC is opt-in; set OIDC_ENABLED=true
	if v, ok := os.LookupEnv("OIDC_ENABLED"); ok {
		enabled = v == "true" || v == "1"
	}
	cfg := OIDCConfig{Enabled: enabled}
	if f := os.Getenv("OIDC_CLIENT_SECRET_FILE"); f != "" {
		if b, err := os.ReadFile(f); err == nil {
			cfg.Secret = strings.TrimSpace(string(b))
		}
	}
	if cfg.Secret == "" {
		cfg.Secret = os.Getenv("OIDC_CLIENT_SECRET")
	}
	cfg.Issuer = strings.TrimSuffix(os.Getenv("OIDC_ISSUER_URL"), "/")
	cfg.ClientID = os.Getenv("OIDC_CLIENT_ID")
	cfg.RedirectURL = os.Getenv("OIDC_REDIRECT_URL")
	if s := os.Getenv("OIDC_SCOPES"); s != "" {
		cfg.Scopes = strings.Fields(s)
	} else {
		cfg.Scopes = []string{"openid", "email", "profile", "groups"}
	}
	if u := os.Getenv("OIDC_LOGOUT_URL"); u != "" {
		cfg.LogoutURL = u
	} else {
		cfg.LogoutURL = cfg.Issuer + "/logout"
	}
	return cfg
}

func (c OIDCConfig) valid() bool {
	return c.Enabled && c.Issuer != "" && c.ClientID != "" && c.Secret != "" && c.RedirectURL != ""
}

type oidcProvider struct {
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth    *oauth2.Config
	issuer   string
	clientID string
	secret   string
	scopes   string
}

var (
	oidcMu    sync.Mutex
	oidcCache *oidcProvider
)

func getOIDCProvider(ctx context.Context, cfg OIDCConfig) (*oidcProvider, error) {
	oidcMu.Lock()
	defer oidcMu.Unlock()
	scopesKey := strings.Join(cfg.Scopes, " ")
	if oidcCache != nil && oidcCache.issuer == cfg.Issuer && oidcCache.clientID == cfg.ClientID && oidcCache.secret == cfg.Secret && oidcCache.scopes == scopesKey {
		return oidcCache, nil
	}
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, err
	}
	oidcCache = &oidcProvider{
		provider: provider,
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		oauth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.Secret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  cfg.RedirectURL,
			Scopes:       cfg.Scopes,
		},
		issuer:   cfg.Issuer,
		clientID: cfg.ClientID,
		secret:   cfg.Secret,
		scopes:   strings.Join(cfg.Scopes, " "),
	}
	return oidcCache, nil
}

func oidcRandHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func setOIDCCookie(w http.ResponseWriter, r *http.Request, name, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

func clearOIDCCookies(w http.ResponseWriter, r *http.Request) {
	secure := false
	if r != nil {
		secure = isSecureRequest(r)
	}
	for _, n := range []string{"oidc_state", "oidc_nonce", "oidc_verifier"} {
		http.SetCookie(w, &http.Cookie{Name: n, Value: "", Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	}
}

func oidcFail(w http.ResponseWriter, r *http.Request, msg string) {
	clearOIDCCookies(w, r)
	// Keep "/?error=" — RedChef is an SPA with login modal at "/" (see static/index.html#login-modal), not a dedicated /login page.
	http.Redirect(w, r, "/?error="+msg, http.StatusFound)
}

// OIDCStatus reports whether OIDC login is usable.
func OIDCStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"enabled": loadOIDCConfig().valid()})
}

// OIDCLogin starts the Authorization Code + PKCE flow.
func OIDCLogin(w http.ResponseWriter, r *http.Request) {
	cfg := loadOIDCConfig()
	if !cfg.valid() {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	p, err := getOIDCProvider(ctx, cfg)
	if err != nil {
		http.Error(w, `{"error":"oidc discovery failed"}`, http.StatusBadGateway)
		return
	}
	state, err := oidcRandHex(16)
	if err != nil {
		http.Error(w, `{"error":"failed to generate state"}`, http.StatusInternalServerError)
		return
	}
	nonce, err := oidcRandHex(16)
	if err != nil {
		http.Error(w, `{"error":"failed to generate nonce"}`, http.StatusInternalServerError)
		return
	}
	verifier := oauth2.GenerateVerifier()
	setOIDCCookie(w, r, "oidc_state", state, 300)
	setOIDCCookie(w, r, "oidc_nonce", nonce, 300)
	setOIDCCookie(w, r, "oidc_verifier", verifier, 300)
	url := p.oauth.AuthCodeURL(state,
		oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("nonce", nonce),
	)
	http.Redirect(w, r, url, http.StatusFound)
}

type oidcClaims struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	PreferredName string `json:"preferred_username"`
	Nonce         string `json:"nonce"`
}

// OIDCCallback verifies state/nonce/PKCE + ID token, links/provisions user.
func OIDCCallback(w http.ResponseWriter, r *http.Request) {
	cfg := loadOIDCConfig()
	if !cfg.valid() {
		http.NotFound(w, r)
		return
	}
	stateCookie, err1 := r.Cookie("oidc_state")
	nonceCookie, err2 := r.Cookie("oidc_nonce")
	verifierCookie, err3 := r.Cookie("oidc_verifier")
	if err1 != nil || err2 != nil || err3 != nil || stateCookie.Value == "" || nonceCookie.Value == "" || verifierCookie.Value == "" {
		oidcFail(w, r, "oidc_expired")
		return
	}
	qState := r.URL.Query().Get("state")
	if qState == "" || subtle.ConstantTimeCompare([]byte(qState), []byte(stateCookie.Value)) != 1 {
		oidcFail(w, r, "oidc_state")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	p, err := getOIDCProvider(ctx, cfg)
	if err != nil {
		oidcFail(w, r, "oidc_provider")
		return
	}
	token, err := p.oauth.Exchange(ctx, r.URL.Query().Get("code"), oauth2.VerifierOption(verifierCookie.Value))
	if err != nil {
		oidcFail(w, r, "oidc_exchange")
		return
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		oidcFail(w, r, "oidc_token")
		return
	}
	idToken, err := p.verifier.Verify(ctx, raw)
	if err != nil {
		oidcFail(w, r, "oidc_verify")
		return
	}
	var claims oidcClaims
	if err := idToken.Claims(&claims); err != nil {
		oidcFail(w, r, "oidc_claims")
		return
	}
	if subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(nonceCookie.Value)) != 1 {
		oidcFail(w, r, "oidc_nonce")
		return
	}
	if claims.Email == "" || !claims.EmailVerified {
		oidcFail(w, r, "oidc_email")
		return
	}
	var rawClaims map[string]any
	groups := []string{}
	groupsPresent := false
	if err := idToken.Claims(&rawClaims); err == nil {
		if g, ok := rawClaims["groups"]; ok && g != nil {
			groupsPresent = true
			if arr, ok := g.([]any); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok {
						groups = append(groups, s)
					}
				}
			}
		}
	}
	sub := cfg.Issuer + "|" + claims.Sub
	u, err := linkOrProvisionOIDCUser(r.Context(), sub, claims, groups, groupsPresent)
	if err != nil {
		oidcFail(w, r, "oidc_user")
		return
	}
	clearOIDCCookies(w, r)
	session, err := db.CreateUserSession(u.id)
	if err != nil {
		oidcFail(w, r, "oidc_session")
		return
	}
	if _, err := db.DB.Exec("UPDATE user_sessions SET auth_method = 'oidc', oidc_sub = ? WHERE token = ?", sub, session.Token); err != nil {
		db.DeleteUserSession(session.Token)
		oidcFail(w, r, "oidc_session")
		return
	}
	setSessionCookie(w, r, session.Token)
	http.Redirect(w, r, "/", http.StatusFound)
}

// OIDCLogout clears local session and redirects to IdP logout.
func OIDCLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("session_token"); err == nil && c.Value != "" {
		db.DeleteUserSession(c.Value)
	}
	if c, err := r.Cookie("admin_token"); err == nil && c.Value != "" {
		db.DeleteSession(c.Value)
	}
	secure := isSecureRequest(r)
	http.SetCookie(w, &http.Cookie{Name: "session_token", Value: "", Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: "admin_token", Value: "", Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	// also clear oidc cookies
	clearOIDCCookies(w, r)
	cfg := loadOIDCConfig()
	if cfg.valid() {
		http.Redirect(w, r, cfg.LogoutURL, http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

type oidcUser struct {
	id   int64
	role string
}

func oidcRoleForGroups(groups []string) string {
	for _, g := range groups {
		if g == "admins" {
			return "admin"
		}
	}
	return "normal"
}

func linkOrProvisionOIDCUser(ctx context.Context, sub string, claims oidcClaims, groups []string, groupsPresent bool) (*oidcUser, error) {
	// try oidc_sub
	var id int64
	var role string
	err := db.DB.QueryRowContext(ctx, `SELECT id, role FROM users WHERE oidc_sub = ?`, sub).Scan(&id, &role)
	if err == nil {
		u := &oidcUser{id: id, role: role}
		return maybeSyncOIDCRole(ctx, u, groups, groupsPresent)
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	// try email
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	err = db.DB.QueryRowContext(ctx, `SELECT id, role FROM users WHERE email = ?`, email).Scan(&id, &role)
	if err == nil {
		if _, err := db.DB.ExecContext(ctx, `UPDATE users SET oidc_sub = ? WHERE id = ?`, sub, id); err != nil {
			return nil, err
		}
		u := &oidcUser{id: id, role: role}
		return maybeSyncOIDCRole(ctx, u, groups, groupsPresent)
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	// provision
	role = "normal"
	if groupsPresent {
		role = oidcRoleForGroups(groups)
	}
	pw, err := oidcRandHex(16)
	if err != nil {
		return nil, err
	}
	base := claims.PreferredName
	if base == "" {
		base = claims.Name
	}
	if base == "" {
		if i := strings.Index(email, "@"); i > 0 {
			base = email[:i]
		} else {
			base = email
		}
	}
	username := uniquifyUsername(ctx, base)
	// first user bootstrap is already admin, so provision follows groups mapping
	res, err := db.DB.ExecContext(ctx,
		`INSERT INTO users(username, password_hash, email, role, paid, oidc_sub) VALUES(?, ?, ?, ?, 0, ?)`,
		username, "oidc$"+pw, email, role, sub)
	if err != nil {
		return nil, err
	}
	newID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &oidcUser{id: newID, role: role}, nil
}

func maybeSyncOIDCRole(ctx context.Context, u *oidcUser, groups []string, groupsPresent bool) (*oidcUser, error) {
	if !groupsPresent {
		return u, nil
	}
	role := oidcRoleForGroups(groups)
	if role == u.role {
		return u, nil
	}
	if _, err := db.DB.ExecContext(ctx, `UPDATE users SET role = ? WHERE id = ?`, role, u.id); err != nil {
		return nil, err
	}
	u.role = role
	return u, nil
}

func uniquifyUsername(ctx context.Context, base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "user"
	}
	candidate := base
	for i := 2; ; i++ {
		var one int
		if err := db.DB.QueryRowContext(ctx, `SELECT 1 FROM users WHERE username = ?`, candidate).Scan(&one); err == sql.ErrNoRows {
			return candidate
		} else if err != nil {
			return candidate
		}
		candidate = base + "-" + strconv.Itoa(i)
	}
}
