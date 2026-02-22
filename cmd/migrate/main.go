package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"rss_bot/migrations"
)

func main() {
	dbPath := flag.String("db", envOrDefault("DATABASE_PATH", "./data/bot.db"), "path to sqlite database")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: migrate [-db path] <command>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "  up          Migrate to the latest version")
		fmt.Fprintln(os.Stderr, "  up-one      Migrate one version up")
		fmt.Fprintln(os.Stderr, "  down        Roll back one version")
		fmt.Fprintln(os.Stderr, "  status      Show migration status")
		fmt.Fprintln(os.Stderr, "  version     Show current version")
		fmt.Fprintln(os.Stderr, "  reset       Roll back all migrations")
		os.Exit(1)
	}

	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		log.Fatalf("set dialect: %v", err)
	}

	cmd := args[0]
	switch cmd {
	case "up":
		err = goose.Up(db, ".")
	case "up-one":
		err = goose.UpByOne(db, ".")
	case "down":
		err = goose.Down(db, ".")
	case "status":
		err = goose.Status(db, ".")
	case "version":
		err = goose.Version(db, ".")
	case "reset":
		err = goose.Reset(db, ".")
	default:
		log.Fatalf("unknown command: %s", cmd)
	}

	if err != nil {
		log.Fatalf("%s: %v", cmd, err)
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
