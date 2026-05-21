package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

type responseWriter struct {
	http.ResponseWriter
	code int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	if rw.code != 0 {
		return
	}
	rw.code = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func WithMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w}

		next.ServeHTTP(rw, r)

		if rw.code == 0 {
			rw.code = http.StatusOK
		}

		name := fmt.Sprintf(
			`http_request_duration_seconds{status="%d",url=%q}`,
			rw.code,
			r.URL.Path,
		)
		metrics.GetOrCreateHistogram(name).UpdateDuration(start)
	})
}
