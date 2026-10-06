package internal

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"user-management-api/internal/port/driven"
)

type counterRepository struct {
	driven.UserRepository
	called chan context.Context
}

func (r counterRepository) Count(ctx context.Context) (int64, error) {
	r.called <- ctx

	<-ctx.Done()

	return 0, ctx.Err()
}

func TestCounterCancelsInflightDatabaseCall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ticks := make(chan time.Time, 1)
	ticks <- time.Now()

	repository := counterRepository{called: make(chan context.Context, 1)}

	done := make(chan struct{})
	go func() {
		defer close(done)

		runUserCounter(ctx, repository, slog.New(slog.NewTextHandler(io.Discard, nil)), ticks)
	}()

	select {
	case callCtx := <-repository.called:
		if _, ok := callCtx.Deadline(); !ok {
			t.Fatal("database count has no deadline")
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not count users")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
}
