package flow

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

const (
	ErrNodeNotFound     = "node not found"
	ErrDuplicateNode    = "duplicate node name"
	ErrSelfDependency   = "node cannot depend on itself"
	ErrCyclicDependency = "cyclic dependency detected"
	ErrNoStartNode      = "no start node found"
	ErrExecutionFailed  = "execution failed"
)

const (
	DefaultMaxIterations = 10000
)

type NodeStatus int

const (
	NodeStatusPending NodeStatus = iota
	NodeStatusRunning
	NodeStatusCompleted
	NodeStatusFailed
)

type EdgeType int

const (
	EdgeTypeNormal EdgeType = iota
	EdgeTypeLoop
	EdgeTypeBranch
)

type CondFunc func([]any) bool

type Edge struct {
	from     string
	to       string
	cond     any
	condFunc CondFunc
	condComp *condCompiler
	weight   int
	edgeType EdgeType
}

type Node struct {
	name           string
	status         NodeStatus
	fn             any
	fnValue        reflect.Value
	fnType         reflect.Type
	argTypes       []reflect.Type
	numOut         int
	hasErrorReturn bool
	description    string
	inputs         []string
	outputs        []string
	err            error
	result         []any
	callFn         func([]any) ([]any, error)
	argCount       int
	sliceArg       bool
	sliceElemType  reflect.Type
	mu             sync.RWMutex
}

type Graph struct {
	nodes             map[string]*Node
	edges             map[string][]*Edge
	inDegree          map[string]int
	outDegree         map[string]int
	stepNames         map[string]int
	err               error
	mu                sync.RWMutex
	execPlan          []string
	execPlanValid     bool
	execInEdges       map[string][]*Edge
	branchTargetNodes map[string]bool
	tempInDegree      map[string]int
	visited           map[string]bool
	path              map[string]bool
	execStates        map[string]*nodeState
	layers            [][]string
	layersValid       bool
	largeThreshold    int
	pauseConfig       *PauseConfig
	pauseSignal       PauseSignal
	resourceChecker   ResourceChecker
	pausedAtNode      string
}

const (
	largeGraphThreshold        = 128
	defaultCapacity            = 32
	defaultLayerBoundsCapacity = 16
)

type GraphOption func(*Graph)

func WithCapacity(capacity int) GraphOption {
	return func(g *Graph) {
		g.nodes = make(map[string]*Node, capacity)
		g.edges = make(map[string][]*Edge, capacity)
		g.inDegree = make(map[string]int, capacity)
		g.outDegree = make(map[string]int, capacity)
		g.stepNames = make(map[string]int, capacity)
	}
}

func WithLargeGraphThreshold(threshold int) GraphOption {
	return func(g *Graph) {
		if threshold > 0 {
			g.largeThreshold = threshold
		}
	}
}

func NewGraph(opts ...GraphOption) *Graph {
	g := &Graph{}
	for _, opt := range opts {
		opt(g)
	}
	if g.nodes == nil {
		g.nodes = make(map[string]*Node, defaultCapacity)
		g.edges = make(map[string][]*Edge, defaultCapacity)
		g.inDegree = make(map[string]int, defaultCapacity)
		g.outDegree = make(map[string]int, defaultCapacity)
		g.stepNames = make(map[string]int, defaultCapacity)
	}
	return g
}

