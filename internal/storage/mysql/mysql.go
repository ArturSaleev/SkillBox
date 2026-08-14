package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/aibox/skillbox/internal/migrate"
	"github.com/aibox/skillbox/internal/storage/sqlstore"
	_ "github.com/go-sql-driver/mysql"
)

func Open(ctx context.Context, dsn string, runMigrations bool) (*sqlstore.Store, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if runMigrations {
		if err = migrate.Run(ctx, db, "mysql"); err != nil {
			db.Close()
			return nil, fmt.Errorf("mysql migrate: %w", err)
		}
	}
	return sqlstore.New(db, "mysql"), nil
}
