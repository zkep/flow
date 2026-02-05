package flow

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

const (
	ErrNodeNotFound     = "node not found"
	ErrDuplicateNode    = "duplicate node name"
	ErrSelfDependency   = "node cannot depend on itself"
	ErrCyclicDependency = "cyclic dependency detected"
	ErrNoStartNode      = "no start node found"
	ErrExecutionFailed  = "execution failed"
)

type NodeType int

const (
	NodeTypeNormal NodeType = iota
	NodeTypeStart
	NodeTypeEnd
	NodeTypeBranch
	NodeTypeParallel
	NodeTypeLoop
)

type NodeStatus int

const (
	NodeStatusPending NodeStatus = iota
	NodeStatusRunning
	NodeStatusCompleted
	NodeStatusFailed
)

type Edge struct {
	from   string
	to     string
	cond   any
	weight int
}

type Node struct {
	name        string
	nodeType    NodeType
	status      NodeStatus
	fn          any
	description string
	inputs      []string
	outputs     []string
	err         error
	result      []any
	waitGroup   sync.WaitGroup
}

type NodeExecution struct {
	node    *Node
	results []any
	err     error
}

type Graph struct {
	nodes     map[string]*Node
	edges     map[string][]*Edge
	inDegree  map[string]int
	outDegree map[string]int
	stepNames map[string]int
	err       error
	mu        sync.Mutex
}

func NewGraph() *Graph {
	return &Graph{
		nodes:     make(map[string]*Node),
		edges:     make(map[string][]*Edge),
		inDegree:  make(map[string]int),
		outDegree: make(map[string]int),
		stepNames: make(map[string]int),
	}
}

func (g *Graph) addNode(name string, fn any, nodeType NodeType) *Graph {
	if g.err != nil {
		return g
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[name]; exists {
		g.err = &ChainError{Message: ErrDuplicateNode}
		return g
	}

	node := &Node{
		name:     name,
		nodeType: nodeType,
		status:   NodeStatusPending,
		fn:       fn,
	}

	g.nodes[name] = node
	g.inDegree[name] = 0
	g.outDegree[name] = 0

	return g
}

func (g *Graph) Node(name string, fn any) *Graph {
	return g.addNode(name, fn, NodeTypeNormal)
}

func (g *Graph) StartNode(name string, fn any) *Graph {
	return g.addNode(name, fn, NodeTypeStart)
}

func (g *Graph) EndNode(name string, fn any) *Graph {
	return g.addNode(name, fn, NodeTypeEnd)
}

func (g *Graph) BranchNode(name string, fn any) *Graph {
	return g.addNode(name, fn, NodeTypeBranch)
}

func (g *Graph) ParallelNode(name string, fn any) *Graph {
	return g.addNode(name, fn, NodeTypeParallel)
}

func (g *Graph) LoopNode(name string, fn any) *Graph {
	return g.addNode(name, fn, NodeTypeLoop)
}

func (g *Graph) AddEdge(from, to string) *Graph {
	return g.AddEdgeWithCondition(from, to, nil)
}

func (g *Graph) AddEdgeWithCondition(from, to string, cond any) *Graph {
	if g.err != nil {
		return g
	}

	if from == to {
		g.err = &ChainError{Message: ErrSelfDependency}
		return g
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[from]; !exists {
		g.err = &ChainError{Message: fmt.Sprintf("%s: %s", ErrNodeNotFound, from)}
		return g
	}

	if _, exists := g.nodes[to]; !exists {
		g.err = &ChainError{Message: fmt.Sprintf("%s: %s", ErrNodeNotFound, to)}
		return g
	}

	if g.HasCycle(from, to) {
		g.err = &ChainError{Message: ErrCyclicDependency}
		return g
	}

	edge := &Edge{
		from: from,
		to:   to,
		cond: cond,
	}

	g.edges[from] = append(g.edges[from], edge)
	g.inDegree[to]++
	g.outDegree[from]++

	return g
}

func (g *Graph) HasCycle(from, to string) bool {
	visited := make(map[string]bool)
	path := make(map[string]bool)

	var dfs func(node string) bool
	dfs = func(node string) bool {
		if path[node] {
			return true
		}
		if visited[node] {
			return false
		}

		path[node] = true
		visited[node] = true

		for _, edge := range g.edges[node] {
			nextNode := edge.to
			if nextNode == from || dfs(nextNode) {
				return true
			}
		}

		path[node] = false
		return false
	}

	return dfs(to)
}

func (g *Graph) FindStartNode() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	for name, node := range g.nodes {
		if node.nodeType == NodeTypeStart {
			return name
		}
	}

	for name := range g.nodes {
		if g.inDegree[name] == 0 {
			return name
		}
	}

	return ""
}

func (g *Graph) FindEndNodes() []string {
	g.mu.Lock()
	defer g.mu.Unlock()

	var endNodes []string

	for name, node := range g.nodes {
		if node.nodeType == NodeTypeEnd {
			endNodes = append(endNodes, name)
			continue
		}
		if g.outDegree[name] == 0 {
			endNodes = append(endNodes, name)
		}
	}

	return endNodes
}

func (g *Graph) buildExecutionPlan() ([]string, error) {
	plan := make([]string, 0)
	visited := make(map[string]bool)
	queue := make([]string, 0)

	startNode := g.FindStartNode()
	if startNode == "" {
		return nil, &ChainError{Message: ErrNoStartNode}
	}

	for name, inDegree := range g.inDegree {
		if inDegree == 0 {
			queue = append(queue, name)
		}
	}

	if len(queue) == 0 {
		queue = append(queue, startNode)
	}

	tempInDegree := make(map[string]int)
	for name, degree := range g.inDegree {
		tempInDegree[name] = degree
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}

		plan = append(plan, current)
		visited[current] = true

		for _, edge := range g.edges[current] {
			nextNode := edge.to
			tempInDegree[nextNode]--
			if tempInDegree[nextNode] == 0 {
				queue = append(queue, nextNode)
			}
		}
	}

	if len(plan) != len(g.nodes) {
		return nil, &ChainError{Message: ErrCyclicDependency}
	}

	return plan, nil
}

