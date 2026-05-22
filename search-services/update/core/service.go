package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
)

type Service struct {
	log         *slog.Logger
	db          DB
	xkcd        XKCD
	words       Words
	concurrency int
	updating    atomic.Bool
}

func NewService(
	log *slog.Logger, db DB, xkcd XKCD, words Words, concurrency int,
) (*Service, error) {
	if concurrency < 1 {
		return nil, fmt.Errorf("wrong concurrency specified: %d", concurrency)
	}
	return &Service{
		log:         log,
		db:          db,
		xkcd:        xkcd,
		words:       words,
		concurrency: concurrency,
	}, nil
}

func collectTask(ctx context.Context, ids []int, total int) <-chan int {
	result := make(chan int)

	go func() {
		defer close(result)

		slices.Sort(ids)
		j := 0
		for i := 1; i <= total; i++ {
			if j < len(ids) && ids[j] == i {
				j++
				continue
			}
			select {
			case <-ctx.Done():
				return
			case result <- i:
			}
		}
	}()

	return result
}

func (s *Service) worker(ctx context.Context, id int) {
	info, err := s.xkcd.Get(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			comics := Comics{
				ID:    id,
				URL:   "",
				Words: []string{},
			}
			err = s.db.Add(ctx, comics)
			if err != nil {
				s.log.Error("grps update error: Add in DB", "error", err)
			}
			return
		}
		s.log.Error("grps update error: Get comics info", "id", id, "error", err)
		return
	}

	if ctx.Err() != nil {
		return
	}

	words, err := s.words.Norm(ctx, info.Description)
	if err != nil {
		s.log.Error("grps update error: Norm key words", "error", err)
		return
	}

	comics := Comics{
		ID:    id,
		URL:   info.URL,
		Words: words,
	}

	if ctx.Err() != nil {
		return
	}

	err = s.db.Add(ctx, comics)
	if err != nil {
		s.log.Error("grps update error: Add in DB", "error", err)
	}
}

func (s *Service) Update(ctx context.Context) error {
	s.log.Debug("grps update: Update")

	if !s.updating.CompareAndSwap(false, true) {
		return ErrAlreadyExists
	}
	defer s.updating.Store(false)

	IDs, err := s.db.IDs(ctx)
	if err != nil {
		s.log.Error("grps update error: IDs", "error", err)
		return err
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	total, err := s.xkcd.LastID(ctx)
	if err != nil {
		s.log.Error("grps update error: LastID", "error", err)
		return err
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	ids := collectTask(ctx, IDs, total)
	sema := make(chan struct{}, s.concurrency)

	wg := sync.WaitGroup{}

	for id := range ids {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		sema <- struct{}{}
		wg.Go(func() {
			s.worker(ctx, id)
			<-sema
		})
	}

	wg.Wait()

	return nil
}

func (s *Service) Stats(ctx context.Context) (ServiceStats, error) {
	s.log.Debug("grps update: Stats")

	dbStats, err := s.db.Stats(ctx)
	if err != nil {
		s.log.Error("failed to get stats", "error", err)
		return ServiceStats{}, err
	}

	if ctx.Err() != nil {
		return ServiceStats{}, ctx.Err()
	}

	comicsTotal, err := s.xkcd.LastID(ctx)
	if err != nil {
		s.log.Error("failed to get LastID", "error", err)
		return ServiceStats{}, err
	}

	return ServiceStats{
		DBStats:     dbStats,
		ComicsTotal: comicsTotal,
	}, nil
}

func (s *Service) Status(ctx context.Context) ServiceStatus {
	s.log.Debug("grps update: Status")

	if s.updating.Load() {
		return StatusRunning
	}
	return StatusIdle
}

func (s *Service) Drop(ctx context.Context) error {
	s.log.Debug("grps update: Drop")

	if !s.updating.CompareAndSwap(false, true) {
		return ErrAlreadyExists
	}
	defer s.updating.Store(false)

	err := s.db.Drop(ctx)
	if err != nil {
		s.log.Error(err.Error())
	}
	return err
}
