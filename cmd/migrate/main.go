package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ericovis/freewikigames.com/internal/db"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "error: DATABASE_URL environment variable is not set")
		os.Exit(1)
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: connect to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: ping database: %v\n", err)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "up":
		if err := db.Migrate(ctx, pool); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("migrations applied")

	case "status":
		statuses, err := db.MigrateStatus(ctx, pool)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		for _, s := range statuses {
			state := "pending"
			if s.Applied {
				state = "applied"
			}
			fmt.Printf("%-10s  %s\n", state, s.Version)
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %q\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: migrate <command>")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  up       Apply all pending migrations")
	fmt.Fprintln(os.Stderr, "  status   Show applied/pending status for each migration")
}
