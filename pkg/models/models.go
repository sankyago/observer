// Package models defines the Go structs that mirror the database schema.
// Keep in sync with migrations under ./migrations.
package models

import (
	"time"

	"github.com/google/uuid"
)

type Tenant struct {
	ID        uuid.UUID
	Slug      string
	Name      string
	CreatedAt time.Time
}

type User struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	Email        string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
}

type Device struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Name      string
	Type      string
	CreatedAt time.Time
}

type DeviceCredentials struct {
	DeviceID     uuid.UUID
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

type ActionKind string

const (
	ActionLog      ActionKind = "log"
	ActionWebhook  ActionKind = "webhook"
	ActionEmail    ActionKind = "email"
	ActionWorkflow ActionKind = "workflow"
)

type Action struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Kind      ActionKind
	Config    []byte // raw JSON
	CreatedAt time.Time
}

type RuleOp string

const (
	OpGT  RuleOp = ">"
	OpLT  RuleOp = "<"
	OpGTE RuleOp = ">="
	OpLTE RuleOp = "<="
	OpEQ  RuleOp = "="
	OpNEQ RuleOp = "!="
)

type Rule struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	DeviceID   uuid.UUID
	Field      string
	Op         RuleOp
	Value      float64
	ActionID   uuid.UUID
	Enabled    bool
	DebugUntil *time.Time
	CreatedAt  time.Time
}
