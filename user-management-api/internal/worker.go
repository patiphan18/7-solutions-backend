package internal

import (
	"context"
	"log/slog"
	"time"

	"user-management-api/internal/port/driven"
)

func runUserCounter(ctx context.Context, repository driven.UserRepository, logger *slog.Logger, ticks <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ticks:
			if !ok || ctx.Err() != nil {
				return
			}

			countCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			count, err := repository.Count(countCtx)

			cancel()

			if err != nil {
				if ctx.Err() == nil {
					logger.Error("count users failed", "error", err)
				}

				continue
			}

			logger.Info("total users", "count", count)
		}
	}
}
