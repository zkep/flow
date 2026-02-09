package flow

import (
	"reflect"
)

func (g *Graph) SaveCheckpoint() (*Checkpoint, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	checkpoint := NewCheckpoint(CheckpointTypeGraph)

	steps := make([]StepState, 0, len(g.nodes))
	executed := make([]string, 0)
	pending := make([]string, 0)

	for name, node := range g.nodes {
		node.mu.RLock()
		step := StepState{
			Name:   name,
			Status: int(node.status),
		}

		switch node.status {
		case NodeStatusCompleted, NodeStatusFailed:
			step.Executed = true
			executed = append(executed, name)
		case NodeStatusPending, NodeStatusRunning:
			step.Executed = false
			pending = append(pending, name)
		}
		node.mu.RUnlock()

		steps = append(steps, step)
	}

	nodeResults := make(map[string][]any)
	for name, node := range g.nodes {
		node.mu.RLock()
		if len(node.result) > 0 {
			nodeResults[name] = append([]any{}, node.result...)
		}
		node.mu.RUnlock()
	}

	checkpoint.Data.Steps = steps
	checkpoint.Data.Current = len(executed) - 1
	checkpoint.Data.Extra = map[string]any{
		"node_results":   nodeResults,
		"executed":       executed,
		"pending":        pending,
		"paused_at_node": g.pausedAtNode,
	}

	switch {
	case g.err != nil:
		checkpoint.Data.Error = g.err.Error()
		checkpoint.State = FlowStateFailed
	case len(pending) == 0:
		checkpoint.State = FlowStateCompleted
	case len(executed) > 0:
		checkpoint.State = FlowStatePaused
	}

	return checkpoint, nil
}

func (g *Graph) LoadCheckpoint(checkpoint *Checkpoint) error {
	if checkpoint.Type != CheckpointTypeGraph {
		return ErrCheckpointInvalidType
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	data := checkpoint.Data

	for _, step := range data.Steps {
		if node, ok := g.nodes[step.Name]; ok {
			node.mu.Lock()
			node.status = NodeStatus(step.Status)
			node.mu.Unlock()
		}
	}

	if data.Extra != nil {
		if nodeResultsRaw, ok := data.Extra["node_results"]; ok {
			nodeResultsVal := reflect.ValueOf(nodeResultsRaw)
			if nodeResultsVal.Kind() == reflect.Map {
				for _, key := range nodeResultsVal.MapKeys() {
					name := key.String()
					resultsVal := nodeResultsVal.MapIndex(key)
					if resultsVal.Kind() == reflect.Interface {
						resultsVal = resultsVal.Elem()
					}
					if resultsVal.Kind() == reflect.Slice {
						rs := make([]any, resultsVal.Len())
						for i := 0; i < resultsVal.Len(); i++ {
							elem := resultsVal.Index(i)
							if elem.Kind() == reflect.Interface {
								elem = elem.Elem()
							}
							rs[i] = elem.Interface()
						}
						if node, ok := g.nodes[name]; ok {
							node.mu.Lock()
							node.result = g.convertResultsToNodeTypes(node, rs)
							node.mu.Unlock()
						}
					}
				}
			}
		}
		if pausedAtNode, ok := data.Extra["paused_at_node"].(string); ok {
			g.pausedAtNode = pausedAtNode
		}
	}

	if data.Error != "" {
		g.err = &FlowError{Message: data.Error}
	}

	return nil
}

func (g *Graph) convertResultsToNodeTypes(node *Node, results []any) []any {
	if node == nil || node.fn == nil || node.fnType == nil || len(results) == 0 {
		return results
	}

	numOut := node.numOut
	if node.hasErrorReturn {
		numOut--
	}

	if numOut == 0 {
		return results
	}

	converted := make([]any, len(results))
	for i, result := range results {
		if result == nil {
			converted[i] = nil
			continue
		}

		var targetType reflect.Type
		if i < numOut {
			targetType = node.fnType.Out(i)
		} else {
			converted[i] = result
			continue
		}

		resultVal := reflect.ValueOf(result)
		if !resultVal.Type().AssignableTo(targetType) {
			if resultVal.CanConvert(targetType) {
				converted[i] = resultVal.Convert(targetType).Interface()
			} else {
				converted[i] = result
			}
		} else {
			converted[i] = result
		}
	}

	return converted
}

func (g *Graph) SaveToStore(store CheckpointStore, key string) error {
	checkpoint, err := g.SaveCheckpoint()
	if err != nil {
		return err
	}
	return store.Save(key, checkpoint)
}

func (g *Graph) LoadFromStore(store CheckpointStore, key string) error {
	checkpoint, err := store.Load(key)
	if err != nil {
		return err
	}
	return g.LoadCheckpoint(checkpoint)
}

func (g *Graph) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.err = nil
	g.execPlanValid = false
	g.layersValid = false

	for _, node := range g.nodes {
		node.mu.Lock()
		node.status = NodeStatusPending
		node.result = nil
		node.err = nil
		node.mu.Unlock()
	}
}
