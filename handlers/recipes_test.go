package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"redchef/db"
)

func createRecipePost(t *testing.T, title string, locked bool) *db.Post {
	t.Helper()
	post, err := db.CreatePost(title, "A tasty description", "photo", title+".jpg", title+".jpg", locked)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	recipe := db.RecipeData{
		Ingredients: []string{"200g noodles", "1L broth"},
		Steps:       []string{"Boil", "Simmer", "Serve"},
		Servings:    2,
		PrepMinutes: 10,
		CookMinutes: 15,
	}
	if err := db.UpdatePostRecipe(post.ID, recipe); err != nil {
		t.Fatalf("UpdatePostRecipe: %v", err)
	}
	if err := db.SetPostTags(post.ID, []string{"Ramen", "Quick"}); err != nil {
		t.Fatalf("SetPostTags: %v", err)
	}
	return post
}

func TestListPosts_SearchTagPagination(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	createRecipePost(t, "Ramen Deluxe", false)
	db.CreatePost("Plain Toast", "", "photo", "toast.jpg", "toast.jpg", false)
	for i := 0; i < 30; i++ {
		db.CreatePost("Filler", "", "photo", "f.jpg", "f.jpg", false)
	}

	// Search by q
	resp := httptest.NewRecorder()
	ListPosts(resp, httptest.NewRequest("GET", "/api/posts?q=ramen", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("q search: %d", resp.Code)
	}
	var posts []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&posts)
	if len(posts) != 1 {
		t.Fatalf("q=ramen expected 1 post, got %d", len(posts))
	}

	// Tag filter (case-insensitive)
	resp = httptest.NewRecorder()
	ListPosts(resp, httptest.NewRequest("GET", "/api/posts?tag=ramen", nil))
	json.NewDecoder(resp.Body).Decode(&posts)
	if len(posts) != 1 || posts[0]["title"] != "Ramen Deluxe" {
		t.Fatalf("tag=ramen expected Ramen Deluxe, got %v", posts)
	}

	// Pagination + X-Total-Count
	resp = httptest.NewRecorder()
	ListPosts(resp, httptest.NewRequest("GET", "/api/posts?limit=10&offset=5", nil))
	total := resp.Header().Get("X-Total-Count")
	if total != "32" {
		t.Errorf("X-Total-Count expected 32, got %q", total)
	}
	posts = nil
	json.NewDecoder(resp.Body).Decode(&posts)
	if len(posts) != 10 {
		t.Errorf("limit=10 expected 10 posts, got %d", len(posts))
	}

	// Out-of-range clamps: limit=500 → max 100 returned, offset=-5 → 0
	resp = httptest.NewRecorder()
	ListPosts(resp, httptest.NewRequest("GET", "/api/posts?limit=500&offset=-5", nil))
	posts = nil
	json.NewDecoder(resp.Body).Decode(&posts)
	if len(posts) != 32 {
		t.Errorf("clamped request should return all 32, got %d", len(posts))
	}
}

func TestListPosts_LockedWithholdsRecipe(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	createRecipePost(t, "Secret Dish", true)

	resp := httptest.NewRecorder()
	ListPosts(resp, httptest.NewRequest("GET", "/api/posts", nil))
	var posts []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&posts)

	recipe := posts[0]["recipe"].(map[string]interface{})
	if ing := recipe["ingredients"].([]interface{}); len(ing) != 0 {
		t.Errorf("locked post list response must not expose ingredients, got %v", ing)
	}
	if st := recipe["steps"].([]interface{}); len(st) != 0 {
		t.Errorf("locked post list response must not expose steps, got %v", st)
	}
}