func (g *Graph) AddNode(name string, fn any) *Graph {
	if g.err != nil {
		return g
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[name]; exists {
		g.err = &FlowError{Message: ErrDuplicateNode}
		return g
	}

	g.execPlanValid = false

	node := nodePool.Get()
	*node = Node{
		name:   name,
		status: NodeStatusPending,
		fn:     fn,
	}

	if fn != nil {
		node.fnValue = reflect.ValueOf(fn)
		node.fnType = node.fnValue.Type()
		if node.fnType.Kind() != reflect.Func {
			g.err = &FlowError{Message: ErrNotFunction}
			return g
		}
		numIn := node.fnType.NumIn()
		node.argCount = numIn
		node.argTypes = make([]reflect.Type, numIn)
		for i := range numIn {
			node.argTypes[i] = node.fnType.In(i)
		}
		if numIn == 1 && node.argTypes[0].Kind() == reflect.Slice {
			node.sliceArg = true
			node.sliceElemType = node.argTypes[0].Elem()
		}
		node.numOut = node.fnType.NumOut()
		if node.numOut > 0 {
			lastOutType := node.fnType.Out(node.numOut - 1)
			node.hasErrorReturn = lastOutType.Implements(errorType)
		}
		node.callFn = g.compileNodeCall(node)
	}

	g.nodes[name] = node
	g.inDegree[name] = 0
	g.outDegree[name] = 0

	return g
}

type EdgeOption func(*Edge)

func WithEdgeType(t EdgeType) EdgeOption {
	return func(e *Edge) {
		e.edgeType = t
	}
}

func WithCondition(cond any) EdgeOption {
	return func(e *Edge) {
		e.cond = cond
	}
}

func WithMaxIterations(max int) EdgeOption {
	return func(e *Edge) {
		e.weight = max
	}
}

func (g *Graph) AddEdge(from, to string, opts ...EdgeOption) *Graph {
	if g.err != nil {
		return g
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[from]; !exists {
		g.err = &FlowError{Message: fmt.Sprintf("%s: %s", ErrNodeNotFound, from)}
		return g
	}

	if _, exists := g.nodes[to]; !exists {
		g.err = &FlowError{Message: fmt.Sprintf("%s: %s", ErrNodeNotFound, to)}
		return g
	}

	edge := edgePool.Get()
	*edge = Edge{
		from:     from,
		to:       to,
		edgeType: EdgeTypeNormal,
	}

	for _, opt := range opts {
		opt(edge)
	}

	if edge.cond != nil {
		edge.condFunc = g.compileCondition(edge.cond)
	}

	switch edge.edgeType {
	case EdgeTypeLoop:
		if from != to {
			g.err = &FlowError{Message: "loop edge must have same from and to node"}
			return g
		}
		if edge.weight <= 0 {
			edge.weight = DefaultMaxIterations
		}
	case EdgeTypeNormal, EdgeTypeBranch:
		if from == to {
			g.err = &FlowError{Message: ErrSelfDependency}
			return g
		}
		if g.HasCycle(from, to) {
			g.err = &FlowError{Message: ErrCyclicDependency}
			return g
		}
	}

	g.edges[from] = append(g.edges[from], edge)
	if edge.edgeType == EdgeTypeNormal || edge.edgeType == EdgeTypeBranch {
		g.inDegree[to]++
		g.outDegree[from]++
	}
	g.execPlanValid = false

	return g
}

func (g *Graph) AddEdgeWithCondition(from, to string, cond any) *Graph {
	return g.AddEdge(from, to, WithCondition(cond))
}

func (g *Graph) AddLoopEdge(nodeName string, cond any, maxIterations ...int) *Graph {
	opts := []EdgeOption{WithEdgeType(EdgeTypeLoop), WithCondition(cond)}
	if len(maxIterations) > 0 && maxIterations[0] > 0 {
		opts = append(opts, WithMaxIterations(maxIterations[0]))
	}
	return g.AddEdge(nodeName, nodeName, opts...)
}

func (g *Graph) AddBranchEdge(from string, branches map[string]any) *Graph {
	for to, cond := range branches {
		g.AddEdge(from, to, WithEdgeType(EdgeTypeBranch), WithCondition(cond))
		if g.err != nil {
			return g
		}
	}
	return g
}

func (g *Graph) HasCycle(from, to string) bool {
	if g.visited == nil {
		g.visited = make(map[string]bool, len(g.nodes))
	} else {
		clear(g.visited)
	}
	visited := g.visited

	if g.path == nil {
		g.path = make(map[string]bool, len(g.nodes))
	} else {
		clear(g.path)
	}
	path := g.path

	stack := []string{to}
	index := 0

	for index >= 0 {
		node := stack[index]

		if path[node] {
			return true
		}

		if visited[node] {
			index--
			continue
		}

		path[node] = true
		visited[node] = true

		hasUnvisited := false
		for _, edge := range g.edges[node] {
			if edge.edgeType == EdgeTypeLoop {
				continue
			}
			nextNode := edge.to
			if nextNode == from {
				return true
			}
			if !visited[nextNode] {
				stack = append(stack, nextNode)
				index++
				hasUnvisited = true
				break
			}
		}

		if !hasUnvisited {
			path[node] = false
			index--
		}
	}

	return false
}

func (g *Graph) executeNodeWithLoop(
	nodeName string,
	inputs []any,
) ([]any, error) {
	results, err := g.executeNode(nodeName, inputs)
	if err != nil {
		return nil, err
	}

	for _, edge := range g.edges[nodeName] {
		if edge.from == nodeName && edge.to == nodeName {
			maxIter := edge.weight
			if maxIter <= 0 {
				maxIter = DefaultMaxIterations
			}
			for i := 1; i < maxIter; i++ {
				if edge.condFunc != nil && !edge.condFunc(results) {
					break
				}
				results, err = g.executeNode(nodeName, results)
				if err != nil {
					return nil, err
				}
			}
			break
		}
	}

	return results, nil
}

type nodeState struct {
	results  []any
	err      error
	done     uint32
	finished uint32
	doneSig  chan struct{}
}

type execContext struct {
	graph             *Graph
	ctx               context.Context
	plan              []string
	states            map[string]*nodeState
	incomingEdges     map[string][]*Edge
	branchTargetNodes map[string]bool
	errChan           chan error
	doneChan          chan struct{}
}

type nodeTask struct {
	ctx  *execContext
	name string
}

func (g *Graph) SetPauseConfig(config *PauseConfig) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.pauseConfig = config
}

func (g *Graph) SetPauseSignal(signal PauseSignal) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.pauseSignal = signal
}

