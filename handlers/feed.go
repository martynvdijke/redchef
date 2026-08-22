package handlers

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"

	"redchef/db"
)

// Feed handles both RSS XML and JSON Feed.
// Content negotiation:
//   - ?format=json  -> JSON Feed 1.1
//   - Accept: application/json or application/feed+json -> JSON
//   - otherwise -> RSS 2.0 (application/rss+xml)

func Feed(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	accept := r.Header.Get("Accept")
	wantJSON := format == "json" || strings.Contains(accept, "application/json") || strings.Contains(accept, "application/feed+json")

	posts, err := db.GetPosts(&db.PostFilter{})
	if err != nil {
		if wantJSON {
			jsonError(w, "failed to list posts", http.StatusInternalServerError)
			return
		}
		http.Error(w, "failed to list posts", http.StatusInternalServerError)
		return
	}
	if posts == nil {
		posts = []db.Post{}
	}

	baseURL := getBaseURL(r)

	if wantJSON {
		serveJSONFeed(w, r, posts, baseURL)
		return
	}
	serveRSSFeed(w, posts, baseURL)
}

func getBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	// Respect X-Forwarded-Proto (common behind reverse proxies)
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	host := r.Host
	if host == "" {
		host = "localhost"
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

// ── RSS ──

type rss struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	MediaNS string     `xml:"xmlns:media,attr,omitempty"`
	AtomNS  string     `xml:"xmlns:atom,attr,omitempty"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Language    string    `xml:"language,omitempty"`
	LastBuild   string    `xml:"lastBuildDate,omitempty"`
	AtomLink    *atomLink `xml:"atom:link,omitempty"`
	Items       []rssItem `xml:"item"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type rssItem struct {
	Title        string        `xml:"title"`
	Link         string        `xml:"link"`
	GUID         guid          `xml:"guid"`
	PubDate      string        `xml:"pubDate,omitempty"`
	Description  string        `xml:"description"`
	Enclosure    *enclosure    `xml:"enclosure,omitempty"`
	MediaContent *mediaContent `xml:"media:content,omitempty"`
	MediaThumb   *mediaThumb   `xml:"media:thumbnail,omitempty"`
}

type guid struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

type enclosure struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Length string `xml:"length,attr"`
}

type mediaContent struct {
	URL    string `xml:"url,attr"`
	Medium string `xml:"medium,attr"`
	Type   string `xml:"type,attr"`
}

type mediaThumb struct {
	URL string `xml:"url,attr"`
}