func (g *Graph) evaluateCondition(cond any, results []any) bool {
	if cond == nil {
		return true
	}

	fnValue := reflect.ValueOf(cond)
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func {
		val := cond
		if b, ok := val.(bool); ok {
			return b
		}
		return true
	}

	var args []reflect.Value
	argCount := fnType.NumIn()

	// Prepare arguments for condition function
	if len(results) > 0 {
		if fnType.IsVariadic() {
			// Variadic function: need to handle differently
			if len(results) > 0 {
				// Check if we need to convert to variadic type
				sliceType := fnType.In(argCount - 1).Elem()
				slice := reflect.MakeSlice(reflect.SliceOf(sliceType), 0, len(results))
				for _, result := range results {
					slice = reflect.Append(slice, reflect.ValueOf(result))
				}
				args = append(args, slice)
			}
		} else if argCount == 1 {
			// Single parameter function: pass first result (if available)
			args = append(args, reflect.ValueOf(results[0]))
		} else if argCount == len(results) {
			// Multi-parameter function with matching argument count: pass all results
			for _, result := range results {
				args = append(args, reflect.ValueOf(result))
			}
		} else if argCount > len(results) {
			// Function expects more parameters: fill missing parameters with nil
			for _, result := range results {
				args = append(args, reflect.ValueOf(result))
			}
			for i := len(results); i < argCount; i++ {
				args = append(args, reflect.Zero(fnType.In(i)))
			}
		} else {
			// Function expects fewer parameters: only pass first N results
			for i := range argCount {
				args = append(args, reflect.ValueOf(results[i]))
			}
		}
	} else if argCount > 0 {
		// No results but function expects parameters: fill all parameters with nil
		for i := range argCount {
			args = append(args, reflect.Zero(fnType.In(i)))
		}
	}

	// Call condition function
	var condResult []reflect.Value
	if fnType.IsVariadic() {
		condResult = fnValue.CallSlice(args)
	} else {
		condResult = fnValue.Call(args)
	}

	// Process return results
	if len(condResult) > 0 {
		if condResult[0].Kind() == reflect.Bool {
			return condResult[0].Bool()
		} else if condResult[0].Kind() == reflect.Interface {
			// Handle case where return type is interface
			if !condResult[0].IsNil() {
				if b, ok := condResult[0].Elem().Interface().(bool); ok {
					return b
				}
			}
		}
	}

	// Default return true
	return true
}

func (g *Graph) executeNode(nodeName string, inputs []any) ([]any, error) {
	node, exists := g.nodes[nodeName]
	if !exists {
		return nil, &ChainError{Message: ErrNodeNotFound}
	}

	node.status = NodeStatusRunning
	node.err = nil
	node.result = nil

	if node.fn == nil {
		node.status = NodeStatusCompleted
		return inputs, nil
	}

	fnValue := reflect.ValueOf(node.fn)
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func {
		return nil, &ChainError{Message: ErrNotFunction}
	}

	// Convert nil inputs to empty slice
	if inputs == nil {
		inputs = make([]any, 0)
	}

	args, err := prepareArgs(inputs, fnType)
	if err != nil {
		node.err = err
		node.status = NodeStatusFailed
		return nil, err
	}

	results := fnValue.Call(args)

	if len(results) > fnType.NumOut() {
		node.err = &ChainError{Message: ErrFunctionPanicked}
		node.status = NodeStatusFailed
		return nil, node.err
	}

	node.result = make([]any, 0, len(results))
	for _, result := range results {
		node.result = append(node.result, result.Interface())
	}

	if fnType.NumOut() > 1 {
		lastOutType := fnType.Out(fnType.NumOut() - 1)
		if lastOutType.Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			if len(results) == fnType.NumOut() {
				errValue := results[fnType.NumOut()-1]
				if !errValue.IsNil() {
					node.err = errValue.Interface().(error)
					node.status = NodeStatusFailed
					return nil, node.err
				}
			}
		}
	}

	node.status = NodeStatusCompleted
	return node.result, nil
}

