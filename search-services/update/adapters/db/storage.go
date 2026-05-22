package db

import (
	"context"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/kirillveshnyakov/XKCD_searcher/search-services/update/core"
	"github.com/lib/pq"
)

type DB struct {
	log  *slog.Logger
	conn *sqlx.DB
}

func New(log *slog.Logger, address string) (*DB, error) {
	db, err := sqlx.Connect("pgx", address)
	if err != nil {
		log.Error("connection problem", "address", address, "error", err)
		return nil, err
	}
	return &DB{
		log:  log,
		conn: db,
	}, nil
}

func (db *DB) Add(ctx context.Context, comics core.Comics) error {
	query := `
		INSERT INTO comics (id, url, words)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET
			url = EXCLUDED.url,
			words = EXCLUDED.words`

	_, err := db.conn.ExecContext(ctx, query, comics.ID, comics.URL, pq.Array(comics.Words))
	if err != nil {
		db.log.Error("db error: failed to add comics", "id", comics.ID, "error", err)
		return err
	}

	return nil
}

func (db *DB) Stats(ctx context.Context) (core.DBStats, error) {
	var stats core.DBStats

	query := `
		SELECT
			(SELECT COUNT(*) FROM comics) as comicsfetched,
			COUNT(word) as wordstotal,
			COUNT(DISTINCT word) as wordsunique
		FROM comics, unnest(words) as word`

	err := db.conn.QueryRowxContext(ctx, query).StructScan(&stats)

	if err != nil {
		db.log.Debug("db error: error get stats", "error", err)
		return core.DBStats{}, err
	}

	return stats, nil
}

func (db *DB) IDs(ctx context.Context) ([]int, error) {
	var ids []int

	query := `SELECT id FROM comics WHERE url <> ''`

	err := db.conn.SelectContext(ctx, &ids, query)
	if err != nil {
		db.log.Debug("db error: error get IDs", "error", err)
		return nil, err
	}
	return ids, nil
}

func (db *DB) Drop(ctx context.Context) error {
	query := `TRUNCATE TABLE comics`

	_, err := db.conn.ExecContext(ctx, query)
	if err != nil {
		db.log.Error("db error: failed to drop table content", "error", err)
		return err
	}

	db.log.Info("table comics truncated")
	return nil
}
