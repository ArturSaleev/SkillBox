package migrate

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed sql/*.sql
var files embed.FS

func Run(ctx context.Context, db *sql.DB, dialect string) error {
	entries, err := fs.ReadDir(files, "sql")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	if _, err := db.ExecContext(ctx, migrationTable(dialect)); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), dialect+"_") {
			continue
		}
		parts := strings.SplitN(strings.TrimPrefix(entry.Name(), dialect+"_"), "_", 2)
		version, err := strconv.Atoi(parts[0])
		if err != nil {
			return err
		}
		var count int
		if err := db.QueryRowContext(ctx, bind(dialect, "SELECT COUNT(*) FROM schema_migrations WHERE version = ?"), version).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		body, err := files.ReadFile("sql/" + entry.Name())
		if err != nil {
			return err
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		for _, statement := range strings.Split(string(body), ";") {
			statement = strings.TrimSpace(statement)
			if statement == "" {
				continue
			}
			if _, err = tx.ExecContext(ctx, statement); err != nil {
				break
			}
		}
		if err == nil {
			_, err = tx.ExecContext(ctx, bind(dialect, "INSERT INTO schema_migrations(version,applied_at) VALUES(?,?)"), version, time.Now().UTC().Format(time.RFC3339Nano))
		}
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %s: %w", entry.Name(), err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func migrationTable(dialect string) string {
	if dialect == "mysql" {
		return "CREATE TABLE IF NOT EXISTS schema_migrations (version INT PRIMARY KEY, applied_at VARCHAR(40) NOT NULL)"
	}
	return "CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at VARCHAR(40) NOT NULL)"
}

func bind(dialect, q string) string {
	if dialect != "postgres" {
		return q
	}
	var b strings.Builder
	n := 0
	for _, r := range q {
		if r == '?' {
			n++
			b.WriteString("$")
			b.WriteString(strconv.Itoa(n))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
