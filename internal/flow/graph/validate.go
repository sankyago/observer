package graph

import (
	"encoding/json"
	"fmt"
)

var KnownTypes = map[string]func(json.RawMessage) error{
	"mqtt_source":    validateMQTTSource,
	"threshold":      validateThreshold,
	"rate_of_change": validateRateOfChange,
	"debug_sink":     validateDebugSink,
}

func Validate(g Graph) error {
	ids := make(map[string]struct{}, len(g.Nodes))
	for _, n := range g.Nodes {
		if n.ID == "" {
			return fmt.Errorf("node has empty id")
		}
		if _, dup := ids[n.ID]; dup {
			return fmt.Errorf("duplicate node id %q", n.ID)
		}
		ids[n.ID] = struct{}{}
		v, ok := KnownTypes[n.Type]
		if !ok {
			return fmt.Errorf("unknown node type %q on node %q", n.Type, n.ID)
		}
		if err := v(n.Data); err != nil {
			return fmt.Errorf("node %q: %w", n.ID, err)
		}
	}
	for _, e := range g.Edges {
		if _, ok := ids[e.Source]; !ok {
			return fmt.Errorf("edge %s: source %q not found", e.ID, e.Source)
		}
		if _, ok := ids[e.Target]; !ok {
			return fmt.Errorf("edge %s: target %q not found", e.ID, e.Target)
		}
	}
	return detectCycle(g)
}

func detectCycle(g Graph) error {
	adj := make(map[string][]string, len(g.Nodes))
	for _, e := range g.Edges {
		adj[e.Source] = append(adj[e.Source], e.Target)
	}
	color := make(map[string]int) // 0=white, 1=gray, 2=black
	var visit func(string) error
	visit = func(id string) error {
		switch color[id] {
		case 1:
			return fmt.Errorf("cycle detected at node %q", id)
		case 2:
			return nil
		}
		color[id] = 1
		for _, next := range adj[id] {
			if err := visit(next); err != nil {
				return err
			}
		}
		color[id] = 2
		return nil
	}
	for _, n := range g.Nodes {
		if err := visit(n.ID); err != nil {
			return err
		}
	}
	return nil
}

type thresholdCfg struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

func validateThreshold(raw json.RawMessage) error {
	var c thresholdCfg
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("threshold data: %w", err)
	}
	if c.Min >= c.Max {
		return fmt.Errorf("threshold: min must be < max")
	}
	return nil
}

type rateCfg struct {
	MaxPerSecond float64 `json:"max_per_second"`
	WindowSize   int     `json:"window_size"`
}

func validateRateOfChange(raw json.RawMessage) error {
	var c rateCfg
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("rate_of_change data: %w", err)
	}
	if c.MaxPerSecond <= 0 {
		return fmt.Errorf("rate_of_change: max_per_second must be > 0")
	}
	if c.WindowSize < 2 {
		return fmt.Errorf("rate_of_change: window_size must be >= 2")
	}
	return nil
}

type mqttSourceCfg struct {
	Broker   string `json:"broker"`
	Topic    string `json:"topic"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

func validateMQTTSource(raw json.RawMessage) error {
	var c mqttSourceCfg
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("mqtt_source data: %w", err)
	}
	if c.Broker == "" {
		return fmt.Errorf("mqtt_source: broker required")
	}
	if c.Topic == "" {
		return fmt.Errorf("mqtt_source: topic required")
	}
	return nil
}

func validateDebugSink(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var any map[string]any
	if err := json.Unmarshal(raw, &any); err != nil {
		return fmt.Errorf("debug_sink data: %w", err)
	}
	return nil
}
