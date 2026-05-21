package core

import (
	"cmp"
	"context"
	"log/slog"
	"maps"
	"slices"
)

type Service struct {
	log   *slog.Logger
	db    DB
	words Words
	index *Index
}

func NewService(
	log *slog.Logger, db DB, words Words,
) (*Service, error) {
	return &Service{
		log:   log,
		db:    db,
		words: words,
		index: NewIndex(),
	}, nil
}

func (s *Service) Search(ctx context.Context, phrase string, limit int) ([]Comics, error) {
	words, err := s.words.Norm(ctx, phrase)
	if err != nil {
		s.log.Error("search service error: Norm phrase", "phrase", phrase, "error", err)
		return nil, err
	}

	comics, err := s.db.Find(ctx, words, limit)
	if err != nil {
		s.log.Error("search service error: find in DB", "error", err)
		return nil, err
	}

	return comics, nil
}

func (s *Service) SearchIndex(ctx context.Context, phrase string, limit int) ([]Comics, error) {
	words, err := s.words.Norm(ctx, phrase)
	if err != nil {
		s.log.Error("search service error: Norm phrase", "phrase", phrase, "error", err)
		return nil, err
	}

	numberOfMatches := make(map[int]int)
	for _, word := range words {
		IDs := s.index.GetIDs(word)
		if len(IDs) == 0 {
			continue
		}
		for _, ID := range IDs {
			numberOfMatches[ID]++
		}
	}

	sortedIDs := slices.SortedFunc(maps.Keys(numberOfMatches), func(a, b int) int {
		return cmp.Compare(numberOfMatches[b], numberOfMatches[a])
	})

	limit = min(limit, len(sortedIDs))
	sortedIDs = sortedIDs[:limit]

	comics := make([]Comics, 0, limit)
	for _, id := range sortedIDs {
		comic, err := s.db.GetById(ctx, id)
		if err != nil {
			s.log.Error("search service error: get comic by id", "comic", id, "error", err)
			return nil, err
		}
		comics = append(comics, Comics{
			ID:    comic.ID,
			URL:   comic.URL,
			Words: comic.Words,
		})
	}
	return comics, nil
}

func (s *Service) BuildIndex(ctx context.Context) error {
	newIndex := NewIndex()

	lastID, err := s.db.GetLastID(ctx)
	if err != nil {
		s.log.Error("search service error: build index (get lastID in DB)", "error", err)
		return err
	}

	total := 0
	for i := 1; i <= lastID; i++ {
		comic, err := s.db.GetById(ctx, i)
		total++
		if err != nil {
			s.log.Error("search service error: build index (get comic by id from DB)", "id", i, "error", err)
		}
		newIndex.Add(i, comic.Words)
	}

	s.index = newIndex
	s.log.Info("build index", "total", total)
	return nil
}
