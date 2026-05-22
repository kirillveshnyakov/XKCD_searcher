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
	searchpb "github.com/kirillveshnyakov/XKCD_searcher/search-services/proto/search"
	"github.com/kirillveshnyakov/XKCD_searcher/search-services/search/adapters/db"
	searchgrpc "github.com/kirillveshnyakov/XKCD_searcher/search-services/search/adapters/grpc"
	index "github.com/kirillveshnyakov/XKCD_searcher/search-services/search/adapters/initiator"
	"github.com/kirillveshnyakov/XKCD_searcher/search-services/search/adapters/words"
	"github.com/kirillveshnyakov/XKCD_searcher/search-services/search/config"
	"github.com/kirillveshnyakov/XKCD_searcher/search-services/search/core"
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
	defer closers.CloseOrLog(storage, log)

	// words adapter
	words, err := words.NewClient(cfg.WordsAddress, log)
	if err != nil {
		return fmt.Errorf("failed create Words client: %v", err)
	}
	defer closers.CloseOrLog(words, log)

	// service
	searcher, err := core.NewService(log, storage, words)
	if err != nil {
		return fmt.Errorf("failed create Search service: %v", err)
	}

	// grpc server
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("failed to listen port %s: %v", cfg.Address, err)
	}

	serveErr := make(chan error, 1)
	defer close(serveErr)

	s := grpc.NewServer()
	searchpb.RegisterSearchServer(s, searchgrpc.NewServer(searcher))
	reflection.Register(s)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt, syscall.SIGINT, syscall.SIGTERM,
	)
	defer stop()

	initiator := index.NewInitiator(searcher, cfg.IndexTTL, log)
	initiatorWait, ok := initiator.Run(ctx)

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

	if ok {
		initiatorWait()
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
