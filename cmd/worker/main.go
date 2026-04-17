package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ericovis/freewikigames.com/internal/ai"
	"github.com/ericovis/freewikigames.com/internal/db"
	"github.com/ericovis/freewikigames.com/internal/questions"
	"github.com/ericovis/freewikigames.com/internal/worker"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		logger.Error("DATABASE_URL is not set")
		os.Exit(1)
	}

	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434"
	}
	ollamaModel := os.Getenv("OLLAMA_MODEL")
	if ollamaModel == "" {
		ollamaModel = "gemma4:e2b"
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := db.New(ctx, dsn)
	if err != nil {
		logger.Error("connect to database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	aiClient := ai.New(ollamaHost, ollamaModel)
	generator := questions.New(aiClient, logger)

	w := worker.NewQuestionWorker(database.Pages(), database.Questions(), generator, logger)
	if err := w.Run(ctx); err != nil {
		logger.Error("question worker", "err", err)
		os.Exit(1)
	}
}
