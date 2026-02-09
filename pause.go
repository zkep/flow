package flow

import (
	"context"
	"errors"
	"sync"
)

type PausableFlow interface {
	FlowCheckpointable
	Pause() error
	Resume(ctx context.Context) error
	State() FlowState
}

type PauseMode int

const (
	PauseModeImmediate PauseMode = iota
	PauseModeAtNode
	PauseModeOnError
)

type PauseConfig struct {
	Mode         PauseMode
	PauseAtNodes map[string]bool
	OnErrorPause bool
}

func NewPauseConfig() *PauseConfig {
	return &PauseConfig{
		Mode:         PauseModeImmediate,
		PauseAtNodes: make(map[string]bool),
		OnErrorPause: false,
	}
}

func (c *PauseConfig) SetPauseAtNodes(names ...string) *PauseConfig {
	c.Mode = PauseModeAtNode
	for _, name := range names {
		c.PauseAtNodes[name] = true
	}
	return c
}

func (c *PauseConfig) SetPauseOnError() *PauseConfig {
	c.OnErrorPause = true
	return c
}

func (c *PauseConfig) ShouldPauseAtNode(nodeName string) bool {
	if c.Mode == PauseModeAtNode {
		return c.PauseAtNodes[nodeName]
	}
	return false
}

type ResumeConfig struct {
	SkipCompleted bool
	RetryFailed   bool
}

func NewResumeConfig() *ResumeConfig {
	return &ResumeConfig{
		SkipCompleted: true,
		RetryFailed:   false,
	}
}

func (c *ResumeConfig) SetRetryFailed() *ResumeConfig {
	c.RetryFailed = true
	return c
}

type ResourceChecker interface {
	CheckAvailable(nodeName string) bool
}

type PauseSignal interface {
	ShouldPause() bool
	Reset()
}

type SimplePauseSignal struct {
	paused bool
	mu     sync.RWMutex
}

func NewSimplePauseSignal() *SimplePauseSignal {
	return &SimplePauseSignal{}
}

func (s *SimplePauseSignal) SetPaused(paused bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paused = paused
}

func (s *SimplePauseSignal) ShouldPause() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.paused
}

func (s *SimplePauseSignal) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paused = false
}

type SimpleResourceChecker struct {
	available  int
	perNodeUse int
	mu         sync.RWMutex
}

func NewSimpleResourceChecker(available, perNodeUse int) *SimpleResourceChecker {
	return &SimpleResourceChecker{
		available:  available,
		perNodeUse: perNodeUse,
	}
}

func (c *SimpleResourceChecker) SetAvailable(available int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.available = available
}

func (c *SimpleResourceChecker) CheckAvailable(nodeName string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.available >= c.perNodeUse
}

func (c *SimpleResourceChecker) Consume() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.available -= c.perNodeUse
}

func (c *SimpleResourceChecker) Release() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.available += c.perNodeUse
}

var (
	ErrNodeNotPausable      = errors.New("node is not in pausable state")
	ErrNoPausePoint         = errors.New("no pause point set")
	ErrFlowPaused           = errors.New("flow is paused")
	ErrResourceNotAvailable = errors.New("resource not available")
)

func (g *Graph) Pause() error {
	return g.PauseWithConfig(NewPauseConfig())
}

func (g *Graph) PauseWithConfig(config *PauseConfig) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, node := range g.nodes {
		if node.status == NodeStatusRunning {
			node.status = NodeStatusPending
		}
	}

	return nil
}

func (g *Graph) PauseAtNode(nodeName string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	node, ok := g.nodes[nodeName]
	if !ok {
		return &FlowError{Message: ErrNodeNotFound}
	}

	if node.status == NodeStatusCompleted || node.status == NodeStatusFailed {
		return ErrNodeNotPausable
	}

	return nil
}

func (g *Graph) Resume(ctx context.Context) error {
	return g.ResumeWithConfig(ctx, NewResumeConfig())
}

func (g *Graph) ResumeWithConfig(ctx context.Context, config *ResumeConfig) error {
	g.mu.Lock()

	g.pausedAtNode = ""
	g.err = nil

	if g.pauseSignal != nil {
		g.pauseSignal.Reset()
	}

	for _, node := range g.nodes {
		if config.SkipCompleted && node.status == NodeStatusCompleted {
			continue
		}
		if config.RetryFailed && node.status == NodeStatusFailed {
			node.status = NodeStatusPending
			node.result = nil
			node.err = nil
		}
	}

	g.mu.Unlock()

	return g.RunWithContext(ctx)
}

func (g *Graph) State() FlowState {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.err != nil {
		return FlowStateFailed
	}

	if g.pausedAtNode != "" {
		return FlowStatePaused
	}

	completed := 0
	total := len(g.nodes)

	for _, node := range g.nodes {
		if node.status == NodeStatusCompleted {
			completed++
		}
	}

	if completed == 0 {
		return FlowStateIdle
	}
	if completed == total {
		return FlowStateCompleted
	}
	return FlowStatePaused
}

func (g *Graph) GetNodesByStatus(status NodeStatus) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]string, 0)
	for name, node := range g.nodes {
		if node.status == status {
			result = append(result, name)
		}
	}
	return result
}