func (g *Graph) SetResourceChecker(checker ResourceChecker) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.resourceChecker = checker
}

func (g *Graph) GetPausedAtNode() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.pausedAtNode
}

func (g *Graph) shouldPauseAtNode(nodeName string) bool {
	if g.pauseConfig != nil && g.pauseConfig.ShouldPauseAtNode(nodeName) {
		return true
	}
	return false
}

func (g *Graph) shouldPauseForSignal() bool {
	if g.pauseSignal != nil {
		return g.pauseSignal.ShouldPause()
	}
	return false
}

func (g *Graph) checkResourceAvailable(nodeName string) bool {
	if g.resourceChecker != nil {
		return g.resourceChecker.CheckAvailable(nodeName)
	}
	return true
}

func (g *Graph) Run() error {
	if g.err != nil {
		return g.err
	}
	return g.RunWithContext(context.Background())
}

func (g *Graph) RunWithContext(ctx context.Context) error {
	if g.err != nil {
		return g.err
	}

	return g.executeGraphParallelWithContext(ctx)
}

func (g *Graph) RunSequential() error {
	if g.err != nil {
		return g.err
	}
	return g.RunSequentialWithContext(context.Background())
}

func (g *Graph) RunSequentialWithContext(ctx context.Context) error {
	if g.err != nil {
		return g.err
	}

	plan, err := g.buildExecutionPlan()
	if err != nil {
		return err
	}

	g.buildExecInEdges()

	return g.executeSequential(ctx, plan)
}

func (g *Graph) buildExecInEdges() {
	allEdges := g.edges
	if g.execInEdges == nil {
		g.execInEdges = make(map[string][]*Edge, len(allEdges))
	} else {
		clear(g.execInEdges)
	}
	for _, edges := range allEdges {
		for _, edge := range edges {
			g.execInEdges[edge.to] = append(g.execInEdges[edge.to], edge)
		}
	}
}

