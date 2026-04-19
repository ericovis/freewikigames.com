package db

import (
	"context"
	"testing"
)

func insertTestUser(t *testing.T, ctx context.Context, dao *UserDAO, username string) *User {
	t.Helper()
	u, err := dao.Insert(ctx, username)
	if err != nil {
		t.Fatalf("insert user %q: %v", username, err)
	}
	return u
}

func TestGameSessionDAO_Insert_Solo(t *testing.T) {
	truncateTables(t)

	ctx := context.Background()
	dao := &GameSessionDAO{pool: testPool}

	s, err := dao.Insert(ctx, "solo", "en", nil)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if s.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if s.Mode != "solo" {
		t.Errorf("expected mode 'solo', got %q", s.Mode)
	}
	if s.Status != "waiting" {
		t.Errorf("expected status 'waiting', got %q", s.Status)
	}
	if s.Language != "en" {
		t.Errorf("expected language 'en', got %q", s.Language)
	}
}

func TestGameSessionDAO_UpdateStatus(t *testing.T) {
	truncateTables(t)

	ctx := context.Background()
	dao := &GameSessionDAO{pool: testPool}

	s, err := dao.Insert(ctx, "solo", "en", nil)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := dao.UpdateStatus(ctx, s.ID, "active"); err != nil {
		t.Fatalf("update status: %v", err)
	}

	updated, err := dao.FindByID(ctx, s.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if updated.Status != "active" {
		t.Errorf("expected 'active', got %q", updated.Status)
	}
}

func TestGameSessionDAO_InsertParticipant_And_List(t *testing.T) {
	truncateTables(t)

	ctx := context.Background()
	sessionDAO := &GameSessionDAO{pool: testPool}
	userDAO := &UserDAO{pool: testPool}

	alice := insertTestUser(t, ctx, userDAO, "alice")
	bob := insertTestUser(t, ctx, userDAO, "bob")

	s, err := sessionDAO.Insert(ctx, "multiplayer", "en", nil)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	if _, err := sessionDAO.InsertParticipant(ctx, s.ID, alice.ID); err != nil {
		t.Fatalf("add alice: %v", err)
	}
	if _, err := sessionDAO.InsertParticipant(ctx, s.ID, bob.ID); err != nil {
		t.Fatalf("add bob: %v", err)
	}

	participants, err := sessionDAO.ListParticipants(ctx, s.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(participants) != 2 {
		t.Errorf("expected 2 participants, got %d", len(participants))
	}
}

func TestGameSessionDAO_IncrementScore(t *testing.T) {
	truncateTables(t)

	ctx := context.Background()
	sessionDAO := &GameSessionDAO{pool: testPool}
	userDAO := &UserDAO{pool: testPool}

	alice := insertTestUser(t, ctx, userDAO, "alice")
	s, err := sessionDAO.Insert(ctx, "solo", "en", nil)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if _, err := sessionDAO.InsertParticipant(ctx, s.ID, alice.ID); err != nil {
		t.Fatalf("add participant: %v", err)
	}

	if err := sessionDAO.IncrementScore(ctx, s.ID, alice.ID); err != nil {
		t.Fatalf("increment: %v", err)
	}
	if err := sessionDAO.IncrementScore(ctx, s.ID, alice.ID); err != nil {
		t.Fatalf("increment 2: %v", err)
	}

	participants, err := sessionDAO.ListParticipants(ctx, s.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if participants[0].Score != 2 {
		t.Errorf("expected score 2, got %d", participants[0].Score)
	}
}

func TestGameSessionDAO_SetFinished(t *testing.T) {
	truncateTables(t)

	ctx := context.Background()
	dao := &GameSessionDAO{pool: testPool}

	s, err := dao.Insert(ctx, "solo", "en", nil)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := dao.SetFinished(ctx, s.ID); err != nil {
		t.Fatalf("set finished: %v", err)
	}

	updated, err := dao.FindByID(ctx, s.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if updated.Status != "finished" {
		t.Errorf("expected 'finished', got %q", updated.Status)
	}
	if updated.FinishedAt == nil {
		t.Error("expected non-nil finished_at")
	}
}
