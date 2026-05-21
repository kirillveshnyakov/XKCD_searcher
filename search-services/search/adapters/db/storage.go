package db

import (
	"context"
	"fmt"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"yadro.com/course/search/core"
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

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) Find(ctx context.Context, words []string, limit int) ([]core.Comics, error) {
	query := `
        SELECT id, url 
        FROM comics 
        WHERE words && $1 
        	AND url <> ''
        ORDER BY cardinality(array(
            SELECT * FROM unnest(words) 
            WHERE unnest = ANY($1)
        )) DESC 
        LIMIT $2`

	var result []core.Comics
	err := db.conn.SelectContext(ctx, &result, query, pq.Array(words), limit)
	if err != nil {
		return nil, fmt.Errorf("query Find failed: %w", err)
	}

	return result, nil
}

type Comics struct {
	ID    int            `db:"id"`
	URL   string         `db:"url"`
	Words pq.StringArray `db:"words"`
}

func (db *DB) GetById(ctx context.Context, id int) (core.Comics, error) {
	query := `
		SELECT id, url, words
		FROM comics 
		WHERE id = $1`

	var comics Comics
	err := db.conn.GetContext(ctx, &comics, query, id)
	if err != nil {
		return core.Comics{}, fmt.Errorf("query GetById failed: %w", err)
	}
	return core.Comics{
		ID:    comics.ID,
		URL:   comics.URL,
		Words: comics.Words,
	}, nil
}

func (db *DB) GetLastID(ctx context.Context) (int, error) {
	query := `
		SELECT coalesce(max(id), 0) 
		FROM comics`

	var id int
	err := db.conn.GetContext(ctx, &id, query)
	if err != nil {
		return 0, fmt.Errorf("query GetLastId failed: %w", err)
	}
	return id, nil
}
