package actions

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Input struct {
	Kind      string
	Config    []byte // JSON
	FlowID    uuid.UUID
	NodeID    string
	DeviceID  uuid.UUID
	TenantID  uuid.UUID
	MessageID uuid.UUID
	Payload   []byte
}

type Runner interface {
	Run(ctx context.Context, in Input) error
}

type Registry struct {
	Log     Runner
	Webhook Runner
	Email   Runner
	Linear  Runner
}

func (r Registry) Run(ctx context.Context, in Input) error {
	switch in.Kind {
	case "log":
		if r.Log == nil {
			return fmt.Errorf("log runner not configured")
		}
		return r.Log.Run(ctx, in)
	case "webhook":
		if r.Webhook == nil {
			return fmt.Errorf("webhook runner not configured")
		}
		return r.Webhook.Run(ctx, in)
	case "email":
		if r.Email == nil {
			return fmt.Errorf("email runner not configured")
		}
		return r.Email.Run(ctx, in)
	case "linear":
		if r.Linear == nil {
			return fmt.Errorf("linear runner not configured")
		}
		return r.Linear.Run(ctx, in)
	default:
		return fmt.Errorf("unknown action kind: %s", in.Kind)
	}
}
