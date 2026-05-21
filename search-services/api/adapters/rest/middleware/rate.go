package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type RateLimiter struct {
	limit    int
	interval time.Duration
	req      []time.Time
	lock     sync.Mutex
}

func NewRateLimiter(limit int, interval time.Duration) *RateLimiter {
	if limit <= 0 {
		panic("rate limiter limit must be >= 0")
	}

	return &RateLimiter{
		limit:    limit,
		interval: interval,
		req:      make([]time.Time, 0, limit),
		lock:     sync.Mutex{},
	}
}

func (r *RateLimiter) Allow() bool {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.cleanReq(time.Now())

	if len(r.req) < r.limit {
		r.req = append(r.req, time.Now())
		return true
	}

	return false
}

func (r *RateLimiter) Wait(ctx context.Context) bool {
	for {
		wait := r.timeToWait()
		if wait == 0 {
			return true
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false
		case <-timer.C:
		}
	}
}

func (r *RateLimiter) timeToWait() time.Duration {
	now := time.Now()

	r.lock.Lock()
	defer r.lock.Unlock()

	r.cleanReq(now)

	if len(r.req) < r.limit {
		r.req = append(r.req, now)
		return 0
	}

	return r.req[0].Add(r.interval).Sub(now)
}

func (r *RateLimiter) cleanReq(now time.Time) {
	timeTrigger := now.Add(-r.interval)
	for len(r.req) > 0 && r.req[0].Before(timeTrigger) {
		r.req = r.req[1:]
	}
}

func Rate(next http.HandlerFunc, rps int, log *slog.Logger) http.HandlerFunc {
	limiter := NewRateLimiter(rps, time.Second)
	return func(w http.ResponseWriter, r *http.Request) {
		if ok := limiter.Wait(r.Context()); !ok {
			log.Warn("request cancelled while waiting for rate limiter", "path", r.URL.Path)
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}

		next.ServeHTTP(w, r)
	}
}