func serveRSSFeed(w http.ResponseWriter, posts []db.Post, baseURL string) {
	channel := rssChannel{
		Title:       "Red Copper Chef — Official Fan Page",
		Link:        baseURL + "/",
		Description: "5-minute gourmet meals. Zero shame. Kitchen royalty. 🔥 — Latest posts from Red Copper Chef",
		Language:    "en",
		AtomLink:    &atomLink{Href: baseURL + "/feed.xml", Rel: "self", Type: "application/rss+xml"},
	}

	if len(posts) > 0 {
		channel.LastBuild = posts[0].CreatedAt.Format("Mon, 02 Jan 2006 15:04:05 -0700")
	}

	for _, p := range posts {
		postURL := fmt.Sprintf("%s/posts/%d", baseURL, p.ID)
		pubDate := p.CreatedAt.Format("Mon, 02 Jan 2006 15:04:05 -0700")

		// Description is the post message; use CDATA-safe plain text.
		// xml.Marshal will escape it automatically.
		desc := p.Description

		item := rssItem{
			Title:       p.Title,
			Link:        postURL,
			GUID:        guid{IsPermaLink: "true", Value: postURL},
			PubDate:     pubDate,
			Description: desc,
		}

		// Image / media — always included (posts are jokes, even locked)
		if p.Filename != "" && !strings.HasPrefix(p.Filename, "_raw_") {
			mediaURL := baseURL + "/uploads/" + p.Filename
			mimeType := mimeForMediaType(p.MediaType, p.Filename)
			item.Enclosure = &enclosure{
				URL:    mediaURL,
				Type:   mimeType,
				Length: "0",
			}
			medium := "image"
			if p.MediaType == "video" {
				medium = "video"
			}
			item.MediaContent = &mediaContent{
				URL:    mediaURL,
				Medium: medium,
				Type:   mimeType,
			}
		}
		if p.Thumbnail != "" && p.Thumbnail != p.Filename && !strings.HasPrefix(p.Thumbnail, "_raw_") {
			thumbURL := baseURL + "/uploads/" + p.Thumbnail
			item.MediaThumb = &mediaThumb{URL: thumbURL}
		} else if p.Filename != "" && p.MediaType == "photo" && !strings.HasPrefix(p.Filename, "_raw_") {
			// For photos without separate thumbnail, use the image itself as thumbnail
			item.MediaThumb = &mediaThumb{URL: baseURL + "/uploads/" + p.Filename}
		}

		channel.Items = append(channel.Items, item)
	}
	if channel.Items == nil {
		channel.Items = []rssItem{}
	}

	feed := rss{
		Version: "2.0",
		MediaNS: "http://search.yahoo.com/mrss/",
		AtomNS:  "http://www.w3.org/2005/Atom",
		Channel: channel,
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(feed); err != nil {
		// fallback
		http.Error(w, "failed to encode feed", http.StatusInternalServerError)
	}
}

func mimeForMediaType(mediaType, filename string) string {
	switch mediaType {
	case "video":
		lower := strings.ToLower(filename)
		if strings.HasSuffix(lower, ".webm") {
			return "video/webm"
		}
		if strings.HasSuffix(lower, ".mov") {
			return "video/quicktime"
		}
		return "video/mp4"
	default:
		lower := strings.ToLower(filename)
		switch {
		case strings.HasSuffix(lower, ".png"):
			return "image/png"
		case strings.HasSuffix(lower, ".gif"):
			return "image/gif"
		case strings.HasSuffix(lower, ".webp"):
			return "image/webp"
		default:
			return "image/jpeg"
		}
	}
}

// ── JSON Feed 1.1 ──

type jsonFeed struct {
	Version     string     `json:"version"`
	Title       string     `json:"title"`
	HomePageURL string     `json:"home_page_url"`
	FeedURL     string     `json:"feed_url,omitempty"`
	Description string     `json:"description,omitempty"`
	Items       []jsonItem `json:"items"`
}

type jsonItem struct {
	ID            string `json:"id"`
	URL           string `json:"url"`
	Title         string `json:"title"`
	ContentText   string `json:"content_text,omitempty"`
	Image         string `json:"image,omitempty"`
	DatePublished string `json:"date_published,omitempty"`
}

func serveJSONFeed(w http.ResponseWriter, r *http.Request, posts []db.Post, baseURL string) {
	feedURL := baseURL + r.URL.Path
	if r.URL.RawQuery != "" {
		feedURL += "?" + r.URL.RawQuery
	}
	jf := jsonFeed{
		Version:     "https://jsonfeed.org/version/1.1",
		Title:       "Red Copper Chef — Official Fan Page",
		HomePageURL: baseURL + "/",
		FeedURL:     feedURL,
		Description: "5-minute gourmet meals. Zero shame. Kitchen royalty. 🔥 — Latest posts from Red Copper Chef",
		Items:       []jsonItem{},
	}
	for _, p := range posts {
		postURL := fmt.Sprintf("%s/posts/%d", baseURL, p.ID)
		item := jsonItem{
			ID:            postURL,
			URL:           postURL,
			Title:         p.Title,
			ContentText:   p.Description,
			DatePublished: p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if p.Filename != "" && !strings.HasPrefix(p.Filename, "_raw_") {
			item.Image = baseURL + "/uploads/" + p.Filename
		} else if p.Thumbnail != "" {
			item.Image = baseURL + "/uploads/" + p.Thumbnail
		}
		jf.Items = append(jf.Items, item)
	}

	w.Header().Set("Content-Type", "application/feed+json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(jf)
}
