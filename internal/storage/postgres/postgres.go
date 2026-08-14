package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/aibox/skillbox/internal/migrate"
	"github.com/aibox/skillbox/internal/storage/sqlstore"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func Open(ctx context.Context, dsn string, runMigrations bool) (*sqlstore.Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if runMigrations {
		if err = migrate.Run(ctx, db, "postgres"); err != nil {
			db.Close()
			return nil, fmt.Errorf("postgres migrate: %w", err)
		}
	}
	return sqlstore.New(db, "postgres"), nil
}
