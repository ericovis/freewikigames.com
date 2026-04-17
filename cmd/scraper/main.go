package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ericovis/freewikigames.com/internal/db"
	"github.com/ericovis/freewikigames.com/internal/scraper"
	"github.com/ericovis/freewikigames.com/internal/worker"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	urls := os.Args[1:]
	if len(urls) == 0 {
		logger.Error("usage: scraper <url> [url ...]")
		os.Exit(1)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		logger.Error("DATABASE_URL is not set")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := db.New(ctx, dsn)
	if err != nil {
		logger.Error("connect to database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	sc := scraper.New(scraper.DefaultConfig())

	w := worker.NewScrapeWorker(urls, sc, database.Pages(), logger)
	if err := w.Run(ctx); err != nil {
		logger.Error("scrape worker", "err", err)
		os.Exit(1)
	}
}
