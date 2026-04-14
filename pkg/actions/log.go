package actions

import (
	"context"
	"log/slog"
)

type LogAction struct {
	Logger *slog.Logger
}

func (a LogAction) Run(_ context.Context, in Input) error {
	a.Logger.Info("ALERT",
		"flow_id", in.FlowID,
		"node_id", in.NodeID,
		"device_id", in.DeviceID,
		"message_id", in.MessageID,
		"payload", string(in.Payload),
	)
	return nil
}
