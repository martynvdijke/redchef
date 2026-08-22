package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"redchef/db"
)

// metaPlaceholder marks where post metadata (JSON-LD + Open Graph tags) is
// injected into the SPA shell served on /posts/{id}.
const metaPlaceholder = "<!--#meta-->"

// recipeLD is the schema.org Recipe JSON-LD payload injected into shareable
// pages. Locked posts serialize only name/description/image.
type recipeLD struct {
	Context      string          `json:"@context"`
	Type         string          `json:"@type"`
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	Image        string          `json:"image,omitempty"`
	Ingredients  []string        `json:"recipeIngredient,omitempty"`
	Instructions []ldInstruction `json:"recipeInstructions,omitempty"`
	Yield        string          `json:"recipeYield,omitempty"`
	PrepTime     string          `json:"prepTime,omitempty"`
	CookTime     string          `json:"cookTime,omitempty"`
}

type ldInstruction struct {
	Type string `json:"@type"`
	Text string `json:"text"`
}

// RenderPostPage serves the SPA shell for /posts/{id} with machine-readable
// post metadata injected at the placeholder. Crawlers and link-preview bots
// don't execute JS, so this must happen server-side.
func RenderPostPage(w http.ResponseWriter, r *http.Request, indexHTML []byte) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	post, err := db.GetPost(id)
	if err != nil {
		// Unknown post: serve the plain SPA shell (client shows its own 404).
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
		return
	}

	unlocked := !post.Locked
	if !unlocked {
		userID := getUserID(r)
		if userID > 0 {
			if user, err := db.GetUserByID(userID); err == nil && (user.Paid || user.Role == "admin") {
				unlocked = true
			}
		}
	}
	if !unlocked {
		if purchased, _ := db.HasUserPurchased(getUserID(r), post.ID); purchased {
			unlocked = true
		}
	}

	baseURL := getBaseURL(r)
	meta := buildPostMeta(post, unlocked, baseURL)

	html := injectMeta(indexHTML, meta)
	if html == nil {
		// Placeholder missing and no <head> found — serve unmodified.
		log.Printf("[share] could not find injection point in index.html")
		html = indexHTML
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(html)
}

// buildPostMeta renders the JSON-LD script and Open Graph tags for a post.
func buildPostMeta(post *db.Post, unlocked bool, baseURL string) string {
	pageURL := fmt.Sprintf("%s/posts/%d", baseURL, post.ID)
	imageURL := ""
	if thumb := mediaFileForPost(post); thumb != "" {
		imageURL = baseURL + "/uploads/" + thumb
	}

	ld := recipeLD{
		Context:     "https://schema.org",
		Type:        "Recipe",
		Name:        post.Title,
		Description: post.Description,
		Image:       imageURL,
	}
	if unlocked && !post.Recipe.IsZero() {
		ld.Ingredients = nonEmpty(post.Recipe.Ingredients)
		steps := nonEmpty(post.Recipe.Steps)
		for _, s := range steps {
			ld.Instructions = append(ld.Instructions, ldInstruction{Type: "HowToStep", Text: s})
		}
		if post.Recipe.Servings > 0 {
			ld.Yield = fmt.Sprintf("%d servings", post.Recipe.Servings)
		}
		ld.PrepTime = isoDuration(post.Recipe.PrepMinutes)
		ld.CookTime = isoDuration(post.Recipe.CookMinutes)
	}

	var b strings.Builder
	b.WriteString(`<script type="application/ld+json">`)
	if data, err := json.Marshal(ld); err == nil {
		b.Write(data)
	}
	b.WriteString("</script>\n")
	b.WriteString(fmt.Sprintf(`<meta property="og:title" content="%s">`+"\n", htmlAttr(post.Title)))
	b.WriteString(fmt.Sprintf(`<meta property="og:url" content="%s">`+"\n", htmlAttr(pageURL)))
	b.WriteString(fmt.Sprintf(`<meta property="og:type" content="article">` + "\n"))
	if post.Description != "" {
		b.WriteString(fmt.Sprintf(`<meta property="og:description" content="%s">`+"\n", htmlAttr(post.Description)))
	}
	if imageURL != "" {
		b.WriteString(fmt.Sprintf(`<meta property="og:image" content="%s">`+"\n", htmlAttr(imageURL)))
	}
	return strings.TrimRight(b.String(), "\n")
}

func mediaFileForPost(post *db.Post) string {
	if !strings.HasPrefix(post.Thumbnail, "_raw_") && post.Thumbnail != "" {
		return post.Thumbnail
	}
	if !strings.HasPrefix(post.Filename, "_raw_") && post.Filename != "" {
		return post.Filename
	}
	return ""
}

func nonEmpty(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			out = append(out, item)
		}
	}
	return out
}

// isoDuration formats minutes as an ISO 8601 duration ("PT90M").
func isoDuration(minutes int) string {
	if minutes <= 0 {
		return ""
	}
	return fmt.Sprintf("PT%dM", minutes)
}

func htmlAttr(s string) string {
	r := strings.NewReplacer("&", "&amp;", `"`, "&quot;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// injectMeta replaces the placeholder with meta content; if the placeholder
// is missing it falls back to inserting right after <head>. Returns nil when
// neither anchor exists.
func injectMeta(indexHTML []byte, meta string) []byte {
	if bytes.Contains(indexHTML, []byte(metaPlaceholder)) {
		return bytes.ReplaceAll(indexHTML, []byte(metaPlaceholder), []byte(meta))
	}
	if idx := bytes.Index(bytes.ToLower(indexHTML), []byte("<head>")); idx >= 0 {
		insertAt := idx + len("<head>")
		out := make([]byte, 0, len(indexHTML)+len(meta)+1)
		out = append(out, indexHTML[:insertAt]...)
		out = append(out, '\n')
		out = append(out, []byte(meta)...)
		out = append(out, indexHTML[insertAt:]...)
		return out
	}
	return nil
}
