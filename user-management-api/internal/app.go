package internal

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"google.golang.org/grpc"
	"user-management-api/internal/bootstrap"
	"user-management-api/internal/config"
)

type app struct {
	config config.Config
}

func New() (*app, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}

	return &app{config: cfg}, nil
}

func (a *app) StartServer(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	cfg := a.config
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	dependencies, err := bootstrap.New(startup, cfg, slog.Default())

	cancel()

	if err != nil {
		return err
	}

	defer func() {
		disconnect, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := dependencies.Close(disconnect); err != nil {
			slog.Error("failed to close dependencies", "error", err)
		}
	}()

	repository := dependencies.Repository
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           dependencies.Handler,
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    16 * 1024,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	listener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return err
	}
	defer listener.Close()

	grpcServer := dependencies.GRPCServer
	defer grpcServer.Stop()

	workerCtx, cancelWorker := context.WithCancel(ctx)
	defer cancelWorker()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		runUserCounter(workerCtx, repository, slog.Default(), ticker.C)
	}()

	done := make(chan error, 2)

	go func() {
		slog.Info("grpc server listening", "address", listener.Addr().String())

		done <- grpcServer.Serve(listener)
	}()
	go func() {
		slog.Info("server listening", "address", server.Addr)

		done <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case err = <-done:
	}

	cancelWorker()

	shutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	grpcStopped := make(chan struct{})

	go func() { grpcServer.GracefulStop(); close(grpcStopped) }()

	shutdownErr := server.Shutdown(shutdown)
	select {
	case <-grpcStopped:
	case <-shutdown.Done():
		grpcServer.Stop()
	}

	if shutdownErr != nil {
		_ = server.Close()
	}

	wg.Wait()

	if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, grpc.ErrServerStopped) {
		return err
	}

	return shutdownErr
}