func (g *Graph) executeSequential(ctx context.Context, plan []string) error {
	resultsMap := make(map[string][]any, len(plan))

	for _, name := range plan {
		select {
		case <-ctx.Done():
			return &FlowError{Message: fmt.Sprintf("execution canceled: %v", ctx.Err())}
		default:
		}

		if g.shouldPauseForSignal() {
			g.mu.Lock()
			g.pausedAtNode = name
			g.mu.Unlock()
			return ErrFlowPaused
		}

		if g.shouldPauseAtNode(name) {
			g.mu.Lock()
			g.pausedAtNode = name
			g.mu.Unlock()
			return ErrFlowPaused
		}

		if !g.checkResourceAvailable(name) {
			g.mu.Lock()
			g.pausedAtNode = name
			g.mu.Unlock()
			return ErrResourceNotAvailable
		}

		node := g.nodes[name]
		if node == nil {
			return &FlowError{Message: ErrNodeNotFound}
		}

		node.mu.RLock()
		isCompleted := node.status == NodeStatusCompleted
		var existingResult []any
		if isCompleted && len(node.result) > 0 {
			existingResult = make([]any, len(node.result))
			copy(existingResult, node.result)
		}
		node.mu.RUnlock()

		if isCompleted {
			resultsMap[name] = g.convertNodeResultsForInput(node, existingResult)
			continue
		}

		inEdges := g.execInEdges[name]
		var inputs []any

		if len(inEdges) == 0 {
			inputs = nil
		} else {
			for _, edge := range inEdges {
				if edge.edgeType == EdgeTypeLoop {
					continue
				}
				if fromResults, ok := resultsMap[edge.from]; ok {
					inputs = append(inputs, fromResults...)
				}
			}
		}

		results, err := g.executeNodeWithLoop(name, inputs)
		if err != nil {
			if g.pauseConfig != nil && g.pauseConfig.OnErrorPause {
				g.mu.Lock()
				g.pausedAtNode = name
				g.mu.Unlock()
			}
			return &FlowError{Message: fmt.Sprintf("node %s failed: %v", name, err)}
		}

		resultsMap[name] = results
		g.mu.Lock()
		g.stepNames[name] = len(g.stepNames)
		g.mu.Unlock()
	}

	return nil
}

func (g *Graph) buildExecutionPlan() ([]string, error) {
	if g.execPlanValid && len(g.execPlan) > 0 {
		return g.execPlan, nil
	}

	nodeCount := len(g.nodes)

	if g.tempInDegree == nil {
		g.tempInDegree = make(map[string]int, nodeCount)
	} else {
		clear(g.tempInDegree)
	}
	tempInDegree := g.tempInDegree
	for name := range g.nodes {
		tempInDegree[name] = 0
	}
	for _, edges := range g.edges {
		for _, edge := range edges {
			if edge.edgeType == EdgeTypeNormal || edge.edgeType == EdgeTypeBranch {
				tempInDegree[edge.to]++
			}
		}
	}

	if g.visited == nil {
		g.visited = make(map[string]bool, nodeCount)
	} else {
		clear(g.visited)
	}
	visited := g.visited

	plan := stringSlicePool.Get(nodeCount)

	startNode := g.findStartNode()
	if startNode == "" {
		stringSlicePool.Put(plan)
		return nil, &FlowError{Message: ErrNoStartNode}
	}

	queue := stringSlicePool.Get(nodeCount)
	for name, degree := range tempInDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	if len(queue) == 0 {
		queue = append(queue, startNode)
	}

	head := 0
	for head < len(queue) {
		current := queue[head]
		head++

		if visited[current] {
			continue
		}

		plan = append(plan, current)
		visited[current] = true

		for _, edge := range g.edges[current] {
			if edge.edgeType == EdgeTypeLoop {
				continue
			}
			nextNode := edge.to
			tempInDegree[nextNode]--
			if tempInDegree[nextNode] == 0 {
				queue = append(queue, nextNode)
			}
		}
	}

	stringSlicePool.Put(queue)

	if len(plan) != nodeCount {
		stringSlicePool.Put(plan)
		return nil, &FlowError{Message: ErrCyclicDependency}
	}

	g.execPlan = append(g.execPlan[:0], plan...)
	g.execPlanValid = true

	if g.branchTargetNodes == nil {
		g.branchTargetNodes = make(map[string]bool, nodeCount)
	} else {
		clear(g.branchTargetNodes)
	}
	for _, edges := range g.edges {
		for _, e := range edges {
			if e.edgeType == EdgeTypeBranch {
				g.branchTargetNodes[e.to] = true
			}
		}
	}

	stringSlicePool.Put(plan)

	return g.execPlan, nil
}

