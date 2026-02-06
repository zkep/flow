package flow

import "sync"

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
				return make([]T, 0, defaultCap)
			},
		},
		defaultCap:  defaultCap,
		minCapacity: minCapacity,
	}
}

func (p *SlicePool[T]) Get(minCap int) []T {
	s := p.pool.Get().([]T)[:0]
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
		p.pool.Put(s[:0])
	}
}
