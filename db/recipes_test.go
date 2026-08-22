package db

import (
	"strings"
	"testing"
)

func TestRecipeRoundTrip(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	post, err := CreatePost("Pasta", "desc", "photo", "p.jpg", "p.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	recipe := RecipeData{
		Ingredients: []string{"200g flour", "2 eggs"},
		Steps:       []string{"Mix", "Knead", "Rest 1h"},
		Servings:    4,
		PrepMinutes: 20,
		CookMinutes: 45,
	}
	if err := UpdatePostRecipe(post.ID, recipe); err != nil {
		t.Fatalf("UpdatePostRecipe: %v", err)
	}

	got, err := GetPost(post.ID)
	if err != nil {
		t.Fatalf("GetPost: %v", err)
	}
	if got.Recipe.Servings != 4 || got.Recipe.PrepMinutes != 20 || got.Recipe.CookMinutes != 45 {
		t.Errorf("scalar fields mismatch: %+v", got.Recipe)
	}
	if len(got.Recipe.Ingredients) != 2 || got.Recipe.Ingredients[0] != "200g flour" {
		t.Errorf("ingredients mismatch: %v", got.Recipe.Ingredients)
	}
	if len(got.Recipe.Steps) != 3 || got.Recipe.Steps[2] != "Rest 1h" {
		t.Errorf("steps order/content mismatch: %v", got.Recipe.Steps)
	}
}

func TestRecipeClearAndMalformed(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	post, _ := CreatePost("Soup", "", "photo", "s.jpg", "s.jpg", false)

	if err := UpdatePostRecipe(post.ID, RecipeData{Ingredients: []string{"water"}}); err != nil {
		t.Fatalf("set recipe: %v", err)
	}
	// Zero recipe clears the stored JSON.
	if err := UpdatePostRecipe(post.ID, RecipeData{}); err != nil {
		t.Fatalf("clear recipe: %v", err)
	}
	got, _ := GetPost(post.ID)
	if !got.Recipe.IsZero() {
		t.Errorf("expected zero recipe after clear, got %+v", got.Recipe)
	}

	// Malformed JSON in the column must not break reads.
	DB.Exec("UPDATE posts SET recipe_json = '{not json' WHERE id = ?", post.ID)
	got, err := GetPost(post.ID)
	if err != nil {
		t.Fatalf("GetPost with malformed recipe_json: %v", err)
	}
	if !got.Recipe.IsZero() {
		t.Errorf("malformed recipe_json should parse to zero value, got %+v", got.Recipe)
	}
}

func TestValidateRecipeData(t *testing.T) {
	long := strings.Repeat("x", 201)
	cases := []struct {
		name    string
		recipe  RecipeData
		wantErr bool
	}{
		{"empty ok", RecipeData{}, false},
		{"valid full", RecipeData{Ingredients: []string{"a"}, Steps: []string{"b"}, Servings: 2, PrepMinutes: 5, CookMinutes: 10}, false},
		{"too many ingredients", RecipeData{Ingredients: make([]string, 51)}, true},
		{"oversized ingredient", RecipeData{Ingredients: []string{long}}, true},
		{"oversized step", RecipeData{Steps: []string{long}}, true},
		{"negative servings", RecipeData{Servings: -1}, true},
		{"crazy minutes", RecipeData{CookMinutes: 10001}, true},
	}
	for _, tc := range cases {
		err := ValidateRecipeData(&tc.recipe)
		if (err != nil) != tc.wantErr {
			t.Errorf("%s: wantErr=%v got %v", tc.name, tc.wantErr, err)
		}
	}
}

