package grpc

import (
	"context"

	searchpb "github.com/kirillveshnyakov/XKCD_searcher/search-services/proto/search"
	"github.com/kirillveshnyakov/XKCD_searcher/search-services/search/core"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func NewServer(service core.Searcher) *server {
	return &server{service: service}
}

type server struct {
	searchpb.UnimplementedSearchServer
	service core.Searcher
}

func (s *server) Ping(_ context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s *server) Search(ctx context.Context, request *searchpb.SearchRequest) (*searchpb.SearchReply, error) {
	phrase := request.GetPhrase()
	limit := int(request.GetLimit())

	if phrase == "" {
		return nil, status.Error(codes.InvalidArgument, "Empty phrase")
	}
	if limit <= 0 {
		return nil, status.Error(codes.InvalidArgument, "Wrong limit")
	}

	comics, err := s.service.Search(ctx, phrase, limit)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &searchpb.SearchReply{
		Comics: fromCoreToProtoComicsArray(comics),
	}, nil
}

func (s *server) SearchIndex(ctx context.Context, request *searchpb.SearchRequest) (*searchpb.SearchReply, error) {
	phrase := request.GetPhrase()
	limit := int(request.GetLimit())

	if phrase == "" {
		return nil, status.Error(codes.InvalidArgument, "Empty phrase")
	}
	if limit <= 0 {
		return nil, status.Error(codes.InvalidArgument, "Wrong limit")
	}

	comics, err := s.service.SearchIndex(ctx, phrase, limit)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &searchpb.SearchReply{
		Comics: fromCoreToProtoComicsArray(comics),
	}, nil
}

func fromCoreToProtoComic(comic core.Comics) *searchpb.Comic {
	return &searchpb.Comic{
		Id:  int64(comic.ID),
		Url: comic.URL,
	}
}

func fromCoreToProtoComicsArray(comics []core.Comics) []*searchpb.Comic {
	protoComics := make([]*searchpb.Comic, len(comics))
	for i, comic := range comics {
		protoComics[i] = fromCoreToProtoComic(comic)
	}
	return protoComics
}
