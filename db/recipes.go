package db

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RecipeData holds optional structured recipe metadata for a post. It is
// stored as a single JSON blob in posts.recipe_json (” when absent).
type RecipeData struct {
	Ingredients []string `json:"ingredients"`
	Steps       []string `json:"steps"`
	Servings    int      `json:"servings"`
	PrepMinutes int      `json:"prep_minutes"`
	CookMinutes int      `json:"cook_minutes"`
}

// Recipe validation bounds.
const (
	maxRecipeItems    = 50
	maxRecipeItemLen  = 200
	maxRecipeServings = 10000
	maxRecipeMinutes  = 10000
)

// IsZero reports whether the recipe carries no meaningful data.
func (r RecipeData) IsZero() bool {
	return len(r.Ingredients) == 0 && len(r.Steps) == 0 &&
		r.Servings == 0 && r.PrepMinutes == 0 && r.CookMinutes == 0
}

// ValidateRecipeData enforces bounds so a single post can't bloat the DB or
// the JSON-LD output. Returns an error naming the offending field.
func ValidateRecipeData(r *RecipeData) error {
	if err := validateRecipeList("ingredients", r.Ingredients); err != nil {
		return err
	}
	if err := validateRecipeList("steps", r.Steps); err != nil {
		return err
	}
	if r.Servings < 0 || r.Servings > maxRecipeServings {
		return fmt.Errorf("servings must be between 0 and %d", maxRecipeServings)
	}
	if r.PrepMinutes < 0 || r.PrepMinutes > maxRecipeMinutes {
		return fmt.Errorf("prep_minutes must be between 0 and %d", maxRecipeMinutes)
	}
	if r.CookMinutes < 0 || r.CookMinutes > maxRecipeMinutes {
		return fmt.Errorf("cook_minutes must be between 0 and %d", maxRecipeMinutes)
	}
	return nil
}

func validateRecipeList(field string, items []string) error {
	if len(items) > maxRecipeItems {
		return fmt.Errorf("%s: at most %d items allowed", field, maxRecipeItems)
	}
	for i, item := range items {
		if len(item) > maxRecipeItemLen {
			return fmt.Errorf("%s[%d]: exceeds %d characters", field, i, maxRecipeItemLen)
		}
	}
	return nil
}

// NormalizeRecipe trims whitespace on list items and drops empties.
func NormalizeRecipe(r *RecipeData) {
	r.Ingredients = normalizeStringList(r.Ingredients)
	r.Steps = normalizeStringList(r.Steps)
}

func normalizeStringList(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

// parseRecipeJSON decodes a posts.recipe_json value. Empty or malformed
// values yield the zero RecipeData rather than an error — old rows and
// legacy data must never break reads.
func parseRecipeJSON(s string) RecipeData {
	var r RecipeData
	if strings.TrimSpace(s) == "" {
		return RecipeData{}
	}
	if err := json.Unmarshal([]byte(s), &r); err != nil {
		return RecipeData{}
	}
	return r
}

// UpdatePostRecipe stores recipe metadata for a post. A zero-valued recipe
// clears the stored JSON.
func UpdatePostRecipe(postID int64, r RecipeData) error {
	NormalizeRecipe(&r)
	value := ""
	if !r.IsZero() {
		b, err := json.Marshal(r)
		if err != nil {
			return err
		}
		value = string(b)
	}
	_, err := DB.Exec("UPDATE posts SET recipe_json = ? WHERE id = ?", value, postID)
	return err
}
