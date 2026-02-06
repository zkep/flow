package flow

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
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

var (
	anySlicePool          = NewSlicePool[any](128, 32)
	stringSlicePool       = NewSlicePool[string](128, 32)
	reflectValueSlicePool = NewSlicePool[reflect.Value](128, 32)

	nodePool = NewObjectPool(
		func() *Node { return &Node{} },
		WithReset(func(n *Node) {
			n.name = ""
			n.status = NodeStatusPending
			n.fn = nil
			n.fnValue = reflect.Value{}
			n.fnType = nil
			n.argTypes = nil
			n.numOut = 0
			n.hasErrorReturn = false
			n.description = ""
			n.inputs = nil
			n.outputs = nil
			n.err = nil
			n.result = nil
			n.callFn = nil
			n.argCount = 0
			n.sliceArg = false
			n.sliceElemType = nil
		}),
	)
	edgePool = NewObjectPool(
		func() *Edge { return &Edge{} },
		WithReset(func(e *Edge) {
			e.from = ""
			e.to = ""
			e.cond = nil
			e.condFunc = nil
			e.condComp = nil
			e.weight = 0
			e.edgeType = EdgeTypeNormal
		}),
	)
	nodeStatePool = NewObjectPool(
		func() *nodeState { return &nodeState{} },
		WithReset(func(s *nodeState) {
			s.results = nil
			s.err = nil
			s.done = 0
			s.finished = 0
			s.doneSig = nil
		}),
	)
	condCompilerPool = NewObjectPool(
		func() *condCompiler { return &condCompiler{} },
		WithReset(func(c *condCompiler) {
			c.fnValue = reflect.Value{}
			c.fnType = nil
			c.argCount = 0
			c.isVariadic = false
		}),
	)
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

type condCompiler struct {
	fnValue    reflect.Value
	fnType     reflect.Type
	argCount   int
	isVariadic bool
}

func newCondCompiler(cond any) *condCompiler {
	c := condCompilerPool.Get()
	c.fnValue = reflect.ValueOf(cond)
	c.fnType = c.fnValue.Type()
	c.argCount = c.fnType.NumIn()
	c.isVariadic = c.fnType.IsVariadic()
	return c
}

func (c *condCompiler) eval(results []any) bool {
	args := argsPool.Get(c.argCount)
	defer argsPool.Put(args)

	if c.isVariadic && len(results) > 0 {
		sliceType := c.fnType.In(c.argCount - 1).Elem()
		slice := reflect.MakeSlice(reflect.SliceOf(sliceType), 0, len(results))
		for _, result := range results {
			slice = reflect.Append(slice, reflect.ValueOf(result))
		}
		args = append(args, slice)
	} else if c.argCount > 0 {
		resultCount := min(len(results), c.argCount)
		for i := range resultCount {
			args = append(args, reflect.ValueOf(results[i]))
		}
		for i := resultCount; i < c.argCount; i++ {
			args = append(args, reflect.Zero(c.fnType.In(i)))
		}
	}

	var condResult []reflect.Value
	if c.isVariadic {
		condResult = c.fnValue.CallSlice(args)
	} else {
		condResult = c.fnValue.Call(args)
	}

	if len(condResult) > 0 {
		if condResult[0].Kind() == reflect.Bool {
			return condResult[0].Bool()
		}
		if condResult[0].Kind() == reflect.Interface && !condResult[0].IsNil() {
			if b, ok := condResult[0].Elem().Interface().(bool); ok {
				return b
			}
		}
	}
	return true
}

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
	waitGroup      sync.WaitGroup
	callFn         func([]any) ([]any, error)
	argCount       int
	sliceArg       bool
	sliceElemType  reflect.Type
}

type NodeExecution struct {
	node    *Node
	results []any
	err     error
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
	execResults       map[string][]any
	execInEdges       map[string][]*Edge
	branchTargetNodes map[string]bool
	tempInDegree      map[string]int
	visited           map[string]bool
	path              map[string]bool
	execStates        map[string]*nodeState
	layers            [][]string
	layersValid       bool
	largeThreshold    int
}

const largeGraphThreshold = 128

