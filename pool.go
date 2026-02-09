package flow

import (
	"reflect"
	"sync"
)

const (
	defaultSlicePoolCap = 128
	defaultSlicePoolMin = 32
)

type ObjectPool[T any] struct {
	pool  sync.Pool
	reset func(T)
}

func NewObjectPool[T any](creator func() T, opts ...PoolOption[T]) *ObjectPool[T] {
	p := &ObjectPool[T]{
		pool: sync.Pool{
			New: func() any { return creator() },
		},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

type PoolOption[T any] func(*ObjectPool[T])

func WithReset[T any](reset func(T)) PoolOption[T] {
	return func(p *ObjectPool[T]) {
		p.reset = reset
	}
}

func (p *ObjectPool[T]) Get() T {
	return p.pool.Get().(T)
}

func (p *ObjectPool[T]) Put(x T) {
	if p.reset != nil {
		p.reset(x)
	}
	p.pool.Put(x)
}

type SlicePool[T any] struct {
	pool        sync.Pool
	defaultCap  int
	minCapacity int
}

func NewSlicePool[T any](defaultCap, minCapacity int) *SlicePool[T] {
	if defaultCap <= 0 {
		defaultCap = 8
	}
	if minCapacity <= 0 {
		minCapacity = defaultCap
	}
	return &SlicePool[T]{
		pool: sync.Pool{
			New: func() any {
				s := make([]T, 0, defaultCap)
				return &s
			},
		},
		defaultCap:  defaultCap,
		minCapacity: minCapacity,
	}
}

func (p *SlicePool[T]) Get(minCap int) []T {
	sp := p.pool.Get().(*[]T)
	s := (*sp)[:0]
	if cap(s) < minCap {
		if minCap > p.defaultCap {
			return make([]T, 0, minCap)
		}
		return make([]T, 0, p.defaultCap)
	}
	return s
}

func (p *SlicePool[T]) Put(s []T) {
	if cap(s) >= p.minCapacity {
		sp := s[:0]
		p.pool.Put(&sp)
	}
}

var (
	anySlicePool          = NewSlicePool[any](defaultSlicePoolCap, defaultSlicePoolMin)
	stringSlicePool       = NewSlicePool[string](defaultSlicePoolCap, defaultSlicePoolMin)
	reflectValueSlicePool = NewSlicePool[reflect.Value](defaultSlicePoolCap, defaultSlicePoolMin)

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

var (
	taskPool = sync.Pool{
		New: func() any { return &nodeTask{} },
	}

	localWorkerPoolPool = sync.Pool{
		New: func() any {
			return &localWorkerPool{}
		},
	}
)
