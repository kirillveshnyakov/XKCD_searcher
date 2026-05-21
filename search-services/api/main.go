package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	AAA "yadro.com/course/api/adapters/aaa"
	"yadro.com/course/api/adapters/rest"
	"yadro.com/course/api/adapters/rest/middleware"
	"yadro.com/course/api/adapters/search"
	"yadro.com/course/api/adapters/update"
	"yadro.com/course/api/adapters/words"
	"yadro.com/course/api/config"
	"yadro.com/course/api/core"
	"yadro.com/course/closers"
)

const (
	httpShutdownTime = 30 * time.Second
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "server configuration file")
	flag.Parse()

	cfg := config.MustLoad(configPath)

	log := mustMakeLogger(cfg.LogLevel)

	if err := run(cfg, log); err != nil {
		slog.Error("run failed", "error", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	log.Info("starting server")
	log.Debug("debug messages are enabled")

	wordsClient, err := words.NewClient(cfg.WordsAddress, log)
	if err != nil {
		return fmt.Errorf("cannot init words adapter: %v", err)
	}
	defer closers.CloseOrLog(wordsClient, log)

	updateClient, err := update.NewClient(cfg.UpdateAddress, log)
	if err != nil {
		return fmt.Errorf("cannot init words adapter: %v", err)
	}
	defer closers.CloseOrLog(updateClient, log)

	searchClient, err := search.NewClient(cfg.SearchAddress, log)
	if err != nil {
		return fmt.Errorf("cannot init words adapter: %v", err)
	}
	defer closers.CloseOrLog(searchClient, log)

	log.Info("Token TTL: ", "", cfg.TokenTTL)
	aaa, err := AAA.New(cfg.TokenTTL, log)
	if err != nil {
		return fmt.Errorf("cannot init AAA adapter: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /api/ping", rest.NewPingHandler(log, map[string]core.Pinger{
		"words":  wordsClient,
		"update": updateClient,
		"search": searchClient,
	}))

	mux.Handle("GET /metrics", rest.NewMetricsHandler())
	mux.Handle("POST /api/login", rest.NewLoginHandler(log, aaa))

	mux.Handle("GET /api/db/stats", rest.NewUpdateStatsHandler(log, updateClient))
	mux.Handle("GET /api/db/status", rest.NewUpdateStatusHandler(log, updateClient))
	mux.Handle("POST /api/db/update",
		middleware.Auth(
			rest.NewUpdateHandler(log, updateClient),
			aaa,
		),
	)
	mux.Handle("DELETE /api/db",
		middleware.Auth(
			rest.NewDropHandler(log, updateClient),
			aaa,
		),
	)

	mux.Handle("GET /api/search",
		middleware.Concurrency(
			rest.NewSearchHandler(log, searchClient),
			cfg.SearchConcurrency,
		),
	)
	mux.Handle("GET /api/isearch",
		middleware.Rate(
			rest.NewSearchIndexHandler(log, searchClient),
			cfg.SearchRate, log,
		),
	)

	server := http.Server{
		Addr:        cfg.HTTPConfig.Address,
		ReadTimeout: cfg.HTTPConfig.Timeout,
		Handler:     middleware.WithMetrics(mux),
	}

	serveErr := make(chan error, 1)
	defer close(serveErr)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt, syscall.SIGTERM,
	)
	defer stop()

	go func() {
		log.Info("http gateway started", "address", cfg.HTTPConfig.Address)
		if err2 := server.ListenAndServe(); err2 != nil && !errors.Is(err2, http.ErrServerClosed) {
			serveErr <- fmt.Errorf("http gateway listen error: %w", err2)
			return
		}
		serveErr <- nil
	}()

	select {
	case err = <-serveErr:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), httpShutdownTime)
	defer cancel()

	log.Info("shutting down http server")
	if err = server.Shutdown(shutdownCtx); err != nil {
		log.Warn("http gateway shutdown error", "error", err)
		if closeErr := server.Close(); closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
			log.Warn("http gateway forced close error", "error", closeErr)
		}
		return fmt.Errorf("http gateway shutdown error: %w", err)
	}
	log.Info("http gateway gracefully shutdown")

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