type localWorkerPool struct {
	workers  int
	taskChan chan *nodeTask
	wg       sync.WaitGroup
}

var localWorkerPoolPool = sync.Pool{
	New: func() any {
		return &localWorkerPool{}
	},
}

func newLocalWorkerPool(workers int) *localWorkerPool {
	if workers <= 0 {
		workers = defaultWorkerCount
	}
	pool := localWorkerPoolPool.Get().(*localWorkerPool)
	pool.workers = workers
	if pool.taskChan == nil || cap(pool.taskChan) < workers*4 {
		pool.taskChan = make(chan *nodeTask, workers*4)
	} else {
		for len(pool.taskChan) > 0 {
			<-pool.taskChan
		}
	}
	for i := 0; i < workers; i++ {
		pool.wg.Add(1)
		go pool.worker()
	}
	return pool
}

func (p *localWorkerPool) worker() {
	defer p.wg.Done()
	for task := range p.taskChan {
		if task == nil {
			return
		}
		executeNodeWorkerTask(task)
		taskPool.Put(task)
	}
}

func (p *localWorkerPool) Submit(task *nodeTask) {
	p.taskChan <- task
}

func (p *localWorkerPool) Shutdown() {
	for i := 0; i < p.workers; i++ {
		p.taskChan <- nil
	}
	p.wg.Wait()
	localWorkerPoolPool.Put(p)
}

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
	g := &Graph{
		nodes:     make(map[string]*Node, 16),
		edges:     make(map[string][]*Edge, 16),
		inDegree:  make(map[string]int, 16),
		outDegree: make(map[string]int, 16),
		stepNames: make(map[string]int, 16),
	}
	for _, opt := range opts {
		opt(g)
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
		g.err = &ChainError{Message: ErrDuplicateNode}
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
			g.err = &ChainError{Message: ErrNotFunction}
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
		g.err = &ChainError{Message: fmt.Sprintf("%s: %s", ErrNodeNotFound, from)}
		return g
	}

	if _, exists := g.nodes[to]; !exists {
		g.err = &ChainError{Message: fmt.Sprintf("%s: %s", ErrNodeNotFound, to)}
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
			g.err = &ChainError{Message: "loop edge must have same from and to node"}
			return g
		}
		if edge.weight <= 0 {
			edge.weight = DefaultMaxIterations
		}
	case EdgeTypeNormal:
		if from == to {
			g.err = &ChainError{Message: ErrSelfDependency}
			return g
		}
		if g.HasCycle(from, to) {
			g.err = &ChainError{Message: ErrCyclicDependency}
			return g
		}
	default:
		if from == to {
			g.err = &ChainError{Message: ErrSelfDependency}
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

func (g *Graph) compileCondition(cond any) CondFunc {
	if cond == nil {
		return nil
	}

	if c, ok := cond.(CondFunc); ok {
		return c
	}

	if b, ok := cond.(bool); ok {
		if b {
			return nil
		}
		return func([]any) bool { return false }
	}

	fnValue := reflect.ValueOf(cond)
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func {
		return nil
	}

	comp := newCondCompiler(cond)
	return comp.eval
}

func (g *Graph) compileNodeCall(node *Node) func([]any) ([]any, error) {
	if node.fn == nil {
		return func(inputs []any) ([]any, error) {
			return inputs, nil
		}
	}

	fnValue := node.fnValue
	argCount := node.argCount
	sliceArg := node.sliceArg
	sliceElemType := node.sliceElemType
	hasError := node.hasErrorReturn
	argTypes := node.argTypes

	return func(inputs []any) ([]any, error) {
		args := reflectValueSlicePool.Get(argCount)
		defer reflectValueSlicePool.Put(args)

		if len(inputs) > 0 {
			if argCount > 0 && len(inputs) == argCount {
				for i := range len(inputs) {
					input := inputs[i]
					if input == nil {
						args = append(args, reflect.Zero(argTypes[i]))
						continue
					}
					val := reflect.ValueOf(input)
					if !val.Type().AssignableTo(argTypes[i]) {
						if val.CanConvert(argTypes[i]) {
							val = val.Convert(argTypes[i])
						} else {
							return nil, &ChainError{Message: ErrArgTypeMismatch}
						}
					}
					args = append(args, val)
				}
			} else if sliceArg {
				sliceValue := reflect.MakeSlice(argTypes[0], len(inputs), len(inputs))
				for i := range inputs {
					val := reflect.ValueOf(inputs[i])
					if !val.Type().AssignableTo(sliceElemType) {
						if val.CanConvert(sliceElemType) {
							val = val.Convert(sliceElemType)
						} else {
							return nil, &ChainError{Message: ErrArgTypeMismatch}
						}
					}
					sliceValue.Index(i).Set(val)
				}
				args = append(args, sliceValue)
			} else if len(inputs) > 0 {
				currentValue := inputs[0]
				currentValueType := reflect.TypeOf(currentValue)
				currentValueValue := reflect.ValueOf(currentValue)

				if currentValueType == nil {
					if argCount > 0 {
						args = append(args, reflect.Zero(argTypes[0]))
					}
				} else if currentValueType.Kind() == reflect.Slice || currentValueType.Kind() == reflect.Array {
					elemCount := currentValueValue.Len()
					if argCount > 0 && elemCount != argCount {
						return nil, &ChainError{Message: ErrArgCountMismatch}
					}
					for i := range elemCount {
						elem := currentValueValue.Index(i)
						if elem.Kind() == reflect.Interface {
							elem = elem.Elem()
						}
						args = append(args, elem)
					}
				} else {
					if argCount > 0 {
						val := currentValueValue
						if !val.Type().AssignableTo(argTypes[0]) {
							if val.CanConvert(argTypes[0]) {
								val = val.Convert(argTypes[0])
							} else {
								return nil, &ChainError{Message: ErrArgTypeMismatch}
							}
						}
						args = append(args, val)
					}
				}
			}
		}

		if len(args) != argCount {
			return nil, &ChainError{Message: ErrArgCountMismatch}
		}

		results := fnValue.Call(args)

		if hasError {
			errValue := results[len(results)-1]
			if !errValue.IsNil() {
				return nil, errValue.Interface().(error)
			}
			results = results[:len(results)-1]
		}

		out := make([]any, len(results))
		for i, r := range results {
			out[i] = r.Interface()
		}
		return out, nil
	}
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
	wg                sync.WaitGroup
}

type nodeTask struct {
	ctx  *execContext
	name string
}

var taskPool = sync.Pool{
	New: func() any { return &nodeTask{} },
}

const defaultWorkerCount = 8

type globalWorker struct {
	taskChan chan *nodeTask
	wg       sync.WaitGroup
}

var gw *globalWorker
var gwOnce sync.Once

func getGlobalWorker() *globalWorker {
	gwOnce.Do(func() {
		gw = &globalWorker{
			taskChan: make(chan *nodeTask, 1024),
		}
		for i := 0; i < defaultWorkerCount; i++ {
			gw.wg.Add(1)
			go gw.worker()
		}
	})
	return gw
}

func (w *globalWorker) worker() {
	defer w.wg.Done()
	for task := range w.taskChan {
		if task == nil {
			return
		}
		executeNodeWorkerTask(task)
		taskPool.Put(task)
	}
}

func (w *globalWorker) Submit(task *nodeTask) {
	w.taskChan <- task
}

func (g *Graph) executeGraphParallelWithContext(ctx context.Context) error {
	nodeCount := len(g.nodes)

	threshold := largeGraphThreshold
	if g.largeThreshold > 0 {
		threshold = g.largeThreshold
	}

	if nodeCount >= threshold {
		return g.executeGraphParallelLarge(ctx)
	}

	return g.executeGraphParallelSmall(ctx)
}

func (g *Graph) executeGraphParallelSmall(ctx context.Context) error {
	plan, err := g.buildExecutionPlan()
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return &ChainError{Message: fmt.Sprintf("execution canceled: %v", ctx.Err())}
	default:
	}

	allEdges := g.edges

	var incomingEdges map[string][]*Edge
	if g.execInEdges != nil && g.execPlanValid {
		incomingEdges = g.execInEdges
	} else {
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
		incomingEdges = g.execInEdges
	}

	if g.execStates == nil {
		g.execStates = make(map[string]*nodeState, len(plan))
	} else {
		clear(g.execStates)
	}
	states := g.execStates
	for _, name := range plan {
		state := nodeStatePool.Get()
		state.doneSig = make(chan struct{}, 1)
		states[name] = state
	}

	errChan := make(chan error, 1)
	doneChan := make(chan struct{}, len(plan))

	execCtx := &execContext{
		graph:             g,
		ctx:               ctx,
		plan:              plan,
		states:            states,
		incomingEdges:     incomingEdges,
		branchTargetNodes: g.branchTargetNodes,
		errChan:           errChan,
		doneChan:          doneChan,
	}

	worker := getGlobalWorker()

	go func() {
		for _, nodeName := range plan {
			task := taskPool.Get().(*nodeTask)
			task.ctx = execCtx
			task.name = nodeName
			worker.Submit(task)
		}
	}()

	var execErr error
	total := len(plan)
	completed := 0
	for completed < total {
		select {
		case <-ctx.Done():
			execErr = &ChainError{Message: fmt.Sprintf("execution canceled: %v", ctx.Err())}
			return execErr
		case err := <-errChan:
			execErr = err
			return execErr
		case <-doneChan:
			completed++
		}
	}

	for _, state := range states {
		nodeStatePool.Put(state)
	}

	return execErr
}

