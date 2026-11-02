package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"redchef/db"
)

// DeleteMe closes the signed-in user's account after a typed confirmation.
// The confirmation must match the email address on the account (the same value
// used as the username), so a stray request cannot delete an account silently.
func DeleteMe(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == 0 {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := db.GetUserByID(userID)
	if err != nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}

	var req struct {
		Confirm string `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	if !strings.EqualFold(strings.TrimSpace(req.Confirm), strings.TrimSpace(user.Email)) {
		jsonError(w, "type your email address to confirm deletion", http.StatusBadRequest)
		return
	}

	if err := db.DeleteUser(userID); err != nil {
		writeDeleteError(w, err)
		return
	}
	clearSessionCookie(w, r)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// SettingsDeleteAccount is the browser form variant. The SPA sends it as a
// normal form POST with the typed email; on success it redirects back to the
// feed with a flag the client can show as a toast.
func SettingsDeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	user, err := db.GetUserByID(userID)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	confirm := r.FormValue("confirm")
	if !strings.EqualFold(strings.TrimSpace(confirm), strings.TrimSpace(user.Email)) {
		http.Error(w, "type your email address to confirm deletion", http.StatusBadRequest)
		return
	}
	if err := db.DeleteUser(userID); err != nil {
		switch {
		case errors.Is(err, db.ErrLastAdmin):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, db.ErrUserNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			http.Error(w, "failed to delete account", http.StatusInternalServerError)
		}
		return
	}
	clearSessionCookie(w, r)
	http.Redirect(w, r, "/?deleted=1", http.StatusSeeOther)
}

func writeDeleteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, db.ErrLastAdmin):
		jsonError(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, db.ErrUserNotFound):
		jsonError(w, err.Error(), http.StatusNotFound)
	default:
		jsonError(w, "failed to delete account", http.StatusInternalServerError)
	}
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// ── Account export ──

type exportProfile struct {
	ID         int64     `json:"id"`
	Email      string    `json:"email"`
	Role       string    `json:"role"`
	Paid       bool      `json:"paid"`
	CreatedAt  time.Time `json:"created_at"`
	OIDCLinked bool      `json:"oidc_linked"`
}

type exportComment struct {
	ID        int64  `json:"id"`
	PostID    int64  `json:"post_id"`
	PostTitle string `json:"post_title"`
	ParentID  *int64 `json:"parent_id,omitempty"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

type exportFavourite struct {
	PostID    int64  `json:"post_id"`
	PostTitle string `json:"post_title"`
	CreatedAt string `json:"created_at"`
}

type exportTip struct {
	PostID      int64  `json:"post_id"`
	PostTitle   string `json:"post_title"`
	AmountCents int    `json:"amount_cents"`
	CreatedAt   string `json:"created_at"`
}

type exportPurchase struct {
	PostID     int64  `json:"post_id"`
	PostTitle  string `json:"post_title"`
	PriceCents int    `json:"price_cents"`
	CreatedAt  string `json:"created_at"`
}

type accountExport struct {
	ExportedAt time.Time         `json:"exported_at"`
	Profile    exportProfile     `json:"profile"`
	Comments   []exportComment   `json:"comments"`
	Favourites []exportFavourite `json:"favourites"`
	Tips       []exportTip       `json:"tips"`
	Purchases  []exportPurchase  `json:"purchases"`
}

// ExportMe returns the signed-in user's own data as JSON. It deliberately
// omits the password hash, OIDC subject, session tokens and API token hashes.
func ExportMe(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == 0 {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := db.GetUserByID(userID)
	if err != nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}

	out := accountExport{
		ExportedAt: time.Now().UTC(),
		Profile: exportProfile{
			ID:        user.ID,
			Email:     user.Email,
			Role:      user.Role,
			Paid:      user.Paid,
			CreatedAt: user.CreatedAt,
		},
		Comments:   []exportComment{},
		Favourites: []exportFavourite{},
		Tips:       []exportTip{},
		Purchases:  []exportPurchase{},
	}

	var oidcSub sql.NullString
	if err := db.DB.QueryRow("SELECT oidc_sub FROM users WHERE id = ?", userID).Scan(&oidcSub); err == nil {
		out.Profile.OIDCLinked = oidcSub.Valid && oidcSub.String != ""
	}

	if rows, err := db.DB.Query(`
		SELECT c.id, c.post_id, p.title, c.parent_id, c.body, c.created_at
		FROM comments c JOIN posts p ON p.id = c.post_id
		WHERE c.user_id = ? ORDER BY c.created_at ASC, c.id ASC`, userID); err == nil {
		for rows.Next() {
			var c exportComment
			var parent sql.NullInt64
			if err := rows.Scan(&c.ID, &c.PostID, &c.PostTitle, &parent, &c.Body, &c.CreatedAt); err == nil {
				if parent.Valid {
					c.ParentID = &parent.Int64
				}
				out.Comments = append(out.Comments, c)
			}
		}
		rows.Close()
	}

	if rows, err := db.DB.Query(`
		SELECT f.post_id, p.title, f.created_at
		FROM favourites f JOIN posts p ON p.id = f.post_id
		WHERE f.user_id = ? ORDER BY f.created_at ASC`, userID); err == nil {
		for rows.Next() {
			var f exportFavourite
			if err := rows.Scan(&f.PostID, &f.PostTitle, &f.CreatedAt); err == nil {
				out.Favourites = append(out.Favourites, f)
			}
		}
		rows.Close()
	}

	if rows, err := db.DB.Query(`
		SELECT t.post_id, p.title, t.amount_cents, t.created_at
		FROM tips t JOIN posts p ON p.id = t.post_id
		WHERE t.user_id = ? ORDER BY t.created_at ASC`, userID); err == nil {
		for rows.Next() {
			var t exportTip
			if err := rows.Scan(&t.PostID, &t.PostTitle, &t.AmountCents, &t.CreatedAt); err == nil {
				out.Tips = append(out.Tips, t)
			}
		}
		rows.Close()
	}

	if rows, err := db.DB.Query(`
		SELECT pu.post_id, p.title, pu.price_cents, pu.created_at
		FROM purchases pu JOIN posts p ON p.id = pu.post_id
		WHERE pu.user_id = ? ORDER BY pu.created_at ASC`, userID); err == nil {
		for rows.Next() {
			var p exportPurchase
			if err := rows.Scan(&p.PostID, &p.PostTitle, &p.PriceCents, &p.CreatedAt); err == nil {
				out.Purchases = append(out.Purchases, p)
			}
		}
		rows.Close()
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="redchef-account-export.json"`)
	w.Header().Set("Cache-Control", "no-store")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(out)
}
