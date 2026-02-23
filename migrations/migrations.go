// Package migrations embeds SQL migration files and provides a function to apply them.
package migrations

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

// FS contains the embedded SQL migration files.
//
//go:embed *.sql
var FS embed.FS

// Run applies all pending migrations to the given database.
func Run(db *sql.DB) error {
	goose.SetBaseFS(FS)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
