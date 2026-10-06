package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"user-management-api/internal"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := internal.New()
	if err != nil {
		slog.Error("failed to initialize service", "error", err)
		os.Exit(1)
	}

	if err := app.StartServer(ctx); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
