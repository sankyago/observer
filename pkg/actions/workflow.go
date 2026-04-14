package actions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/observer-io/observer/pkg/models"
)

// WorkflowAction executes a DAG of sub-actions. The action's Config is a JSON
// object of shape:
//
//	{
//	  "nodes": [{"id": "n1", "kind": "log", "config": {}}, ...],
//	  "edges": [{"source": "n1", "target": "n2"}, ...]
//	}
//
// Nodes are executed in topological order. Execution stops on the first error.
type WorkflowAction struct {
	Registry *Registry // injected non-recursive runners (log/webhook/email)
}

type workflowNode struct {
	ID     string          `json:"id"`
	Kind   string          `json:"kind"`
	Config json.RawMessage `json:"config"`
}

type workflowEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type workflowConfig struct {
	Nodes []workflowNode `json:"nodes"`
	Edges []workflowEdge `json:"edges"`
}

func (a WorkflowAction) Run(ctx context.Context, in Input) error {
	var cfg workflowConfig
	if err := json.Unmarshal(in.Action.Config, &cfg); err != nil {
		return fmt.Errorf("parse workflow: %w", err)
	}
	if len(cfg.Nodes) == 0 {
		return fmt.Errorf("workflow has no nodes")
	}
	order, err := topoSort(cfg.Nodes, cfg.Edges)
	if err != nil {
		return err
	}
	nodeByID := make(map[string]workflowNode, len(cfg.Nodes))
	for _, n := range cfg.Nodes {
		nodeByID[n.ID] = n
	}
	for _, id := range order {
		n := nodeByID[id]
		subAction := in.Action
		subAction.Kind = models.ActionKind(n.Kind)
		subAction.Config = []byte(n.Config)
		if err := a.Registry.Run(ctx, Input{
			Action: subAction, RuleID: in.RuleID, DeviceID: in.DeviceID,
			TenantID: in.TenantID, MessageID: in.MessageID, Payload: in.Payload,
		}); err != nil {
			return fmt.Errorf("node %s (%s): %w", n.ID, n.Kind, err)
		}
	}
	return nil
}

// topoSort returns node IDs in execution order. Nodes with no incoming edges
// come first; among equals, original order is preserved.
func topoSort(nodes []workflowNode, edges []workflowEdge) ([]string, error) {
	inDeg := make(map[string]int, len(nodes))
	for _, n := range nodes {
		inDeg[n.ID] = 0
	}
	outs := make(map[string][]string, len(nodes))
	for _, e := range edges {
		if _, ok := inDeg[e.Target]; !ok {
			continue
		}
		if _, ok := inDeg[e.Source]; !ok {
			continue
		}
		inDeg[e.Target]++
		outs[e.Source] = append(outs[e.Source], e.Target)
	}
	var queue []string
	for _, n := range nodes {
		if inDeg[n.ID] == 0 {
			queue = append(queue, n.ID)
		}
	}
	var order []string
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		order = append(order, id)
		for _, next := range outs[id] {
			inDeg[next]--
			if inDeg[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	if len(order) != len(nodes) {
		return nil, fmt.Errorf("workflow contains a cycle")
	}
	return order, nil
}
