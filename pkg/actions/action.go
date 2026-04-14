// Package actions defines the Action interface and concrete implementations.
package actions

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/observer-io/observer/pkg/models"
)

type Input struct {
	Action    models.Action
	RuleID    uuid.UUID
	DeviceID  uuid.UUID
	TenantID  uuid.UUID
	MessageID uuid.UUID
	Payload   []byte
}

type Runner interface {
	Run(ctx context.Context, in Input) error
}

type Registry struct {
	Log      Runner
	Webhook  Runner
	Workflow Runner
}

func (r *Registry) Run(ctx context.Context, in Input) error {
	switch in.Action.Kind {
	case models.ActionLog:
		if r.Log == nil {
			return fmt.Errorf("log action not configured")
		}
		return r.Log.Run(ctx, in)
	case models.ActionWebhook:
		if r.Webhook == nil {
			return fmt.Errorf("webhook action not configured")
		}
		return r.Webhook.Run(ctx, in)
	case models.ActionWorkflow:
		if r.Workflow == nil {
			return fmt.Errorf("workflow action not configured")
		}
		return r.Workflow.Run(ctx, in)
	default:
		return fmt.Errorf("unknown action kind: %s", in.Action.Kind)
	}
}
