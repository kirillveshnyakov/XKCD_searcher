package core

import (
	"context"
)

type Searcher interface {
	Search(context.Context, string, int) ([]Comics, error)
	SearchIndex(context.Context, string, int) ([]Comics, error)
	BuildIndex(ctx context.Context) error
}

type DB interface {
	Find(ctx context.Context, words []string, limit int) ([]Comics, error)
	GetById(ctx context.Context, id int) (Comics, error)
	GetLastID(ctx context.Context) (int, error)
}

type Words interface {
	Norm(ctx context.Context, phrase string) ([]string, error)
}