func TestGetPost_LockedWithholdsRecipe_UnlockedShowsIt(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	post := createRecipePost(t, "Borscht", true)

	// Anonymous detail view
	req := httptest.NewRequest("GET", "/api/posts/1", nil)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	GetPost(resp, req)
	var detail map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&detail)
	recipe := detail["recipe"].(map[string]interface{})
	if len(recipe["ingredients"].([]interface{})) != 0 {
		t.Error("locked detail must withhold ingredients")
	}

	// Admin (authenticated) sees everything
	req = authenticatedRequest(t, "GET", "/api/posts/1", nil)
	req.SetPathValue("id", "1")
	resp = httptest.NewRecorder()
	GetPost(resp, req)
	detail = nil
	json.NewDecoder(resp.Body).Decode(&detail)
	recipe = detail["recipe"].(map[string]interface{})
	if len(recipe["ingredients"].([]interface{})) != 2 {
		t.Errorf("unlocked detail should show ingredients, got %v", recipe["ingredients"])
	}
	if len(recipe["steps"].([]interface{})) != 3 {
		t.Errorf("unlocked detail should show steps, got %v", recipe["steps"])
	}
	if tags, ok := detail["tags"].([]interface{}); !ok || len(tags) != 2 {
		t.Errorf("detail should include tags, got %v", detail["tags"])
	}
	_ = post
}

func TestGetTagsEndpoint(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	createRecipePost(t, "Tagged Post", false)

	resp := httptest.NewRecorder()
	ListTags(resp, httptest.NewRequest("GET", "/api/tags", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("/api/tags: %d", resp.Code)
	}
	var tags []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&tags)
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %v", tags)
	}
	found := map[string]float64{}
	for _, tag := range tags {
		found[tag["name"].(string)] = tag["count"].(float64)
	}
	if found["Ramen"] != 1 || found["Quick"] != 1 {
		t.Errorf("unexpected tag counts: %v", found)
	}
}

func TestRenderPostPage_JSONLD(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	indexHTML := []byte(`<!DOCTYPE html><html><head><title>Shell</title><!--#meta--></head><body></body></html>`)

	unlocked := createRecipePost(t, "Open Recipe", false)

	req := httptest.NewRequest("GET", "/posts/1", nil)
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	RenderPostPage(resp, req, indexHTML)

	body := resp.Body.String()
	if !strings.Contains(body, `"@type":"Recipe"`) {
		t.Error("unlocked page should contain Recipe JSON-LD")
	}
	if !strings.Contains(body, "recipeIngredient") || !strings.Contains(body, "200g noodles") {
		t.Error("unlocked JSON-LD should include ingredients")
	}
	if !strings.Contains(body, `og:title`) || !strings.Contains(body, "Open Recipe") {
		t.Error("page should include og:title with post title")
	}
	if !strings.Contains(body, `og:image`) {
		t.Error("page should include og:image")
	}
	if strings.Contains(body, "<!--#meta-->") {
		t.Error("placeholder should be replaced")
	}
	_ = unlocked

	// Locked post: no ingredient/step leakage in JSON-LD
	createRecipePost(t, "Locked Recipe", true)
	req = httptest.NewRequest("GET", "/posts/2", nil)
	req.SetPathValue("id", "2")
	resp = httptest.NewRecorder()
	RenderPostPage(resp, req, indexHTML)
	body = resp.Body.String()
	if strings.Contains(body, "recipeIngredient") || strings.Contains(body, "noodles") {
		t.Error("locked JSON-LD must not contain ingredients")
	}
	if !strings.Contains(body, "Locked Recipe") {
		t.Error("locked JSON-LD should still carry the title")
	}
}

