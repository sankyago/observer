package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sankyago/observer/internal/api"
	"github.com/sankyago/observer/internal/db"
	"github.com/sankyago/observer/internal/devices"
	devstore "github.com/sankyago/observer/internal/devices/store"
	"github.com/sankyago/observer/internal/flow"
	"github.com/sankyago/observer/internal/flow/runtime"
	flowstore "github.com/sankyago/observer/internal/flow/store"
	"github.com/sankyago/observer/internal/ingest"
)

func main() {
	dbURL := env("DATABASE_URL", "postgres://observer:observer@localhost:5432/observer")
	addr := env("HTTP_ADDR", ":8080")
	migDir := env("MIGRATIONS_DIR", "migrations")
	brokerURL := env("EMQX_BROKER_URL", "tcp://localhost:1883")
	mqUser := env("EMQX_USERNAME", "observer-consumer")
	mqPass := env("EMQX_PASSWORD", "")
	mqGroup := env("EMQX_SHARED_GROUP", "observer")
	secret := env("MQTT_WEBHOOK_SECRET", "")

	if mqPass == "" || secret == "" {
		log.Fatal("EMQX_PASSWORD and MQTT_WEBHOOK_SECRET are required")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("pg: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, migDir); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	devRepo := devstore.NewRepo(pool)
	// Build mqttAuth first so we can pass Invalidate as a callback into devSvc.
	// devSvc is injected into mqttAuth after construction via SetService.
	mqttAuth := api.NewMQTTAuthHandler(secret, mqUser)
	devSvc := devices.NewService(devRepo, devices.WithTokenInvalidator(mqttAuth.Invalidate))
	mqttAuth.SetService(devSvc)
	router := ingest.NewRouter()
	mgr := runtime.NewManager(router)
	flowSvc := flow.NewService(ctx, flowstore.NewRepo(pool), mgr)

	if err := flowSvc.LoadEnabled(ctx); err != nil {
		log.Fatalf("load flows: %v", err)
	}

	consumer := ingest.NewConsumer(ingest.Config{
		BrokerURL:   brokerURL,
		Username:    mqUser,
		Password:    mqPass,
		SharedGroup: mqGroup,
		ClientID:    fmt.Sprintf("observer-%d", time.Now().UnixNano()),
	}, router)

	go func() {
		if err := consumer.Run(ctx); err != nil {
			log.Printf("consumer exited: %v", err)
		}
	}()

	httpHandler := api.NewRouter(flowSvc, devSvc, api.WithMQTTAuthHandler(mqttAuth))
	srv := &http.Server{Addr: addr, Handler: httpHandler, ReadTimeout: 10 * time.Second}

	go func() {
		log.Printf("listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	shutdownCtx, sc := context.WithTimeout(context.Background(), 5*time.Second)
	defer sc()
	_ = srv.Shutdown(shutdownCtx)
	mgr.StopAll()
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
