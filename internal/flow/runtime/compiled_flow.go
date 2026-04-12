package runtime

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/sankyago/observer/internal/flow/graph"
	"github.com/sankyago/observer/internal/flow/nodes"
	"github.com/sankyago/observer/internal/model"
)

type CompiledFlow struct {
	nodes   []nodeRuntime
	bus     *EventBus
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	stopped chan struct{}
}

type nodeRuntime struct {
	node nodes.Node
	in   chan model.SensorReading
	out  chan model.SensorReading
}

func Compile(g graph.Graph) (*CompiledFlow, error) {
	return CompileWithSinkWriter(g, os.Stderr)
}

func CompileWithSinkWriter(g graph.Graph, sinkOut io.Writer) (*CompiledFlow, error) {
	instances := make(map[string]*nodeRuntime, len(g.Nodes))
	for _, n := range g.Nodes {
		inst, err := buildNode(n, sinkOut)
		if err != nil {
			return nil, err
		}
		instances[n.ID] = &nodeRuntime{node: inst}
	}

	// Wire each edge: source.out -> target.in.
	// v1: a node has at most one `out` channel.
	// Multiple downstream nodes would need a fan-out; for now reject.
	outUsed := map[string]bool{}
	inUsed := map[string]bool{}
	for _, e := range g.Edges {
		if outUsed[e.Source] {
			return nil, fmt.Errorf("node %q has multiple outgoing edges (not supported in v1)", e.Source)
		}
		if inUsed[e.Target] {
			return nil, fmt.Errorf("node %q has multiple incoming edges (not supported in v1)", e.Target)
		}
		outUsed[e.Source] = true
		inUsed[e.Target] = true
		ch := make(chan model.SensorReading, 64)
		instances[e.Source].out = ch
		instances[e.Target].in = ch
	}

	cf := &CompiledFlow{bus: NewEventBus(), stopped: make(chan struct{})}
	for _, n := range g.Nodes {
		cf.nodes = append(cf.nodes, *instances[n.ID])
	}
	return cf, nil
}

func buildNode(n graph.Node, sinkOut io.Writer) (nodes.Node, error) {
	switch n.Type {
	case "mqtt_source":
		return nodes.NewMQTTSource(n.ID, n.Data)
	case "threshold":
		return nodes.NewThreshold(n.ID, n.Data)
	case "rate_of_change":
		return nodes.NewRateOfChange(n.ID, n.Data)
	case "debug_sink":
		return nodes.NewDebugSink(n.ID, sinkOut), nil
	default:
		return nil, fmt.Errorf("unknown node type %q", n.Type)
	}
}

func (cf *CompiledFlow) Bus() *EventBus { return cf.bus }

func (cf *CompiledFlow) Start(parent context.Context) error {
	ctx, cancel := context.WithCancel(parent)
	cf.cancel = cancel

	eventsCh := make(chan nodes.FlowEvent, 256)
	cf.wg.Add(1)
	go func() {
		defer cf.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-eventsCh:
				if !ok {
					return
				}
				cf.bus.Publish(e)
			}
		}
	}()

	for i := range cf.nodes {
		nr := cf.nodes[i]
		cf.wg.Add(1)
		go func() {
			defer cf.wg.Done()
			_ = nr.node.Run(ctx, nr.in, nr.out, eventsCh)
		}()
	}
	return nil
}

func (cf *CompiledFlow) Stop() {
	if cf.cancel != nil {
		cf.cancel()
	}
	cf.wg.Wait()
	cf.bus.Close()
}