func TestAdminUpdatePost_RecipeAndTags(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	post, _ := db.CreatePost("Editable", "", "photo", "e.jpg", "e.jpg", false)

	// Valid update with recipe + tags
	body := `{"recipe":{"ingredients":["a","b"],"steps":["do"],"servings":3,"prep_minutes":5,"cook_minutes":8},"tags":["Dinner"," Fast "]}`
	req := authenticatedRequest(t, "PATCH", "/api/admin/posts/1", strings.NewReader(body))
	req.SetPathValue("id", "1")
	resp := httptest.NewRecorder()
	AdminUpdatePost(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("PATCH with recipe+tags: %d %s", resp.Code, resp.Body.String())
	}
	got, _ := db.GetPost(post.ID)
	if len(got.Recipe.Ingredients) != 2 || got.Recipe.Servings != 3 {
		t.Errorf("recipe not persisted: %+v", got.Recipe)
	}
	tags, _ := db.GetTagsForPost(post.ID)
	if len(tags) != 2 || tags[0] != "Dinner" || tags[1] != "Fast" {
		t.Errorf("tags not normalized/persisted: %v", tags)
	}

	// Invalid recipe rejected with 400 naming the field
	long := strings.Repeat("x", 201)
	body = `{"recipe":{"ingredients":["` + long + `"]}}`
	req = authenticatedRequest(t, "PATCH", "/api/admin/posts/1", strings.NewReader(body))
	req.SetPathValue("id", "1")
	resp = httptest.NewRecorder()
	AdminUpdatePost(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Errorf("oversized ingredient should 400, got %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "ingredients") {
		t.Errorf("error should name offending field, got: %s", resp.Body.String())
	}

	// Clearing recipe via empty object works
	body = `{"recipe":{"ingredients":[],"steps":[],"servings":0,"prep_minutes":0,"cook_minutes":0}}`
	req = authenticatedRequest(t, "PATCH", "/api/admin/posts/1", strings.NewReader(body))
	req.SetPathValue("id", "1")
	resp = httptest.NewRecorder()
	AdminUpdatePost(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("clear recipe: %d", resp.Code)
	}
	got, _ = db.GetPost(post.ID)
	if !got.Recipe.IsZero() {
		t.Errorf("recipe should be cleared, got %+v", got.Recipe)
	}
}

func TestAdminUpload_RecipeAndTagsFormFields(t *testing.T) {
	cleanup := setupHandlerTest(t)
	defer cleanup()

	// Build a multipart upload with recipe + tags form fields.
	var buf strings.Builder
	boundary := "----redchefboundary"
	buf.WriteString("--" + boundary + "\r\n")
	writeField := func(name, value string) {
		buf.WriteString(`Content-Disposition: form-data; name="` + name + `"` + "\r\n\r\n")
		buf.WriteString(value + "\r\n")
		buf.WriteString("--" + boundary + "\r\n")
	}
	writeField("title", "Uploaded Recipe")
	writeField("recipe", `{"ingredients":["flour"],"servings":1}`)
	writeField("tags", "Baking, Bread")
	buf.WriteString(`Content-Disposition: form-data; name="file"; filename="cake.jpg"` + "\r\n")
	buf.WriteString("Content-Type: image/jpeg\r\n\r\n")
	buf.WriteString("\x00\x01\x02fakejpegdata\r\n")
	buf.WriteString("--" + boundary + "--\r\n")

	req := authenticatedRequest(t, "POST", "/api/admin/upload", strings.NewReader(buf.String()))
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	resp := httptest.NewRecorder()
	AdminUpload(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", resp.Code, resp.Body.String())
	}
	ProcessWG.Wait()

	posts, _ := db.GetPosts(nil)
	if len(posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(posts))
	}
	p := posts[0]
	if p.Recipe.Servings != 1 || len(p.Recipe.Ingredients) != 1 {
		t.Errorf("upload recipe not persisted: %+v", p.Recipe)
	}
	tags, _ := db.GetTagsForPost(p.ID)
	if len(tags) != 2 || tags[0] != "Baking" || tags[1] != "Bread" {
		t.Errorf("upload tags not persisted: %v", tags)
	}

	// Invalid recipe JSON on upload is rejected
	buf.Reset()
	buf.WriteString("--" + boundary + "\r\n")
	writeField("title", "Bad Recipe")
	writeField("recipe", `{not-json`)
	buf.WriteString(`Content-Disposition: form-data; name="file"; filename="pie.jpg"` + "\r\n")
	buf.WriteString("Content-Type: image/jpeg\r\n\r\n")
	buf.WriteString("\x00\x01\x02fake\r\n")
	buf.WriteString("--" + boundary + "--\r\n")

	req = authenticatedRequest(t, "POST", "/api/admin/upload", strings.NewReader(buf.String()))
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	resp = httptest.NewRecorder()
	AdminUpload(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Errorf("invalid recipe JSON should 400, got %d", resp.Code)
	}
}
