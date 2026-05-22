package words

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kirillveshnyakov/XKCD_searcher/search-services/api/core"
	wordspb "github.com/kirillveshnyakov/XKCD_searcher/search-services/proto/words"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Client struct {
	log    *slog.Logger
	client wordspb.WordsClient
	conn   *grpc.ClientConn
}

func NewClient(address string, log *slog.Logger) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("error grpc connect to words service, address: %s: %w", address, err)
	}
	return &Client{
		client: wordspb.NewWordsClient(conn),
		log:    log,
		conn:   conn,
	}, nil
}

func (c *Client) Norm(ctx context.Context, phrase string) ([]string, error) {
	resp, err := c.client.Norm(ctx, &wordspb.WordsRequest{Phrase: phrase})
	if err != nil {
		if status.Code(err) == codes.ResourceExhausted {
			return nil, core.ErrBadArguments
		}
		return nil, err
	}
	return resp.GetWords(), nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.client.Ping(ctx, &emptypb.Empty{})
	return err
}

func (c *Client) Close() error {
	return c.conn.Close()
}
