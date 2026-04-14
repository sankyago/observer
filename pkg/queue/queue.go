// Package queue defines the abstract job queue used between transport and runner.
// Implementations: inmem (Mode A, monolith) and river (Mode B, split services).
package queue

import (
	"context"

	"github.com/google/uuid"
)

type Job struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	DeviceID      uuid.UUID
	RuleID        uuid.UUID
	ActionID      uuid.UUID
	MessageID     uuid.UUID
	Payload       []byte // raw telemetry JSON snapshot
	CorrelationID uuid.UUID
}

type Handler func(ctx context.Context, j Job) error

type Queue interface {
	Enqueue(ctx context.Context, j Job) error
	Consume(ctx context.Context, h Handler) error // blocks until ctx done
	Close() error
}
