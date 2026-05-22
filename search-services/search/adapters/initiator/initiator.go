package initiator

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/kirillveshnyakov/XKCD_searcher/search-services/search/core"
)

type initiator struct {
	searcher core.Searcher
	period   time.Duration
	logger   *slog.Logger
}

func NewInitiator(searcher core.Searcher, period time.Duration, logger *slog.Logger) *initiator {
	return &initiator{
		searcher: searcher,
		period:   period,
		logger:   logger,
	}
}

func (i *initiator) Run(ctx context.Context) (func(), bool) {
	i.logger.Info("initiator start")
	err := i.searcher.BuildIndex(ctx)
	if err != nil {
		i.logger.Error("initiator - build index", "error", err)
		return nil, false
	}

	wg := &sync.WaitGroup{}
	wg.Go(func() {
		timer := time.NewTicker(i.period)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				i.logger.Info("initiator - context done")
				return
			case <-timer.C:
				err := i.searcher.BuildIndex(ctx)
				if err != nil {
					i.logger.Error("initiator - update index", "error", err)
				}
			}
		}
	})

	return wg.Wait, true
}
