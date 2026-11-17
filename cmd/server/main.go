package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	taskv1 "github.com/Chibas/http_service/gen/task/v1"
	"github.com/Chibas/http_service/internal/config"
	"github.com/Chibas/http_service/internal/database"
	"github.com/Chibas/http_service/internal/grpcserver"
	"github.com/Chibas/http_service/internal/middleware"
	"github.com/Chibas/http_service/internal/task"
	_ "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	config, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed loading server config: %v", err)
	}
	db, err := database.ConnectDB(config)
	if err != nil {
		log.Fatalf("DB startup failed: %v", err)
	}
	defer db.Close()

	router :=
		middleware.RequestIDMiddleware(
			middleware.LoggingMiddleware(
				middleware.MetricsMiddleware(
					middleware.RecoveryMiddleware(
						task.AppRouter(db),
					),
				),
			),
		)

	server := &http.Server{
		Addr:    ":" + config.HTTPPort,
		Handler: router,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 2)

	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		slog.Error("Failed to open gRPC listener", "error", err)
		return
	}
	grpcServer := grpc.NewServer()
	taskServer := grpcserver.NewServer(db)
	taskv1.RegisterTaskServiceServer(grpcServer, taskServer)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			errCh <- fmt.Errorf("gRPC server: %w", err)
		}
	}()

	go func() {
		fmt.Printf("Server running on port %s\n", config.HTTPPort)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		slog.Log(ctx, slog.LevelWarn, "Shutdown signal received")
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server failed", "error", err)
		}
	}

	sdwnCtx, sdwnCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer sdwnCancel()

	httpStopped := make(chan error, 1)
	go func() {
		httpStopped <- server.Shutdown(sdwnCtx)
	}()

	grpcStopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(grpcStopped)
	}()

	select {
	case <-grpcStopped:
		slog.Info("gRPC server stopped gracefully")
	case <-sdwnCtx.Done():
		select {
		case <-grpcStopped:
			// already finished gracefully
		default:
			grpcServer.Stop()
			slog.Warn("gRPC shutdown deadline exceeded; forced stop")
		}
	}

	if err := <-httpStopped; err != nil {
		slog.Error("HTTP shutdown failed", "error", err)

		// Close connections if graceful shutdown timed out.
		if err := server.Close(); err != nil {
			slog.Error("HTTP forced close failed", "error", err)
		}
	} else {
		slog.Info("HTTP server stopped gracefully")
	}
}
