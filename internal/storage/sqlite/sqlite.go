package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/aibox/skillbox/internal/migrate"
	"github.com/aibox/skillbox/internal/storage/sqlstore"
	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, path string, runMigrations bool) (*sqlstore.Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+abs+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if runMigrations {
		if err = migrate.Run(ctx, db, "sqlite"); err != nil {
			db.Close()
			return nil, fmt.Errorf("sqlite migrate: %w", err)
		}
	}
	return sqlstore.New(db, "sqlite"), nil
}
