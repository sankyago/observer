package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sankyago/observer/internal/engine"
	"github.com/sankyago/observer/internal/flusher"
	"github.com/sankyago/observer/internal/model"
	"github.com/sankyago/observer/internal/subscriber"
)

func main() {
	brokerURL := envOrDefault("MQTT_BROKER", "tcp://localhost:1883")
	mqttTopic := envOrDefault("MQTT_TOPIC", "sensors/#")
	databaseURL := envOrDefault("DATABASE_URL", "postgres://observer:observer@localhost:5432/observer")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Database
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("database ping failed: %v", err)
	}
	log.Println("connected to TimescaleDB")

	// Channels
	mqttToEngine := make(chan model.SensorReading, 1000)
	engineToFlusher := make(chan model.SensorReading, 1000)

	// Subscriber
	sub := subscriber.New(brokerURL, mqttTopic, mqttToEngine)
	if err := sub.Run(brokerURL); err != nil {
		log.Fatalf("mqtt subscriber failed: %v", err)
	}
	defer sub.Stop()

	// Engine
	eng := engine.NewEngine()
	go eng.Run(mqttToEngine, engineToFlusher)

	// Flusher
	f := flusher.NewFlusher(pool, engineToFlusher)
	go f.Run(ctx)

	log.Println("observer started")

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("shutting down...")
	sub.Stop()
	close(mqttToEngine)
	cancel()
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
