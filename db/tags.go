package db

import (
	"strings"
)

// Tag validation bounds.
const (
	MaxTagsPerPost = 20
	MaxTagLength   = 50
)

// TagWithCount pairs a tag name with the number of published-visible posts
// carrying it (used by GET /api/tags for the filter UI).
type TagWithCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// normalizeTagNames trims, drops empties, and dedupes case-insensitively
// while preserving first-seen order.
func normalizeTagNames(names []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
	}
	return out
}

// SetPostTags replaces the tag set of a post atomically-ish: existing links
// are removed, then each normalized name is upserted into tags and linked.
// Case-insensitive uniqueness comes from tags.name UNIQUE COLLATE NOCASE.
func SetPostTags(postID int64, names []string) error {
	names = normalizeTagNames(names)

	if _, err := DB.Exec("DELETE FROM post_tags WHERE post_id = ?", postID); err != nil {
		return err
	}

	for _, name := range names {
		if _, err := DB.Exec(
			"INSERT INTO tags (name) VALUES (?) ON CONFLICT(name) DO NOTHING",
			name,
		); err != nil {
			return err
		}
		var tagID int64
		if err := DB.QueryRow("SELECT id FROM tags WHERE name = ?", name).Scan(&tagID); err != nil {
			return err
		}
		if _, err := DB.Exec(
			"INSERT OR IGNORE INTO post_tags (post_id, tag_id) VALUES (?, ?)",
			postID, tagID,
		); err != nil {
			return err
		}
	}
	return nil
}

// GetTagsForPost returns the tag names of a post ordered alphabetically.
func GetTagsForPost(postID int64) ([]string, error) {
	rows, err := DB.Query(`
		SELECT t.name FROM tags t
		JOIN post_tags pt ON pt.tag_id = t.id
		WHERE pt.post_id = ? ORDER BY t.name ASC`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

// ListTagsWithCounts returns every tag still linked to at least one post,
// with its post count, most-used first.
func ListTagsWithCounts() ([]TagWithCount, error) {
	rows, err := DB.Query(`
		SELECT t.name, COUNT(pt.post_id) AS cnt
		FROM tags t
		JOIN post_tags pt ON pt.tag_id = t.id
		GROUP BY t.id
		ORDER BY cnt DESC, t.name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []TagWithCount
	for rows.Next() {
		var tc TagWithCount
		if err := rows.Scan(&tc.Name, &tc.Count); err != nil {
			return nil, err
		}
		tags = append(tags, tc)
	}
	return tags, nil
}

// deleteOrphanTags removes tags no longer linked to any post.
func deleteOrphanTags() {
	DB.Exec("DELETE FROM tags WHERE id NOT IN (SELECT DISTINCT tag_id FROM post_tags)")
}
