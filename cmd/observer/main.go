package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sankyago/observer/internal/api"
	"github.com/sankyago/observer/internal/db"
	"github.com/sankyago/observer/internal/flow"
	"github.com/sankyago/observer/internal/flow/runtime"
	"github.com/sankyago/observer/internal/flow/store"
)

func main() {
	dbURL := envOrDefault("DATABASE_URL", "postgres://observer:observer@localhost:5432/observer")
	addr := envOrDefault("HTTP_ADDR", ":8080")
	migrationsDir := envOrDefault("MIGRATIONS_DIR", "migrations")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("pgx connect: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, migrationsDir); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	repo := store.NewRepo(pool)
	mgr := runtime.NewManager()
	svc := flow.NewService(ctx, repo, mgr)

	if err := svc.LoadEnabled(ctx); err != nil {
		log.Fatalf("load enabled flows: %v", err)
	}

	server := &http.Server{
		Addr:         addr,
		Handler:      api.NewRouter(svc, nil),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0, // WS needs no write timeout
	}

	go func() {
		log.Printf("listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)
	mgr.StopAll()
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
