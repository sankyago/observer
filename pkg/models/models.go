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

type Flow struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Name      string
	Graph     []byte // JSON bytes — {nodes, edges}
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
