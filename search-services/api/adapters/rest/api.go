package rest

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/VictoriaMetrics/metrics"

	"github.com/kirillveshnyakov/XKCD_searcher/search-services/api/core"
)

func writeJSON(w http.ResponseWriter, message any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	if err := enc.Encode(message); err != nil {
		return fmt.Errorf("error encoding JSON: %w", err)
	}
	return nil
}

func NewMetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics.WritePrometheus(w, true)
	}
}

type PingResponse struct {
	Replies map[string]string `json:"replies"`
}

func NewPingHandler(log *slog.Logger, pingers map[string]core.Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Ping")

		replies := make(map[string]string)
		for s, p := range pingers {
			if err := p.Ping(r.Context()); err != nil {
				replies[s] = "unavailable"
			} else {
				replies[s] = "ok"
			}
		}

		resp := PingResponse{
			Replies: replies,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := writeJSON(w, resp); err != nil {
			log.Error("Error encoding JSON", "error", err)
		}
	}
}

type Authenticator interface {
	Login(user, password string) (string, error)
}

type LoginRequest struct {
	Login    string `json:"name"`
	Password string `json:"password"`
}

func NewLoginHandler(log *slog.Logger, auth Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Parsing data error", http.StatusBadRequest)
			return
		}

		token, err := auth.Login(req.Login, req.Password)
		if err != nil {
			log.Error("Login error", "name", req.Login, "error", err)
			http.Error(w, "Wrong login or password", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		if _, err = w.Write([]byte(token)); err != nil {
			log.Error("Error writing response", "name", req.Login, "error", err)
		}
	}
}

func NewUpdateHandler(log *slog.Logger, updater core.Updater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Update")

		err := updater.Update(r.Context())

		if err != nil {
			if errors.Is(err, core.ErrAlreadyExists) {
				log.Info("Update already exists")
				w.WriteHeader(http.StatusAccepted)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)
	}
}

type StatsResponse struct {
	WordsTotal    int `json:"words_total"`
	WordsUnique   int `json:"words_unique"`
	ComicsFetched int `json:"comics_fetched"`
	ComicsTotal   int `json:"comics_total"`
}

func NewUpdateStatsHandler(log *slog.Logger, updater core.Updater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("UpdateStats")

		stats, err := updater.Stats(r.Context())
		if err != nil {
			log.Error("Error get update stats", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		resp := StatsResponse{
			WordsTotal:    stats.WordsTotal,
			WordsUnique:   stats.WordsUnique,
			ComicsFetched: stats.ComicsFetched,
			ComicsTotal:   stats.ComicsTotal,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err = writeJSON(w, resp); err != nil {
			log.Error("Error encoding JSON", "error", err)
		}
	}
}

type StatusResponse struct {
	Status core.UpdateStatus `json:"status"`
}

func NewUpdateStatusHandler(log *slog.Logger, updater core.Updater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("UpdateStatus")

		status, err := updater.Status(r.Context())
		if err != nil {
			log.Error("Error get update status", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		resp := StatusResponse{
			Status: status,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err = writeJSON(w, resp); err != nil {
			log.Error("Error encoding JSON", "error", err)
		}
	}
}

func NewDropHandler(log *slog.Logger, updater core.Updater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Drop")

		err := updater.Drop(r.Context())

		if err != nil {
			log.Error("Error drop", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)
	}
}

type SearchResponse struct {
	Comics []core.Comics `json:"comics"`
	Total  int           `json:"total"`
}

func NewSearchHandler(log *slog.Logger, searcher core.Searcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Search")

		query := r.URL.Query()

		if !query.Has("phrase") {
			http.Error(w, "Invalid argument, no phrase", http.StatusBadRequest)
			return
		}
		phrase := query.Get("phrase")

		var limit int
		var err error

		if !query.Has("limit") {
			limit = 10
		} else {
			limitStr := query.Get("limit")
			limit, err = strconv.Atoi(limitStr)
			if err != nil || limit <= 0 {
				http.Error(w, "Invalid argument, wrong limit", http.StatusBadRequest)
				return
			}
		}

		comics, err := searcher.Search(r.Context(), phrase, limit)

		if err != nil {
			if errors.Is(err, core.ErrBadArguments) {
				http.Error(w, "Invalid argument", http.StatusBadRequest)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Debug("Search", "comics", comics)

		resp := SearchResponse{
			Comics: comics,
			Total:  len(comics),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err = writeJSON(w, resp); err != nil {
			log.Error("Error encoding JSON", "error", err)
		}
	}
}

func NewSearchIndexHandler(log *slog.Logger, searcher core.Searcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Search")

		query := r.URL.Query()

		if !query.Has("phrase") {
			http.Error(w, "Invalid argument, no phrase", http.StatusBadRequest)
			return
		}
		phrase := query.Get("phrase")

		var limit int
		var err error

		if !query.Has("limit") {
			limit = 10
		} else {
			limitStr := query.Get("limit")
			limit, err = strconv.Atoi(limitStr)
			if err != nil || limit <= 0 {
				http.Error(w, "Invalid argument, wrong limit", http.StatusBadRequest)
				return
			}
		}

		comics, err := searcher.SearchIndex(r.Context(), phrase, limit)

		if err != nil {
			if errors.Is(err, core.ErrBadArguments) {
				http.Error(w, "Invalid argument", http.StatusBadRequest)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Debug("Search", "comics", comics)

		resp := SearchResponse{
			Comics: comics,
			Total:  len(comics),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err = writeJSON(w, resp); err != nil {
			log.Error("Error encoding JSON", "error", err)
		}
	}
}
