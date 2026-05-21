package middleware

import (
	"net/http"
)

func Concurrency(next http.HandlerFunc, limit int) http.HandlerFunc {
	limiter := make(chan struct{}, limit)
	return func(w http.ResponseWriter, r *http.Request) {
		select {
		case limiter <- struct{}{}:
			next.ServeHTTP(w, r)
			<-limiter
		default:
			http.Error(w, "The server is busy", http.StatusServiceUnavailable)
		}
	}
}
