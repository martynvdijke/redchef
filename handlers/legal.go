package handlers

import (
	"embed"
	"html/template"
	"net/http"
	"os"
	"strings"
	"time"

	"redchef/db"
)

//go:embed templates/legal/*.html
var legalTemplateFS embed.FS

// legalLayout is parsed once. Each page file defines its own "content", so it
// must be parsed into a clone of the layout rather than into a shared set where
// the last file parsed would win for every page.
var legalLayout = template.Must(template.ParseFS(legalTemplateFS, "templates/legal/layout.html"))

// LegalData is the operator identity published on the public legal pages.
// Every value comes from configuration; nothing here is read from an account.
type LegalData struct {
	Controller           string
	ContactEmail         string
	ContactAddress       string
	EURepresentative     string
	DPOEmail             string
	SupervisoryAuthority string
	HostingRegion        string
	BaseURL              string
	Year                 int
}

// legalData reads the LEGAL_* environment once per request. Missing values
// fall back to wording that is still true: an unnamed operator, EU hosting as
// the default deployment, and no invented contact address.
func legalData() LegalData {
	d := LegalData{
		Controller:           "The operator of this RedChef instance",
		SupervisoryAuthority: "the data protection authority in your EU/EEA country of residence",
		HostingRegion:        "the European Union",
		BaseURL:              "https://redchef.vandijke.xyz",
		Year:                 time.Now().UTC().Year(),
	}
	if v := strings.TrimSpace(os.Getenv("LEGAL_CONTROLLER_NAME")); v != "" {
		d.Controller = v
	}
	if v := strings.TrimSpace(os.Getenv("LEGAL_CONTACT_EMAIL")); v != "" {
		d.ContactEmail = v
	}
	if v := strings.TrimSpace(os.Getenv("LEGAL_CONTACT_ADDRESS")); v != "" {
		d.ContactAddress = v
	}
	if v := strings.TrimSpace(os.Getenv("LEGAL_EU_REPRESENTATIVE")); v != "" {
		d.EURepresentative = v
	}
	if v := strings.TrimSpace(os.Getenv("LEGAL_DPO_EMAIL")); v != "" {
		d.DPOEmail = v
	}
	if v := strings.TrimSpace(os.Getenv("LEGAL_SUPERVISORY_AUTHORITY")); v != "" {
		d.SupervisoryAuthority = v
	}
	if v := strings.TrimSpace(os.Getenv("LEGAL_HOSTING_REGION")); v != "" {
		d.HostingRegion = v
	}
	if v := strings.TrimSpace(os.Getenv("LEGAL_BASE_URL")); v != "" {
		d.BaseURL = strings.TrimRight(v, "/")
	}
	return d
}

type legalPageData struct {
	Legal       LegalData
	Title       string
	Description string
	SignedIn    bool
	Email       string
	Deleted     bool
}

func renderLegal(w http.ResponseWriter, status int, page string, data legalPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	tmpl, err := legalLayout.Clone()
	if err != nil {
		http.Error(w, "could not render legal page", http.StatusInternalServerError)
		return
	}
	raw, err := legalTemplateFS.ReadFile("templates/legal/" + page + ".html")
	if err == nil {
		_, err = tmpl.Parse(string(raw))
	}
	if err == nil {
		err = tmpl.ExecuteTemplate(w, "layout", data)
	}
	if err != nil {
		// The template is compiled at startup; an execution error means the
		// response is already partially written, so only log-worthy failure.
		http.Error(w, "could not render legal page", http.StatusInternalServerError)
	}
}

// PrivacyPage serves the public privacy policy. It must stay reachable
// without a session: app stores and the GDPR require the policy to be
// readable before anyone signs up.
func PrivacyPage(w http.ResponseWriter, r *http.Request) {
	renderLegal(w, http.StatusOK, "privacy", legalPageData{
		Legal:       legalData(),
		Title:       "Privacy policy",
		Description: "What this RedChef instance stores, why, who processes it, and how to export or delete it.",
	})
}

// TermsPage serves the acceptable-use terms without authentication.
func TermsPage(w http.ResponseWriter, r *http.Request) {
	renderLegal(w, http.StatusOK, "terms", legalPageData{
		Legal:       legalData(),
		Title:       "Terms of use",
		Description: "The rules for using this RedChef instance and what you keep ownership of.",
	})
}

// AccountDeletePage is the public deletion URL Google Play expects for apps
// with accounts. It explains the signed-in path, the email fallback, what is
// removed, and what may remain in backups.
func AccountDeletePage(w http.ResponseWriter, r *http.Request) {
	data := legalPageData{
		Legal:    legalData(),
		Title:    "Delete your account",
		SignedIn: getUserID(r) > 0,
		Deleted:  r.URL.Query().Get("deleted") == "1",
	}
	if data.SignedIn {
		if user, err := db.GetUserByID(getUserID(r)); err == nil {
			data.Email = user.Email
		}
	}
	data.Description = "How to delete a RedChef account, what is removed immediately, and what may remain in backups."
	renderLegal(w, http.StatusOK, "account_delete", data)
}