func (g *Graph) executeGraphSequential() error {
	plan, err := g.buildExecutionPlan()
	if err != nil {
		return err
	}

	nodeResults := make(map[string][]any)

	for _, nodeName := range plan {
		var inputs []any
		var executed bool

		for fromName, edges := range g.edges {
			for _, edge := range edges {
				if edge.to == nodeName {
					if results, ok := nodeResults[fromName]; ok {
						if g.evaluateCondition(edge.cond, results) {
							// Always pass all results because executeNode uses prepareArgs to handle parameter counting
							inputs = append(inputs, results...)
							executed = true
							break
						}
					}
				}
			}
			if executed {
				break
			}
		}

		// Execute node even if no edges are found (start node)
		if g.inDegree[nodeName] == 0 {
			executed = true
		}

		if executed {
			results, err := g.executeNode(nodeName, inputs)
			if err != nil {
				return err
			}

			nodeResults[nodeName] = results
			g.stepNames[nodeName] = len(g.stepNames)
		}
	}

	return nil
}

func (g *Graph) executeGraphParallel() error {
	ctx := context.Background()
	return g.executeGraphParallelWithContext(ctx)
}

func (g *Graph) executeGraphParallelWithContext(ctx context.Context) error {
	plan, err := g.buildExecutionPlan()
	if err != nil {
		return err
	}

	nodeResults := make(map[string][]any)
	nodeExecuted := make(map[string]bool)
	nodeFailed := make(map[string]bool)
	nodeRunning := make(map[string]bool)
	var mu sync.RWMutex

	type step struct {
		nodeName   string
		retryCount int
	}

	maxRetries := 1000
	queue := []step{}
	for _, nodeName := range plan {
		queue = append(queue, step{nodeName: nodeName, retryCount: 0})
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(plan))

	for len(queue) > 0 {
		select {
		case <-ctx.Done():
			return &ChainError{Message: fmt.Sprintf("execution canceled: %v", ctx.Err())}
		default:
		}

		select {
		case err := <-errChan:
			wg.Wait()
			close(errChan)
			return err
		default:
		}

		stepItem := queue[0]
		queue = queue[1:]

		mu.RLock()
		executed := nodeExecuted[stepItem.nodeName]
		failed := nodeFailed[stepItem.nodeName]
		running := nodeRunning[stepItem.nodeName]
		mu.RUnlock()

		if executed || failed {
			continue
		}

		if running {
			queue = append(queue, stepItem)
			continue
		}

		if stepItem.retryCount > maxRetries {
			return &ChainError{Message: fmt.Sprintf("node %s exceeded max retries: %d", stepItem.nodeName, maxRetries)}
		}

		canExecute := false
		var inputs []any

		if g.inDegree[stepItem.nodeName] == 0 {
			canExecute = true
		} else {
			allDependenciesMet := true
			dependencyFailed := false
			dependencySkipped := false
			hasValidEdge := false
			var validInputs []any

			var incomingEdges []*Edge
			for _, edges := range g.edges {
				for _, edge := range edges {
					if edge.to == stepItem.nodeName {
						incomingEdges = append(incomingEdges, edge)
					}
				}
			}

			for _, edge := range incomingEdges {
				mu.RLock()
				fromFailed := nodeFailed[edge.from]
				fromExecuted := nodeExecuted[edge.from]
				hasResult := nodeResults[edge.from] != nil
				mu.RUnlock()

				if fromFailed {
					dependencyFailed = true
					break
				}

				if fromExecuted && !hasResult {
					dependencySkipped = true
					break
				}

				if !hasResult {
					allDependenciesMet = false
					break
				}

				mu.RLock()
				results := nodeResults[edge.from]
				mu.RUnlock()

				if g.evaluateCondition(edge.cond, results) {
					hasValidEdge = true
					validInputs = append(validInputs, results...)
				}
			}

			if dependencyFailed {
				mu.Lock()
				nodeFailed[stepItem.nodeName] = true
				mu.Unlock()
				continue
			}

			if dependencySkipped {
				mu.Lock()
				nodeExecuted[stepItem.nodeName] = true
				mu.Unlock()
				continue
			}

			if allDependenciesMet && hasValidEdge {
				canExecute = true
				inputs = validInputs
			} else if allDependenciesMet && !hasValidEdge {
				mu.Lock()
				nodeExecuted[stepItem.nodeName] = true
				mu.Unlock()
				continue
			}
		}

		if !canExecute {
			if stepItem.retryCount%10 == 0 {
				time.Sleep(1 * time.Millisecond)
			}
			stepItem.retryCount++
			queue = append(queue, stepItem)
			continue
		}

		mu.Lock()
		nodeRunning[stepItem.nodeName] = true
		mu.Unlock()
		wg.Add(1)
		go func(nodeName string, inputs []any) {
			defer wg.Done()
			defer func() {
				mu.Lock()
				nodeRunning[nodeName] = false
				mu.Unlock()
			}()

			results, err := g.executeNode(nodeName, inputs)
			if err != nil {
				mu.Lock()
				nodeFailed[nodeName] = true
				mu.Unlock()
				errChan <- &ChainError{Message: fmt.Sprintf("node %s failed: %v", nodeName, err)}
				return
			}

			mu.Lock()
			nodeExecuted[nodeName] = true
			nodeResults[nodeName] = results
			g.stepNames[nodeName] = len(g.stepNames)
			mu.Unlock()
		}(stepItem.nodeName, inputs)
	}

	wg.Wait()
	close(errChan)

	if len(errChan) > 0 {
		if err := <-errChan; err != nil {
			return err
		}
	}

	return nil
}

