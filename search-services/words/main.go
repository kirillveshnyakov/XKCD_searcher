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
	"strconv"
	"syscall"
	"time"

	wordspb "github.com/kirillveshnyakov/XKCD_searcher/search-services/proto/words"
	"github.com/kirillveshnyakov/XKCD_searcher/search-services/words/config"
	"github.com/kirillveshnyakov/XKCD_searcher/search-services/words/words"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	maxPhraseLen    = 16384
	maxShutdownTime = 5 * time.Second
)

type server struct {
	wordspb.UnimplementedWordsServer
}

func (s *server) Ping(_ context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s *server) Norm(_ context.Context, in *wordspb.WordsRequest) (*wordspb.WordsReply, error) {
	if len(in.GetPhrase()) > maxPhraseLen {
		return nil, status.Error(
			codes.ResourceExhausted,
			"phrase is large than "+strconv.Itoa(maxPhraseLen),
		)
	}
	return &wordspb.WordsReply{
		Words: words.Norm(in.GetPhrase()),
	}, nil
}

func main() {
	var cfgPath string
	flag.StringVar(&cfgPath, "config", "config.yaml", "path to config file")
	flag.Parse()
	cfg := config.MustLoad(cfgPath)

	log := mustMakeLogger(cfg.LogLevel)

	if err := run(cfg, log); err != nil {
		log.Error("failed to run", "error", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	log.Info("starting server")
	log.Debug("debug messages are enabled")

	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("failed to listen port %s: %v", cfg.Address, err)
	}

	serveErr := make(chan error, 1)
	defer close(serveErr)

	s := grpc.NewServer()
	wordspb.RegisterWordsServer(s, &server{})
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