func TestTagsNormalizeAndUniqueness(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	post, _ := CreatePost("Tagged", "", "photo", "t.jpg", "t.jpg", false)

	// Duplicates differing only by case/whitespace collapse to one tag.
	names := []string{"Sauces", "  quick ", "SAUCES", ""}
	if err := SetPostTags(post.ID, names); err != nil {
		t.Fatalf("SetPostTags: %v", err)
	}

	tags, err := GetTagsForPost(post.ID)
	if err != nil {
		t.Fatalf("GetTagsForPost: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 unique tags, got %v", tags)
	}

	// Case-insensitive lookup via the filter path.
	filter := &PostFilter{Tag: "sauces"}
	posts, err := GetPosts(filter)
	if err != nil {
		t.Fatalf("GetPosts by tag: %v", err)
	}
	if len(posts) != 1 {
		t.Errorf("tag filter 'sauces' should match post tagged 'Sauces', got %d", len(posts))
	}

	// Replacing the set removes old links.
	if err := SetPostTags(post.ID, []string{"only"}); err != nil {
		t.Fatalf("SetPostTags replace: %v", err)
	}
	tags, _ = GetTagsForPost(post.ID)
	if len(tags) != 1 || tags[0] != "only" {
		t.Errorf("expected [only], got %v", tags)
	}
	counts, _ := ListTagsWithCounts()
	if len(counts) != 1 || counts[0].Name != "only" || counts[0].Count != 1 {
		t.Errorf("unexpected tag counts: %+v", counts)
	}
}

func TestFilterCompositionAndPagination(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	// p1: photo ramen; p2: video ramen tagged Quick; p3: video other
	CreatePost("Ramen Bowl", "noodles", "photo", "1.jpg", "1.jpg", false)
	p2, _ := CreatePost("Spicy Ramen", "hot soup", "video", "2.mp4", "2.jpg", false)
	p3, _ := CreatePost("Lasagna", "pasta bake", "video", "3.mp4", "3.jpg", false)
	SetPostTags(p2.ID, []string{"Quick"})
	UpdatePostRecipe(p3.ID, RecipeData{Ingredients: []string{"pasta sheets secret"}})

	// q matches title
	posts, _ := GetPosts(&PostFilter{Q: "ramen"})
	if len(posts) != 2 {
		t.Errorf("q=ramen expected 2, got %d", len(posts))
	}
	// q matches ingredients text
	posts, _ = GetPosts(&PostFilter{Q: "secret"})
	if len(posts) != 1 {
		t.Errorf("q over recipe_json expected 1, got %d", len(posts))
	}
	// q composes with type
	posts, _ = GetPosts(&PostFilter{Q: "ramen", Type: "video"})
	if len(posts) != 1 || posts[0].ID != p2.ID {
		t.Errorf("q+type expected only p2, got %d", len(posts))
	}
	// q composes with date_from far future → none
	posts, _ = GetPosts(&PostFilter{Q: "ramen", DateFrom: "2999-01-01"})
	if len(posts) != 0 {
		t.Errorf("q+future date expected 0, got %d", len(posts))
	}
	// tag filter
	posts, _ = GetPosts(&PostFilter{Tag: "Quick"})
	if len(posts) != 1 || posts[0].ID != p2.ID {
		t.Errorf("tag filter expected p2, got %d", len(posts))
	}
	// LIKE wildcards in q are escaped, not treated as patterns
	posts, _ = GetPosts(&PostFilter{Q: "%"})
	if len(posts) != 0 {
		t.Errorf("literal %% should match nothing, got %d", len(posts))
	}

	// Pagination is opt-in
	all, _ := GetPosts(&PostFilter{})
	if len(all) != 3 {
		t.Fatalf("no-limit should return all 3, got %d", len(all))
	}
	page, _ := GetPosts(&PostFilter{Limit: 2})
	if len(page) != 2 {
		t.Errorf("limit=2 expected 2, got %d", len(page))
	}
	page2, _ := GetPosts(&PostFilter{Limit: 2, Offset: 2})
	if len(page2) != 1 {
		t.Errorf("limit=2 offset=2 expected 1, got %d", len(page2))
	}
	total, _ := CountPosts(&PostFilter{})
	if total != 3 {
		t.Errorf("CountPosts expected 3, got %d", total)
	}
	total, _ = CountPosts(&PostFilter{Q: "ramen"})
	if total != 2 {
		t.Errorf("CountPosts(q=ramen) expected 2, got %d", total)
	}
}
