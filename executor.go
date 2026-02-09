package flow

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

const (
	defaultWorkerCount     = 8
	defaultTaskChannelSize = 1024
	taskChannelMultiplier  = 4
	defaultInputBufferSize = 16
)

type localWorkerPool struct {
	workers  int
	taskChan chan *nodeTask
	wg       sync.WaitGroup
}

func newLocalWorkerPool(workers int) *localWorkerPool {
	if workers <= 0 {
		workers = defaultWorkerCount
	}
	pool := localWorkerPoolPool.Get().(*localWorkerPool)
	pool.workers = workers
	if pool.taskChan == nil || cap(pool.taskChan) < workers*taskChannelMultiplier {
		pool.taskChan = make(chan *nodeTask, workers*taskChannelMultiplier)
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

type globalWorker struct {
	taskChan chan *nodeTask
	wg       sync.WaitGroup
}

var gw *globalWorker
var gwOnce sync.Once

func getGlobalWorker() *globalWorker {
	gwOnce.Do(func() {
		gw = &globalWorker{
			taskChan: make(chan *nodeTask, defaultTaskChannelSize),
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
		return &FlowError{Message: fmt.Sprintf("execution canceled: %v", ctx.Err())}
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
			execErr = &FlowError{Message: fmt.Sprintf("execution canceled: %v", ctx.Err())}
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

func executeNodeWorkerTask(task *nodeTask) { //nolint:gocyclo
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
		inputsBuf := anySlicePool.Get(defaultInputBufferSize)
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

	if ctx.graph.shouldPauseForSignal() {
		ctx.graph.mu.Lock()
		ctx.graph.pausedAtNode = name
		ctx.graph.mu.Unlock()
		state.err = ErrFlowPaused
		select {
		case ctx.errChan <- state.err:
		default:
		}
		return
	}

	if ctx.graph.shouldPauseAtNode(name) {
		ctx.graph.mu.Lock()
		ctx.graph.pausedAtNode = name
		ctx.graph.mu.Unlock()
		state.err = ErrFlowPaused
		select {
		case ctx.errChan <- state.err:
		default:
		}
		return
	}

	if !ctx.graph.checkResourceAvailable(name) {
		ctx.graph.mu.Lock()
		ctx.graph.pausedAtNode = name
		ctx.graph.mu.Unlock()
		state.err = ErrResourceNotAvailable
		select {
		case ctx.errChan <- state.err:
		default:
		}
		return
	}

	ctx.graph.mu.RLock()
	node := ctx.graph.nodes[name]
	ctx.graph.mu.RUnlock()

	if node == nil {
		return
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
		state.results = existingResult
		return
	}

	results, execErr := ctx.graph.executeNodeWithLoop(name, inputs)
	if execErr != nil {
		if ctx.graph.pauseConfig != nil && ctx.graph.pauseConfig.OnErrorPause {
			ctx.graph.mu.Lock()
			ctx.graph.pausedAtNode = name
			ctx.graph.mu.Unlock()
		}
		state.err = &FlowError{Message: fmt.Sprintf("node %s failed: %v", name, execErr)}
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

func (g *Graph) executeGraphParallelLarge(ctx context.Context) error {
	layers, err := g.buildLayers()
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return &FlowError{Message: fmt.Sprintf("execution canceled: %v", ctx.Err())}
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
			return &FlowError{Message: fmt.Sprintf("execution canceled: %v", ctx.Err())}
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
				return &FlowError{Message: fmt.Sprintf("execution canceled: %v", ctx.Err())}
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
