package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kirillveshnyakov/XKCD_searcher/search-services/closers"
	updatepb "github.com/kirillveshnyakov/XKCD_searcher/search-services/proto/update"
	"github.com/kirillveshnyakov/XKCD_searcher/search-services/update/adapters/db"
	updategrpc "github.com/kirillveshnyakov/XKCD_searcher/search-services/update/adapters/grpc"
	"github.com/kirillveshnyakov/XKCD_searcher/search-services/update/adapters/words"
	"github.com/kirillveshnyakov/XKCD_searcher/search-services/update/adapters/xkcd"
	"github.com/kirillveshnyakov/XKCD_searcher/search-services/update/config"
	"github.com/kirillveshnyakov/XKCD_searcher/search-services/update/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	maxShutdownTime = 5 * time.Second
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "server configuration file")
	flag.Parse()
	cfg := config.MustLoad(configPath)

	log := mustMakeLogger(cfg.LogLevel)

	if err := run(cfg, log); err != nil {
		log.Error("failed to run", "error", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	log.Info("starting server")
	log.Debug("debug messages are enabled")

	// database adapter
	storage, err := db.New(log, cfg.DBAddress)
	if err != nil {
		return fmt.Errorf("failed to connect to db: %v", err)
	}
	if err = storage.Migrate(); err != nil {
		return fmt.Errorf("failed to migrate db: %v", err)
	}

	// xkcd adapter
	xkcd, err := xkcd.NewClient(cfg.XKCD.URL, cfg.XKCD.Timeout, log)
	if err != nil {
		return fmt.Errorf("failed create XKCD client: %v", err)
	}

	// words adapter
	words, err := words.NewClient(cfg.WordsAddress, log)
	if err != nil {
		return fmt.Errorf("failed create Words client: %v", err)
	}
	defer closers.CloseOrLog(words, log)

	// service
	updater, err := core.NewService(log, storage, xkcd, words, cfg.XKCD.Concurrency)
	if err != nil {
		return fmt.Errorf("failed create Update service: %v", err)
	}

	// grpc server
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("failed to listen port %s: %v", cfg.Address, err)
	}

	serveErr := make(chan error, 1)
	defer close(serveErr)

	s := grpc.NewServer()
	updatepb.RegisterUpdateServer(s, updategrpc.NewServer(updater))
	reflection.Register(s)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt, syscall.SIGINT, syscall.SIGTERM,
	)
	defer stop()

	go func() {
		log.Info("grpc server started", "address", listener.Addr().String())

		if err2 := s.Serve(listener); err2 != nil && !errors.Is(err2, grpc.ErrServerStopped) {
			serveErr <- fmt.Errorf("grpc server error: %w", err2)
			return
		}
		serveErr <- nil
	}()

	select {
	case err = <-serveErr:
		return err
	case <-ctx.Done():
	}

	log.Info("starting graceful stop grpc server")
	stopped := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(stopped)
	}()

	timer := time.NewTimer(maxShutdownTime)
	defer timer.Stop()

	select {
	case <-stopped:
		log.Info("grpc server stopped gracefully")
	case <-timer.C:
		log.Warn("grpc server graceful shutdown timeout exceeded, forcing stop", "timeout", maxShutdownTime)
		s.Stop()
		<-stopped
	}

	return <-serveErr
}

func mustMakeLogger(logLevel string) *slog.Logger {
	var level slog.Level
	switch logLevel {
	case "DEBUG":
		level = slog.LevelDebug
	case "INFO":
		level = slog.LevelInfo
	case "ERROR":
		level = slog.LevelError
	default:
		panic("unknown log level: " + logLevel)
	}
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level, AddSource: level == slog.LevelDebug})
	return slog.New(handler)
}
