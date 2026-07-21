package db

import (
	"testing"
)

func TestCreateAndGetComment(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	_, err := CreateUser("commenter@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	_, err = CreatePost("Test Post", "", "photo", "test.jpg", "test.jpg", false)
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	comment, err := CreateComment(1, 1, nil, "Great post!")
	if err != nil {
		t.Fatalf("CreateComment: %v", err)
	}

	if comment.Body != "Great post!" {
		t.Errorf("expected body 'Great post!', got %q", comment.Body)
	}
	if comment.UserID != 1 {
		t.Errorf("expected user_id 1, got %d", comment.UserID)
	}
	if comment.PostID != 1 {
		t.Errorf("expected post_id 1, got %d", comment.PostID)
	}
	if comment.ParentID != nil {
		t.Errorf("expected nil parent_id, got %v", comment.ParentID)
	}
}

func TestCreateCommentWithParent(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreateUser("parent@test.com", "password123")
	CreatePost("Test", "", "photo", "t.jpg", "t.jpg", false)

	parent, err := CreateComment(1, 1, nil, "Parent")
	if err != nil {
		t.Fatalf("CreateComment parent: %v", err)
	}

	reply, err := CreateComment(1, 1, &parent.ID, "Reply")
	if err != nil {
		t.Fatalf("CreateComment reply: %v", err)
	}

	if reply.ParentID == nil || *reply.ParentID != parent.ID {
		t.Errorf("expected parent_id %d, got %v", parent.ID, reply.ParentID)
	}
}

func TestGetCommentsByPost(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreateUser("user@test.com", "password123")
	CreatePost("Test", "", "photo", "t.jpg", "t.jpg", false)

	CreateComment(1, 1, nil, "First")
	CreateComment(1, 1, nil, "Second")

	comments, err := GetCommentsByPost(1)
	if err != nil {
		t.Fatalf("GetCommentsByPost: %v", err)
	}

	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}
}

func TestGetCommentsByPost_Nonexistent(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	comments, err := GetCommentsByPost(999)
	if err != nil {
		t.Fatalf("GetCommentsByPost: %v", err)
	}
	if len(comments) != 0 {
		t.Errorf("expected empty comments, got %v", comments)
	}
}

func TestDeleteComment(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreateUser("user@test.com", "password123")
	CreatePost("Test", "", "photo", "t.jpg", "t.jpg", false)

	comment, err := CreateComment(1, 1, nil, "Delete me")
	if err != nil {
		t.Fatalf("CreateComment: %v", err)
	}

	err = DeleteComment(comment.ID)
	if err != nil {
		t.Fatalf("DeleteComment: %v", err)
	}

	_, err = GetComment(comment.ID)
	if err == nil {
		t.Error("expected error after deleting comment")
	}
}

