package grpc

import (
	"context"
	"errors"
	"fmt"

	updatepb "github.com/kirillveshnyakov/XKCD_searcher/search-services/proto/update"
	"github.com/kirillveshnyakov/XKCD_searcher/search-services/update/core"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func NewServer(service core.Updater) *Server {
	return &Server{service: service}
}

type Server struct {
	updatepb.UnimplementedUpdateServer
	service core.Updater
}

func (s *Server) Ping(_ context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s *Server) Status(ctx context.Context, _ *emptypb.Empty) (*updatepb.StatusReply, error) {
	switch s.service.Status(ctx) {
	case core.StatusIdle:
		return &updatepb.StatusReply{
			Status: updatepb.Status_STATUS_IDLE,
		}, nil
	case core.StatusRunning:
		return &updatepb.StatusReply{
			Status: updatepb.Status_STATUS_RUNNING,
		}, nil
	}
	return nil, status.Error(codes.Internal, "unknown status from service")
}

func (s *Server) Update(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	if err := s.service.Update(ctx); err != nil {
		if errors.Is(err, core.ErrAlreadyExists) {
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) Stats(ctx context.Context, _ *emptypb.Empty) (*updatepb.StatsReply, error) {
	stats, err := s.service.Stats(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting stats: %w", err)
	}
	return &updatepb.StatsReply{
		WordsTotal:    int64(stats.WordsTotal),
		WordsUnique:   int64(stats.WordsUnique),
		ComicsTotal:   int64(stats.ComicsTotal),
		ComicsFetched: int64(stats.ComicsFetched),
	}, nil
}

func (s *Server) Drop(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	if err := s.service.Drop(ctx); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil

}
