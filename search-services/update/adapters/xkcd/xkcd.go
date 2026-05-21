package xkcd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"yadro.com/course/closers"
	"yadro.com/course/update/core"
)

const (
	lastPath   = "/info.0.json"
	maxRetries = 5
	backoff    = 1 * time.Second
)

type Client struct {
	log    *slog.Logger
	client http.Client
	url    string
}

func NewClient(url string, timeout time.Duration, log *slog.Logger) (*Client, error) {
	if url == "" {
		return nil, fmt.Errorf("empty base url specified")
	}
	return &Client{
		client: http.Client{Timeout: timeout},
		log:    log,
		url:    url,
	}, nil
}

func (c *Client) doReq(ctx context.Context, method string, url string, respStatus int) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}

	var resp *http.Response
	attempt := 0
	for {
		resp, err = c.client.Do(req)
		if err == nil {
			break
		}
		attempt++
		if attempt >= maxRetries {
			return nil, fmt.Errorf("failed to request comics: %v", err)
		}
		c.log.Error("failed to connect, sleeping and retrying", "error", err)
		time.Sleep(backoff)
	}

	if resp.StatusCode != respStatus {
		if resp.StatusCode == http.StatusNotFound {
			return nil, core.ErrNotFound
		}
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}

	return resp, nil
}

func (c *Client) Get(ctx context.Context, id int) (core.XKCDInfo, error) {
	lastId, err := c.LastID(ctx)

	if err != nil {
		return core.XKCDInfo{}, err
	}

	if id > lastId {
		return core.XKCDInfo{}, core.ErrNotFound
	}

	resp, err := c.doReq(ctx, http.MethodGet, fmt.Sprintf("%s/%d/%s", c.url, id, lastPath), http.StatusOK)
	if err != nil {
		c.log.Error("request error", "error", err)
		return core.XKCDInfo{}, err
	}
	defer closers.CloseOrLog(resp.Body, c.log)

	info := struct {
		ID         int    `json:"num"`
		URL        string `json:"img"`
		Title      string `json:"title"`
		SafeTitle  string `json:"safe_title"`
		Alt        string `json:"alt"`
		Transcript string `json:"transcript"`
	}{}
	if err = json.NewDecoder(resp.Body).Decode(&info); err != nil {
		err = fmt.Errorf("json decod error: %w", err)
		c.log.Error(err.Error())
		return core.XKCDInfo{}, err
	}

	return core.XKCDInfo{
		ID:  id,
		URL: info.URL,
		Description: strings.Join(
			[]string{info.Title, info.SafeTitle, info.Alt, info.Transcript},
			" ",
		),
	}, nil
}

func (c *Client) LastID(ctx context.Context) (int, error) {
	resp, err := c.doReq(ctx, http.MethodGet, fmt.Sprintf("%s/info.0.json", c.url), http.StatusOK)
	if err != nil {
		err = fmt.Errorf("request error: %w", err)
		c.log.Error(err.Error())
		return -1, err
	}
	defer closers.CloseOrLog(resp.Body, c.log)

	var info struct {
		Num int `json:"num"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&info); err != nil {
		err = fmt.Errorf("json decod error: %w", err)
		c.log.Error(err.Error())
		return -1, err
	}

	return info.Num, nil
}
