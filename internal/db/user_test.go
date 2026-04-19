package db

import (
	"context"
	"testing"
)

func TestUserDAO_Insert_OK(t *testing.T) {
	truncateTables(t)

	ctx := context.Background()
	dao := &UserDAO{pool: testPool}

	u, err := dao.Insert(ctx, "alice")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if u.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if u.Username != "alice" {
		t.Errorf("expected username 'alice', got %q", u.Username)
	}
}

func TestUserDAO_Insert_DuplicateUsername(t *testing.T) {
	truncateTables(t)

	ctx := context.Background()
	dao := &UserDAO{pool: testPool}

	if _, err := dao.Insert(ctx, "alice"); err != nil {
		t.Fatalf("insert first: %v", err)
	}
	if _, err := dao.Insert(ctx, "alice"); err == nil {
		t.Error("expected error on duplicate username, got nil")
	}
}

func TestUserDAO_FindByUsername_Found(t *testing.T) {
	truncateTables(t)

	ctx := context.Background()
	dao := &UserDAO{pool: testPool}

	inserted, err := dao.Insert(ctx, "bob")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	found, err := dao.FindByUsername(ctx, "bob")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found == nil {
		t.Fatal("expected user, got nil")
	}
	if found.ID != inserted.ID {
		t.Errorf("ID mismatch: got %d, want %d", found.ID, inserted.ID)
	}
}

func TestUserDAO_FindByUsername_NotFound(t *testing.T) {
	truncateTables(t)

	ctx := context.Background()
	dao := &UserDAO{pool: testPool}

	u, err := dao.FindByUsername(ctx, "nobody")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u != nil {
		t.Errorf("expected nil, got %+v", u)
	}
}

func TestUserDAO_FindByID_NotFound(t *testing.T) {
	truncateTables(t)

	ctx := context.Background()
	dao := &UserDAO{pool: testPool}

	u, err := dao.FindByID(ctx, 9999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u != nil {
		t.Errorf("expected nil, got %+v", u)
	}
}