func (g *Graph) executeGraphParallelLarge(ctx context.Context) error {
	layers, err := g.buildLayers()
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return &ChainError{Message: fmt.Sprintf("execution canceled: %v", ctx.Err())}
	default:
	}

	allEdges := g.edges
	nodeCount := len(g.nodes)

	var incomingEdges map[string][]*Edge
	if g.execInEdges != nil && g.layersValid {
		incomingEdges = g.execInEdges
	} else {
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
		incomingEdges = g.execInEdges
	}

	if g.execStates == nil {
		g.execStates = make(map[string]*nodeState, nodeCount)
	} else {
		clear(g.execStates)
	}
	states := g.execStates
	for _, layer := range layers {
		for _, name := range layer {
			state := nodeStatePool.Get()
			state.doneSig = make(chan struct{}, 1)
			states[name] = state
		}
	}

	errChan := make(chan error, 1)
	layerDone := make(chan struct{}, nodeCount)

	execCtx := &execContext{
		graph:             g,
		ctx:               ctx,
		plan:              nil,
		states:            states,
		incomingEdges:     incomingEdges,
		branchTargetNodes: g.branchTargetNodes,
		errChan:           errChan,
		doneChan:          layerDone,
	}

	workerCount := defaultWorkerCount
	if nodeCount < workerCount {
		workerCount = nodeCount
	}
	pool := newLocalWorkerPool(workerCount)
	defer pool.Shutdown()

	var execErr error

	for _, layer := range layers {
		select {
		case <-ctx.Done():
			return &ChainError{Message: fmt.Sprintf("execution canceled: %v", ctx.Err())}
		case err := <-errChan:
			execErr = err
			return execErr
		default:
		}

		for _, nodeName := range layer {
			task := taskPool.Get().(*nodeTask)
			task.ctx = execCtx
			task.name = nodeName
			pool.Submit(task)
		}

		layerTotal := len(layer)
		layerCompleted := 0
		for layerCompleted < layerTotal {
			select {
			case <-ctx.Done():
				return &ChainError{Message: fmt.Sprintf("execution canceled: %v", ctx.Err())}
			case err := <-errChan:
				execErr = err
				return execErr
			case <-layerDone:
				layerCompleted++
			}
		}
	}

	for _, state := range states {
		nodeStatePool.Put(state)
	}

	return execErr
}

