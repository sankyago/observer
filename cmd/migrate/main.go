// Command migrate runs goose database migrations from ./migrations.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/observer-io/observer/pkg/config"
	"github.com/observer-io/observer/pkg/log"
)

func main() {
	dir := flag.String("dir", "migrations", "migrations directory")
	flag.Parse()
	cmd := flag.Arg(0)
	if cmd == "" {
		cmd = "up"
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	logger := log.New(cfg.Log.Level)

	c, err := pgxOpen(cfg.DB.DSN)
	if err != nil {
		logger.Error("open db", "err", err)
		os.Exit(1)
	}
	defer c.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		logger.Error("dialect", "err", err)
		os.Exit(1)
	}
	if err := goose.RunContext(context.Background(), cmd, c, *dir); err != nil {
		logger.Error("migrate", "err", err)
		os.Exit(1)
	}
	logger.Info("migrate done", "cmd", cmd)
}

func pgxOpen(dsn string) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	return stdlib.OpenDB(*cfg), nil
}
