package search

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"yadro.com/course/api/core"
	searchpb "yadro.com/course/proto/search"
)

type Client struct {
	log    *slog.Logger
	client searchpb.SearchClient
	conn   *grpc.ClientConn
}

func NewClient(address string, log *slog.Logger) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("error grpc connect to update service, address: %s: %w", address, err)
	}
	return &Client{
		client: searchpb.NewSearchClient(conn),
		log:    log,
		conn:   conn,
	}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	c.log.Debug("api adapter search: Ping")

	_, err := c.client.Ping(ctx, &emptypb.Empty{})
	return err
}

func (c *Client) Search(ctx context.Context, phrase string, limit int) ([]core.Comics, error) {
	c.log.Debug("api adapter search: Search")

	protoComics, err := c.client.Search(ctx, &searchpb.SearchRequest{
		Phrase: phrase,
		Limit:  int64(limit),
	})

	if err != nil {
		if status.Code(err) == codes.InvalidArgument {
			return nil, core.ErrBadArguments
		}
		return nil, err
	}

	comics := make([]core.Comics, len(protoComics.Comics))
	for i, comic := range protoComics.Comics {
		comics[i] = core.Comics{
			ID:  int(comic.Id),
			URL: comic.Url,
		}
	}
	return comics, nil
}

func (c *Client) SearchIndex(ctx context.Context, phrase string, limit int) ([]core.Comics, error) {
	c.log.Debug("api adapter search index: SearchIndex")

	protoComics, err := c.client.SearchIndex(ctx, &searchpb.SearchRequest{
		Phrase: phrase,
		Limit:  int64(limit),
	})

	if err != nil {
		if status.Code(err) == codes.InvalidArgument {
			return nil, core.ErrBadArguments
		}
		return nil, err
	}

	comics := make([]core.Comics, len(protoComics.Comics))
	for i, comic := range protoComics.Comics {
		comics[i] = core.Comics{
			ID:  int(comic.Id),
			URL: comic.Url,
		}
	}
	return comics, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