func (g *Graph) findStartNode() string {
	for name := range g.nodes {
		if g.inDegree[name] == 0 {
			return name
		}
	}

	return ""
}

func (g *Graph) buildLayers() ([][]string, error) {
	if g.layersValid && len(g.layers) > 0 {
		return g.layers, nil
	}

	nodeCount := len(g.nodes)

	if g.tempInDegree == nil {
		g.tempInDegree = make(map[string]int, nodeCount)
	} else {
		clear(g.tempInDegree)
	}
	tempInDegree := g.tempInDegree
	for name := range g.nodes {
		tempInDegree[name] = 0
	}
	for _, edges := range g.edges {
		for _, edge := range edges {
			if edge.edgeType == EdgeTypeNormal || edge.edgeType == EdgeTypeBranch {
				tempInDegree[edge.to]++
			}
		}
	}

	if g.visited == nil {
		g.visited = make(map[string]bool, nodeCount)
	} else {
		clear(g.visited)
	}
	visited := g.visited

	allNodes := stringSlicePool.Get(nodeCount)
	allNodes = allNodes[:0]

	layerBounds := make([]int, 0, defaultLayerBoundsCapacity)
	layerBounds = append(layerBounds, 0)

	for name, degree := range tempInDegree {
		if degree == 0 {
			allNodes = append(allNodes, name)
		}
	}

	if len(allNodes) == 0 {
		startNode := g.findStartNode()
		if startNode == "" {
			stringSlicePool.Put(allNodes)
			return nil, &FlowError{Message: ErrNoStartNode}
		}
		allNodes = append(allNodes, startNode)
	}

	layerStart := 0
	layerEnd := len(allNodes)
	totalProcessed := 0

	for layerStart < layerEnd {
		for i := layerStart; i < layerEnd; i++ {
			node := allNodes[i]
			visited[node] = true
			for _, edge := range g.edges[node] {
				if edge.edgeType == EdgeTypeLoop {
					continue
				}
				nextNode := edge.to
				tempInDegree[nextNode]--
				if tempInDegree[nextNode] == 0 && !visited[nextNode] {
					allNodes = append(allNodes, nextNode)
				}
			}
		}

		totalProcessed += layerEnd - layerStart
		layerBounds = append(layerBounds, len(allNodes))
		layerStart = layerEnd
		layerEnd = len(allNodes)
	}

	if totalProcessed != nodeCount {
		stringSlicePool.Put(allNodes)
		return nil, &FlowError{Message: ErrCyclicDependency}
	}

	layerCount := len(layerBounds) - 1
	if g.layers == nil {
		g.layers = make([][]string, 0, layerCount)
	} else {
		for _, layer := range g.layers {
			stringSlicePool.Put(layer)
		}
		g.layers = g.layers[:0]
	}

	for i := range layerCount {
		start := layerBounds[i]
		end := layerBounds[i+1]
		layerSize := end - start
		layer := stringSlicePool.Get(layerSize)
		layer = layer[:0]
		layer = append(layer, allNodes[start:end]...)
		g.layers = append(g.layers, layer)
	}

	stringSlicePool.Put(allNodes)
	g.layersValid = true

	return g.layers, nil
}

