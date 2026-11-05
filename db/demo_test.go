package db

import "testing"

func TestEnsureDemoUserSkipsEmptyInstall(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	created, err := EnsureDemoUser("demo@redchef.local", "redchef-demo")
	if err != nil {
		t.Fatalf("EnsureDemoUser on empty install: %v", err)
	}
	if created {
		t.Fatal("demo account must not be created on a fresh install")
	}
	if _, err := GetUserByEmail("demo@redchef.local"); err == nil {
		t.Fatal("demo account exists despite empty install")
	}
}

func TestEnsureDemoUserIdempotentAndRepairs(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	if _, err := CreateUser("admin@example.com", "secret12"); err != nil {
		t.Fatalf("create first user: %v", err)
	}

	created, err := EnsureDemoUser(" Demo@RedChef.Local ", "redchef-demo")
	if err != nil {
		t.Fatalf("EnsureDemoUser: %v", err)
	}
	if !created {
		t.Fatal("expected demo account to be created")
	}

	user, err := GetUserByEmail("demo@redchef.local")
	if err != nil {
		t.Fatalf("get demo: %v", err)
	}
	user, err = GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("get demo by id: %v", err)
	}
	if user.Role != "demo" || !user.Paid || user.Username != "demo@redchef.local" {
		t.Fatalf("unexpected demo account: %+v", user)
	}
	if !CheckPassword("redchef-demo", user.PasswordHash) {
		t.Fatal("demo password not stored correctly")
	}

	created, err = EnsureDemoUser("demo@redchef.local", "redchef-demo")
	if err != nil {
		t.Fatalf("second EnsureDemoUser: %v", err)
	}
	if created {
		t.Fatal("second call should not report a new account")
	}
	again, err := GetUserByEmail("demo@redchef.local")
	if err != nil {
		t.Fatalf("get demo again: %v", err)
	}
	if again.ID != user.ID {
		t.Fatalf("demo account was recreated: %d != %d", again.ID, user.ID)
	}

	if err := UpdateUserPassword(user.ID, "drifted-password"); err != nil {
		t.Fatalf("drift password: %v", err)
	}
	if _, err := DB.Exec("UPDATE users SET username = 'stale-demo', paid = 0 WHERE id = ?", user.ID); err != nil {
		t.Fatalf("drift fields: %v", err)
	}
	if _, err := EnsureDemoUser("demo@redchef.local", "redchef-demo"); err != nil {
		t.Fatalf("repair EnsureDemoUser: %v", err)
	}
	repaired, err := GetUserByEmail("demo@redchef.local")
	if err != nil {
		t.Fatalf("get repaired demo: %v", err)
	}
	if repaired.Role != "demo" || !repaired.Paid {
		t.Fatalf("demo not repaired: %+v", repaired)
	}
	if !CheckPassword("redchef-demo", repaired.PasswordHash) {
		t.Fatal("demo password not repaired")
	}
}

func TestEnsureDemoUserRefusesAdminAndBadInput(t *testing.T) {
	setupTestDB(t)
	defer cleanupTestDB(t)

	if _, err := CreateUser("admin@example.com", "secret12"); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if _, err := EnsureDemoUser("admin@example.com", "redchef-demo"); err == nil {
		t.Fatal("expected error when demo email belongs to an admin")
	}
	if _, err := CreateUser("demo@redchef.local", "secret12"); err != nil {
		t.Fatalf("create colliding normal user: %v", err)
	}
	if _, err := EnsureDemoUser("demo@redchef.local", "redchef-demo"); err == nil {
		t.Fatal("expected error when demo email belongs to a normal member")
	}
	collision, err := GetUserByEmail("demo@redchef.local")
	if err != nil {
		t.Fatalf("get colliding user: %v", err)
	}
	if collision.Role != "normal" || !CheckPassword("secret12", collision.PasswordHash) {
		t.Fatalf("normal member was modified: %+v", collision)
	}
	if _, err := EnsureDemoUser("", "redchef-demo"); err == nil {
		t.Fatal("expected error for empty email")
	}
	if _, err := EnsureDemoUser("demo@redchef.local", ""); err == nil {
		t.Fatal("expected error for empty password")
	}
	if _, err := EnsureDemoUser("demo@redchef.local", "short"); err == nil {
		t.Fatal("expected error for short password")
	}
}
