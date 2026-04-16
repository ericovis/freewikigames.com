package db

import (
	"context"
	"testing"
)

func TestMigrate_AppliesAllPending(t *testing.T) {
	truncateTables(t)

	ctx := context.Background()

	if err := Migrate(ctx, testPool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	statuses, err := MigrateStatus(ctx, testPool)
	if err != nil {
		t.Fatalf("MigrateStatus: %v", err)
	}

	if len(statuses) == 0 {
		t.Fatal("expected at least one migration, got none")
	}

	for _, s := range statuses {
		if !s.Applied {
			t.Errorf("migration %q not applied after Migrate()", s.Version)
		}
	}
}

func TestMigrate_Idempotent(t *testing.T) {
	truncateTables(t)

	ctx := context.Background()

	if err := Migrate(ctx, testPool); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}

	if err := Migrate(ctx, testPool); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}

	statuses, err := MigrateStatus(ctx, testPool)
	if err != nil {
		t.Fatalf("MigrateStatus: %v", err)
	}

	for _, s := range statuses {
		if !s.Applied {
			t.Errorf("migration %q not applied after idempotent run", s.Version)
		}
	}
}

func TestMigrateStatus_PendingAndApplied(t *testing.T) {
	truncateTables(t)

	ctx := context.Background()

	// Before running: all should be pending.
	statuses, err := MigrateStatus(ctx, testPool)
	if err != nil {
		t.Fatalf("MigrateStatus before migrate: %v", err)
	}
	for _, s := range statuses {
		if s.Applied {
			t.Errorf("migration %q should be pending before Migrate()", s.Version)
		}
	}

	if err := Migrate(ctx, testPool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// After running: all should be applied.
	statuses, err = MigrateStatus(ctx, testPool)
	if err != nil {
		t.Fatalf("MigrateStatus after migrate: %v", err)
	}
	for _, s := range statuses {
		if !s.Applied {
			t.Errorf("migration %q should be applied after Migrate()", s.Version)
		}
	}
}