func (g *Graph) convertNodeResultsForInput(node *Node, results []any) []any {
	if node == nil || len(results) == 0 {
		return results
	}

	var converted []any
	for _, result := range results {
		if result == nil {
			converted = append(converted, nil)
			continue
		}

		resultVal := reflect.ValueOf(result)
		resultType := resultVal.Type()

		switch resultType.Kind() { //nolint:exhaustive
		case reflect.Float64:
			fv := resultVal.Float()
			if fv == float64(int64(fv)) {
				converted = append(converted, int(fv))
			} else {
				converted = append(converted, result)
			}
		case reflect.Interface:
			converted = append(converted, g.convertNodeResultsForInput(node, []any{resultVal.Elem().Interface()})...)
		case reflect.Slice:
			sliceLen := resultVal.Len()
			sliceContents := make([]any, sliceLen)
			for j := 0; j < sliceLen; j++ {
				sliceContents[j] = resultVal.Index(j).Interface()
			}
			converted = append(converted, g.convertNodeResultsForInput(node, sliceContents)...)
		default:
			converted = append(converted, result)
		}
	}

	return converted
}

func (g *Graph) executeNode(nodeName string, inputs []any) ([]any, error) {
	node := g.nodes[nodeName]
	if node == nil {
		return nil, &FlowError{Message: ErrNodeNotFound}
	}

	node.mu.Lock()
	node.status = NodeStatusRunning
	node.err = nil
	node.mu.Unlock()

	if node.callFn != nil {
		results, err := node.callFn(inputs)
		node.mu.Lock()
		if err != nil {
			node.err = err
			node.status = NodeStatusFailed
			node.mu.Unlock()
			return nil, err
		}
		node.result = results
		node.status = NodeStatusCompleted
		node.mu.Unlock()
		return results, nil
	}

	node.mu.Lock()
	node.status = NodeStatusCompleted
	node.mu.Unlock()
	return inputs, nil
}

func (g *Graph) Error() error {
	return g.err
}

func (g *Graph) ClearStatus() *Graph {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, node := range g.nodes {
		node.mu.Lock()
		node.status = NodeStatusPending
		node.err = nil
		node.result = nil
		node.mu.Unlock()
	}

	for _, edges := range g.edges {
		for _, edge := range edges {
			if edge.condComp != nil {
				condCompilerPool.Put(edge.condComp)
				edge.condComp = nil
			}
		}
	}

	g.err = nil
	return g
}

func (g *Graph) NodeStatus(nodeName string) (NodeStatus, error) {
	g.mu.RLock()
	node, ok := g.nodes[nodeName]
	g.mu.RUnlock()
	if !ok {
		return NodeStatusPending, &FlowError{Message: ErrNodeNotFound}
	}

	node.mu.RLock()
	status := node.status
	node.mu.RUnlock()
	return status, nil
}

func (g *Graph) NodeResult(nodeName string) ([]any, error) {
	g.mu.RLock()
	node, ok := g.nodes[nodeName]
	g.mu.RUnlock()
	if !ok {
		return nil, &FlowError{Message: ErrNodeNotFound}
	}

	node.mu.RLock()
	defer node.mu.RUnlock()
	if len(node.result) == 0 {
		return nil, nil
	}

	result := make([]any, len(node.result))
	copy(result, node.result)
	return result, nil
}

func (g *Graph) NodeError(nodeName string) error {
	g.mu.RLock()
	node, ok := g.nodes[nodeName]
	g.mu.RUnlock()
	if !ok {
		return &FlowError{Message: ErrNodeNotFound}
	}

	node.mu.RLock()
	err := node.err
	node.mu.RUnlock()
	return err
}

func (g *Graph) String() string {
	var sb strings.Builder

	sb.WriteString("digraph Graph {\n")
	sb.WriteString("    rankdir=TD;\n\n")

	for name := range g.nodes {
		fmt.Fprintf(&sb, "    %q [shape=box,label=%q];\n", name, name)
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
