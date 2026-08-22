package db

import (
	"testing"
	"time"
)

func TestLegacyPostsDefaultPublished(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	post, err := CreatePost("Legacy", "", "photo", "l.jpg", "l.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	got, _ := GetPost(post.ID)
	if got.Status != PostStatusPublished {
		t.Errorf("legacy row should default to published, got %q", got.Status)
	}
	if got.ScheduledAt != nil {
		t.Errorf("legacy row should have no schedule, got %v", got.ScheduledAt)
	}
	if !got.IsVisible() {
		t.Error("legacy row should be visible")
	}
}

func TestDraftExcludedFromPublicReads(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	draft, _ := CreatePost("Hidden Draft", "", "photo", "d.jpg", "d.jpg", false)
	live, _ := CreatePost("Live Post", "", "photo", "p.jpg", "p.jpg", false)
	SetPostTags(draft.ID, []string{"Ghost"})
	SetPostTags(live.ID, []string{"Real"})

	if err := SetPostStatus(draft.ID, PostStatusDraft, nil); err != nil {
		t.Fatalf("SetPostStatus: %v", err)
	}

	posts, _ := GetPosts(&PostFilter{})
	if len(posts) != 1 || posts[0].ID != live.ID {
		t.Errorf("public list should exclude draft, got %d posts", len(posts))
	}
	total, _ := CountPosts(&PostFilter{})
	if total != 1 {
		t.Errorf("public count should be 1, got %d", total)
	}

	// Admin sees both
	all, _ := GetPosts(&PostFilter{IncludeUnpublished: true})
	if len(all) != 2 {
		t.Errorf("admin list should see 2, got %d", len(all))
	}
	drafts, _ := GetPosts(&PostFilter{IncludeUnpublished: true, Status: PostStatusDraft})
	if len(drafts) != 1 || drafts[0].ID != draft.ID {
		t.Errorf("status=draft filter should return the draft, got %d", len(drafts))
	}

	// Tag counts only count visible posts
	counts, _ := ListTagsWithCounts()
	if len(counts) != 1 || counts[0].Name != "Real" {
		t.Errorf("tag counts should exclude draft-only tags, got %+v", counts)
	}
}

func TestScheduledPublishesLazily(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	post, _ := CreatePost("Scheduled", "", "photo", "s.jpg", "s.jpg", false)

	future := time.Now().UTC().Add(2 * time.Hour)
	if err := SetPostStatus(post.ID, PostStatusPublished, &future); err != nil {
		t.Fatalf("schedule future: %v", err)
	}
	got, _ := GetPost(post.ID)
	if got.IsVisible() {
		t.Error("future-scheduled post must not be visible yet")
	}
	posts, _ := GetPosts(&PostFilter{})
	if len(posts) != 0 {
		t.Errorf("future-scheduled post must be excluded from public list, got %d", len(posts))
	}

	past := time.Now().UTC().Add(-1 * time.Minute)
	SetPostStatus(post.ID, PostStatusPublished, &past)
	got, _ = GetPost(post.ID)
	if !got.IsVisible() {
		t.Error("past-due scheduled post must be visible")
	}
	posts, _ = GetPosts(&PostFilter{})
	if len(posts) != 1 {
		t.Errorf("past-due scheduled post must appear publicly, got %d", len(posts))
	}

	// Unpublish / republish round trip
	SetPostStatus(post.ID, PostStatusDraft, nil)
	got, _ = GetPost(post.ID)
	if got.Status != PostStatusDraft || got.ScheduledAt != nil {
		t.Errorf("unpublish should clear schedule, got %q %v", got.Status, got.ScheduledAt)
	}
	SetPostStatus(post.ID, PostStatusPublished, nil)
	got, _ = GetPost(post.ID)
	if !got.IsVisible() {
		t.Error("republished post should be visible")
	}

	if err := SetPostStatus(post.ID, "bogus", nil); err == nil {
		t.Error("invalid status must error")
	}
}