func waitForDone(state *nodeState, ctx context.Context) bool {
	if atomic.LoadUint32(&state.done) != 0 {
		return true
	}
	select {
	case <-state.doneSig:
		return true
	case <-ctx.Done():
		return false
	}
}

func executeNodeWorkerTask(task *nodeTask) {
	ctx := task.ctx
	name := task.name

	state := ctx.states[name]
	inEdges := ctx.incomingEdges[name]
	var inputs []any
	var hasValidInput bool

	defer func() {
		atomic.StoreUint32(&state.done, 1)
		close(state.doneSig)
		if ctx.doneChan != nil {
			select {
			case ctx.doneChan <- struct{}{}:
			default:
			}
		}
	}()

	if len(inEdges) == 0 {
		hasValidInput = true
	} else {
		inputsBuf := anySlicePool.Get(16)
		defer anySlicePool.Put(inputsBuf)

		branchTargetNodes := ctx.branchTargetNodes

		completedCount := 0
		requiredCount := 0
		normalEdges := 0
		for _, edge := range inEdges {
			if edge.edgeType == EdgeTypeLoop {
				continue
			}
			requiredCount++
			if branchTargetNodes[edge.from] {
				requiredCount--
			} else {
				normalEdges++
			}
		}

		if normalEdges > 0 {
			for _, edge := range inEdges {
				if edge.edgeType == EdgeTypeLoop {
					continue
				}
				if branchTargetNodes[edge.from] {
					continue
				}
				fromState := ctx.states[edge.from]
				if !waitForDone(fromState, ctx.ctx) {
					return
				}
				if fromState.err != nil {
					select {
					case ctx.errChan <- fromState.err:
					default:
					}
					return
				}
				if edge.condFunc == nil || edge.condFunc(fromState.results) {
					inputsBuf = append(inputsBuf, fromState.results...)
					completedCount++
				}
			}
		}

		for _, edge := range inEdges {
			if edge.edgeType == EdgeTypeLoop {
				continue
			}
			if !branchTargetNodes[edge.from] {
				continue
			}
			fromState := ctx.states[edge.from]
			if !waitForDone(fromState, ctx.ctx) {
				return
			}
			if fromState.err != nil {
				select {
				case ctx.errChan <- fromState.err:
				default:
				}
				return
			}
			if len(fromState.results) > 0 {
				inputsBuf = append(inputsBuf, fromState.results...)
				completedCount++
				break
			}
		}

		if requiredCount == 0 || completedCount >= requiredCount {
			hasValidInput = true
			inputs = make([]any, len(inputsBuf))
			copy(inputs, inputsBuf)
		}
	}

	if !hasValidInput {
		return
	}

	results, execErr := ctx.graph.executeNodeWithLoop(name, inputs)
	if execErr != nil {
		state.err = &ChainError{Message: fmt.Sprintf("node %s failed: %v", name, execErr)}
		select {
		case ctx.errChan <- state.err:
		default:
		}
		return
	}

	state.results = results
	ctx.graph.mu.Lock()
	ctx.graph.stepNames[name] = len(ctx.graph.stepNames)
	ctx.graph.mu.Unlock()
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

	return g.executeSequential(ctx, plan)
}