func (g *Graph) Run() error {
	return g.RunSequential()
}

func (g *Graph) RunSequential() error {
	if g.err != nil {
		return g.err
	}

	return g.executeGraphSequential()
}

func (g *Graph) RunParallel() error {
	if g.err != nil {
		return g.err
	}

	return g.executeGraphParallel()
}

func (g *Graph) RunParallelWithContext(ctx context.Context) error {
	if g.err != nil {
		return g.err
	}

	return g.executeGraphParallelWithContext(ctx)
}

func (g *Graph) RunWithStrategy(strategy func() error) error {
	if g.err != nil {
		return g.err
	}

	return strategy()
}

func (g *Graph) NodeStatus(name string) NodeStatus {
	g.mu.Lock()
	defer g.mu.Unlock()

	if node, exists := g.nodes[name]; exists {
		return node.status
	}

	return NodeStatusPending
}

func (g *Graph) NodeResult(name string) []any {
	g.mu.Lock()
	defer g.mu.Unlock()

	if node, exists := g.nodes[name]; exists {
		return node.result
	}

	return nil
}

func (g *Graph) NodeError(name string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if node, exists := g.nodes[name]; exists {
		return node.err
	}

	return nil
}

func (g *Graph) Error() error {
	return g.err
}

func (g *Graph) ClearStatus() *Graph {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, node := range g.nodes {
		node.status = NodeStatusPending
		node.err = nil
		node.result = nil
	}

	g.err = nil
	return g
}

func (g *Graph) String() string {
	var sb strings.Builder

	sb.WriteString("digraph Graph {\n")
	sb.WriteString("    rankdir=TD;\n\n")

	for name, node := range g.nodes {
		style := ""
		switch node.nodeType {
		case NodeTypeStart:
			style = "shape=circle,fillcolor=green,style=filled"
		case NodeTypeEnd:
			style = "shape=doublecircle,fillcolor=red,style=filled"
		case NodeTypeBranch:
			style = "shape=diamond,fillcolor=yellow,style=filled"
		case NodeTypeParallel:
			style = "shape=box,fillcolor=cyan,style=filled"
		case NodeTypeLoop:
			style = "shape=oval,fillcolor=orange,style=filled"
		default:
			style = "shape=box"
		}
		fmt.Fprintf(&sb, "    %q [%s,label=%q];\n", name, style, name)
	}

	sb.WriteString("\n")

	for _, edges := range g.edges {
		for _, edge := range edges {
			label := ""
			if edge.cond != nil {
				label = fmt.Sprintf(",label=%q", "cond")
			}
			fmt.Fprintf(&sb, "    %q -> %q [%s];\n", edge.from, edge.to, label)
		}
	}

	sb.WriteString("}\n")

	return sb.String()
}

func (g *Graph) Mermaid() string {
	var sb strings.Builder

	sb.WriteString("graph TD\n\n")

	for _, edges := range g.edges {
		for _, edge := range edges {
			label := ""
			if edge.cond != nil {
				label = "|cond|"
			}
			fmt.Fprintf(&sb, "    %s --> %s%s\n", edge.from, label, edge.to)
		}
	}

	for name := range g.nodes {
		if _, hasEdges := g.edges[name]; !hasEdges {
			if g.inDegree[name] == 0 {
				fmt.Fprintf(&sb, "    %s\n", name)
			}
		}
	}

	return sb.String()
}