func TestGetAllComments(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreateUser("user@test.com", "password123")
	CreatePost("Test", "", "photo", "t.jpg", "t.jpg", false)
	CreateComment(1, 1, nil, "Admin view")

	comments, err := GetAllComments()
	if err != nil {
		t.Fatalf("GetAllComments: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if comments[0].Body != "Admin view" {
		t.Errorf("expected body 'Admin view', got %q", comments[0].Body)
	}
	if comments[0].PostTitle == "" {
		t.Error("expected post title to be populated")
	}
	if comments[0].Username == "" {
		t.Error("expected username to be populated")
	}
}

// ── Favourites ──

func TestToggleFavourite(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreateUser("user@test.com", "password123")
	CreatePost("Fave", "", "photo", "f.jpg", "f.jpg", false)

	favourited, err := ToggleFavourite(1, 1)
	if err != nil {
		t.Fatalf("ToggleFavourite: %v", err)
	}
	if !favourited {
		t.Error("expected favourited=true after toggle on")
	}

	favourited, err = ToggleFavourite(1, 1)
	if err != nil {
		t.Fatalf("ToggleFavourite (off): %v", err)
	}
	if favourited {
		t.Error("expected favourited=false after toggle off")
	}
}

func TestGetUserFavourited(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreateUser("user@test.com", "password123")
	CreatePost("Fave", "", "photo", "f.jpg", "f.jpg", false)

	faved, err := GetUserFavourited(1, 1)
	if err != nil {
		t.Fatalf("GetUserFavourited: %v", err)
	}
	if faved {
		t.Error("expected not favourited initially")
	}

	ToggleFavourite(1, 1)

	faved, err = GetUserFavourited(1, 1)
	if err != nil {
		t.Fatalf("GetUserFavourited: %v", err)
	}
	if !faved {
		t.Error("expected favourited after toggle")
	}
}

func TestGetFavouriteCount(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreateUser("a@test.com", "password123")
	CreateUser("b@test.com", "password123")
	CreatePost("Fave", "", "photo", "f.jpg", "f.jpg", false)

	count, err := GetFavouriteCount(1)
	if err != nil {
		t.Fatalf("GetFavouriteCount: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	ToggleFavourite(1, 1)
	ToggleFavourite(2, 1)

	count, err = GetFavouriteCount(1)
	if err != nil {
		t.Fatalf("GetFavouriteCount: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestListFavourites(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreateUser("user@test.com", "password123")
	CreatePost("Post 1", "", "photo", "p1.jpg", "p1.jpg", false)
	CreatePost("Post 2", "", "video", "p2.mp4", "p2.jpg", true)

	ToggleFavourite(1, 1)

	faves, err := ListFavourites(1)
	if err != nil {
		t.Fatalf("ListFavourites: %v", err)
	}
	if len(faves) != 1 {
		t.Fatalf("expected 1 favourite, got %d", len(faves))
	}
	if faves[0].Title != "Post 1" {
		t.Errorf("expected 'Post 1', got %q", faves[0].Title)
	}
}

func TestListFavourites_Empty(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreateUser("user@test.com", "password123")

	faves, err := ListFavourites(1)
	if err != nil {
		t.Fatalf("ListFavourites: %v", err)
	}
	if len(faves) != 0 {
		t.Errorf("expected 0, got %d", len(faves))
	}
}

// ── Tips ──

func TestCreateAndCountTips(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreateUser("tipper@test.com", "password123")
	CreatePost("Tip Post", "", "photo", "t.jpg", "t.jpg", false)

	err := CreateTip(1, 1, 500)
	if err != nil {
		t.Fatalf("CreateTip: %v", err)
	}

	count, err := GetTipCount(1)
	if err != nil {
		t.Fatalf("GetTipCount: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 tip, got %d", count)
	}

	total, err := GetTotalTipAmount(1)
	if err != nil {
		t.Fatalf("GetTotalTipAmount: %v", err)
	}
	if total != 500 {
		t.Errorf("expected 500, got %d", total)
	}
}

func TestTipsAccumulate(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreateUser("tipper@test.com", "password123")
	CreatePost("Tip Post", "", "photo", "t.jpg", "t.jpg", false)

	CreateTip(1, 1, 200)
	CreateTip(1, 1, 300)

	count, _ := GetTipCount(1)
	if count != 2 {
		t.Errorf("expected 2 tips, got %d", count)
	}

	total, _ := GetTotalTipAmount(1)
	if total != 500 {
		t.Errorf("expected total 500, got %d", total)
	}
}

func TestHasUserTipped(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreateUser("tipper@test.com", "password123")
	CreatePost("Tip Post", "", "photo", "t.jpg", "t.jpg", false)

	tipped, err := HasUserTipped(1, 1)
	if err != nil {
		t.Fatalf("HasUserTipped: %v", err)
	}
	if tipped {
		t.Error("expected not tipped initially")
	}

	CreateTip(1, 1, 100)

	tipped, err = HasUserTipped(1, 1)
	if err != nil {
		t.Fatalf("HasUserTipped: %v", err)
	}
	if !tipped {
		t.Error("expected tipped after tip")
	}
}

func TestGetTipCount_NoTips(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreatePost("Post", "", "photo", "p.jpg", "p.jpg", false)

	count, err := GetTipCount(1)
	if err != nil {
		t.Fatalf("GetTipCount: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

// ── Purchases ──

func TestCreateAndHasPurchase(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreateUser("buyer@test.com", "password123")
	CreatePost("Locked Post", "", "video", "locked.mp4", "locked.jpg", true)

	err := CreatePurchase(1, 1, 20)
	if err != nil {
		t.Fatalf("CreatePurchase: %v", err)
	}

	purchased, err := HasUserPurchased(1, 1)
	if err != nil {
		t.Fatalf("HasUserPurchased: %v", err)
	}
	if !purchased {
		t.Error("expected purchased=true")
	}

	// Verify another user hasn't purchased
	purchased, err = HasUserPurchased(999, 1)
	if err != nil {
		t.Fatalf("HasUserPurchased: %v", err)
	}
	if purchased {
		t.Error("expected purchased=false for other user")
	}
}

func TestHasUserPurchased_NoPurchase(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreateUser("user@test.com", "password123")
	CreatePost("Post", "", "photo", "p.jpg", "p.jpg", false)

	purchased, err := HasUserPurchased(1, 1)
	if err != nil {
		t.Fatalf("HasUserPurchased: %v", err)
	}
	if purchased {
		t.Error("expected purchased=false by default")
	}
}

// ── Post Links ──

func TestSetGetPostLinks(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreatePost("Parent", "", "photo", "p.jpg", "p.jpg", false)
	CreatePost("Child", "", "photo", "c.jpg", "c.jpg", false)

	err := SetPostLinks(1, []int64{2})
	if err != nil {
		t.Fatalf("SetPostLinks: %v", err)
	}

	links, err := GetPostLinks(1)
	if err != nil {
		t.Fatalf("GetPostLinks: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].LinkedPostID != 2 {
		t.Errorf("expected linked_post_id 2, got %d", links[0].LinkedPostID)
	}
}

func TestGetPostLinks_None(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreatePost("Lonely", "", "photo", "l.jpg", "l.jpg", false)

	links, err := GetPostLinks(1)
	if err != nil {
		t.Fatalf("GetPostLinks: %v", err)
	}
	if len(links) != 0 {
		t.Errorf("expected 0 links, got %d", len(links))
	}
}

func TestSetPostLinks_Replace(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreatePost("Post A", "", "photo", "a.jpg", "a.jpg", false)
	CreatePost("Post B", "", "photo", "b.jpg", "b.jpg", false)
	CreatePost("Post C", "", "photo", "c.jpg", "c.jpg", false)

	SetPostLinks(1, []int64{2, 3})

	links, _ := GetPostLinks(1)
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}

	// Replace with just one
	SetPostLinks(1, []int64{3})

	links, _ = GetPostLinks(1)
	if len(links) != 1 {
		t.Fatalf("expected 1 link after replace, got %d", len(links))
	}
	if links[0].LinkedPostID != 3 {
		t.Errorf("expected linked_post_id 3, got %d", links[0].LinkedPostID)
	}
}

func TestSetPostLinks_Empty(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreatePost("Post", "", "photo", "p.jpg", "p.jpg", false)

	// Set empty list
	err := SetPostLinks(1, []int64{})
	if err != nil {
		t.Fatalf("SetPostLinks empty: %v", err)
	}

	links, _ := GetPostLinks(1)
	if len(links) != 0 {
		t.Errorf("expected 0 links, got %d", len(links))
	}
}

// ── Sessions ──

func TestCreateGetUserSession(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreateUser("user@test.com", "password123")

	session, err := CreateUserSession(1)
	if err != nil {
		t.Fatalf("CreateUserSession: %v", err)
	}

	if session.Token == "" {
		t.Error("expected non-empty token")
	}
	if session.UserID != 1 {
		t.Errorf("expected user_id 1, got %d", session.UserID)
	}

	got, err := GetUserSession(session.Token)
	if err != nil {
		t.Fatalf("GetUserSession: %v", err)
	}
	if got.UserID != 1 {
		t.Errorf("expected user_id 1, got %d", got.UserID)
	}
}

func TestGetUserSession_InvalidToken(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	_, err := GetUserSession("invalid-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestDeleteUserSession(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreateUser("user@test.com", "password123")
	session, _ := CreateUserSession(1)

	err := DeleteUserSession(session.Token)
	if err != nil {
		t.Fatalf("DeleteUserSession: %v", err)
	}

	_, err = GetUserSession(session.Token)
	if err == nil {
		t.Error("expected error after deleting session")
	}
}

func TestLegacySessions(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreateUser("admin", "password123")

	session, err := CreateSession(1)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := GetSession(session.Token)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.UserID != 1 {
		t.Errorf("expected user_id 1, got %d", got.UserID)
	}

	err = DeleteSession(session.Token)
	if err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	_, err = GetSession(session.Token)
	if err == nil {
		t.Error("expected error after deleting legacy session")
	}
}

// ── User Queries ──

func TestGetUserByEmail(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	id, err := CreateUser("findme@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	user, err := GetUserByEmail("findme@test.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if user.ID != id {
		t.Errorf("expected ID %d, got %d", id, user.ID)
	}

	_, err = GetUserByEmail("nonexistent@test.com")
	if err == nil {
		t.Error("expected error for nonexistent email")
	}
}

func TestUpdateUserPaid(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreateUser("user@test.com", "password123")

	err := UpdateUserPaid(1, true)
	if err != nil {
		t.Fatalf("UpdateUserPaid: %v", err)
	}

	user, err := GetUserByID(1)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if !user.Paid {
		t.Error("expected paid=true")
	}
}

func TestUpdateUserRole(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreateUser("user@test.com", "password123")

	err := UpdateUserRole(1, "admin")
	if err != nil {
		t.Fatalf("UpdateUserRole: %v", err)
	}

	user, err := GetUserByID(1)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if user.Role != "admin" {
		t.Errorf("expected role 'admin', got %q", user.Role)
	}
}

func TestListUsers(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreateUser("a@test.com", "password123")
	CreateUser("b@test.com", "password123")

	users, err := ListUsers()
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}

func TestUpdateUserEmail(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	CreateUser("old@test.com", "password123")

	err := UpdateUserEmail(1, "new@test.com")
	if err != nil {
		t.Fatalf("UpdateUserEmail: %v", err)
	}

	user, err := GetUserByID(1)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if user.Email != "new@test.com" {
		t.Errorf("expected email 'new@test.com', got %q", user.Email)
	}
}

func TestDeleteUser(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	// Create a first user to act as a "second admin" for the last-admin check
	adminID, err := CreateUser("admin@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	// Make it an admin so the check passes
	UpdateUserRole(adminID, "admin")

	// Now create and delete a normal user
	normalID, err := CreateUser("delete_me@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	err = DeleteUser(normalID)
	if err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	_, err = GetUserByID(normalID)
	if err == nil {
		t.Error("expected error after deleting user")
	}
}

func TestDeleteUser_LastAdmin(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	// The only user is the one we're creating (no other admins)
	_, err := CreateUser("only_user@test.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	// Make it admin
	UpdateUserRole(1, "admin")

	err = DeleteUser(1)
	if err == nil {
		t.Error("expected error when deleting the last admin")
	}
}