func (g *Graph) executeSequential(ctx context.Context, plan []string) error {
	resultsMap := make(map[string][]any, len(plan))

	for _, name := range plan {
		select {
		case <-ctx.Done():
			return &ChainError{Message: fmt.Sprintf("execution canceled: %v", ctx.Err())}
		default:
		}

		node := g.nodes[name]
		if node == nil {
			return &ChainError{Message: ErrNodeNotFound}
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
			return &ChainError{Message: fmt.Sprintf("node %s failed: %v", name, err)}
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
		return nil, &ChainError{Message: ErrNoStartNode}
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
		return nil, &ChainError{Message: ErrCyclicDependency}
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

	layerBounds := make([]int, 0, 16)
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
			return nil, &ChainError{Message: ErrNoStartNode}
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
		return nil, &ChainError{Message: ErrCyclicDependency}
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

	for i := 0; i < layerCount; i++ {
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

func (g *Graph) executeNode(nodeName string, inputs []any) ([]any, error) {
	node := g.nodes[nodeName]
	if node == nil {
		return nil, &ChainError{Message: ErrNodeNotFound}
	}

	node.status = NodeStatusRunning
	node.err = nil

	if node.callFn != nil {
		results, err := node.callFn(inputs)
		if err != nil {
			node.err = err
			node.status = NodeStatusFailed
			return nil, err
		}
		node.result = results
		node.status = NodeStatusCompleted
		return results, nil
	}

	node.status = NodeStatusCompleted
	return inputs, nil
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
