package update

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kirillveshnyakov/XKCD_searcher/search-services/api/core"
	updatepb "github.com/kirillveshnyakov/XKCD_searcher/search-services/proto/update"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Client struct {
	log    *slog.Logger
	client updatepb.UpdateClient
	conn   *grpc.ClientConn
}

func NewClient(address string, log *slog.Logger) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("error grpc connect to update service, address: %s: %w", address, err)
	}
	return &Client{
		client: updatepb.NewUpdateClient(conn),
		log:    log,
		conn:   conn,
	}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	c.log.Debug("api adapter update: Ping")

	_, err := c.client.Ping(ctx, &emptypb.Empty{})
	return err
}

func (c *Client) Status(ctx context.Context) (core.UpdateStatus, error) {
	c.log.Debug("api adapter update: Status")

	status, err := c.client.Status(ctx, &emptypb.Empty{})
	if err != nil {
		return core.StatusUpdateUnknown, fmt.Errorf("error getting update status: %w", err)
	}
	switch status.Status {
	case updatepb.Status_STATUS_IDLE:
		return core.StatusUpdateIdle, nil
	case updatepb.Status_STATUS_RUNNING:
		return core.StatusUpdateRunning, nil
	default:
		return "", fmt.Errorf("error getting update status: unknown status - %s", status.Status)
	}
}

func (c *Client) Stats(ctx context.Context) (core.UpdateStats, error) {
	c.log.Debug("api adapter update: Stats")

	rep, err := c.client.Stats(ctx, &emptypb.Empty{})
	if err != nil {
		return core.UpdateStats{}, err
	}
	return core.UpdateStats{
		WordsTotal:    int(rep.WordsTotal),
		WordsUnique:   int(rep.WordsUnique),
		ComicsFetched: int(rep.ComicsFetched),
		ComicsTotal:   int(rep.ComicsTotal),
	}, nil
}

func (c *Client) Update(ctx context.Context) error {
	c.log.Debug("api adapter update: Update")

	_, err := c.client.Update(ctx, &emptypb.Empty{})

	if status.Code(err) == codes.AlreadyExists {
		return core.ErrAlreadyExists
	}

	return err
}

func (c *Client) Drop(ctx context.Context) error {
	c.log.Debug("api adapter update: Drop")

	_, err := c.client.Drop(ctx, &emptypb.Empty{})
	return err
}

func (c *Client) Close() error {
	return c.conn.Close()
}
