package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zkep/flow"
)

//go:embed approval_flow.json
var ApprovalFlowJSONData []byte

type FlowStatus int

const (
	FlowStatusIdle FlowStatus = iota
	FlowStatusRunning
	FlowStatusPaused
	FlowStatusCompleted
	FlowStatusRejected
	FlowStatusReturned
)

type ApprovalType string

const (
	ApprovalTypeSingle   ApprovalType = "single"
	ApprovalTypeAll      ApprovalType = "all"
	ApprovalTypeAny      ApprovalType = "any"
	ApprovalTypeParallel ApprovalType = "parallel"
)

type ApproverDecision struct {
	Approver   string
	Approved   bool
	Comment    string
	DecisionAt time.Time
}

type ApprovalNode struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Label        string            `json:"label"`
	Action       string            `json:"action,omitempty"`
	Condition    string            `json:"condition,omitempty"`
	TrueEdge     string            `json:"true_edge,omitempty"`
	FalseEdge    string            `json:"false_edge,omitempty"`
	ApprovalType ApprovalType      `json:"approval_type,omitempty"`
	Approvers    []string          `json:"approvers,omitempty"`
	ReturnTarget string            `json:"return_target,omitempty"`
	Config       map[string]string `json:"config,omitempty"`
}

type ApprovalEdge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Condition string `json:"condition,omitempty"`
}

type ApprovalFlow struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Nodes       []ApprovalNode `json:"nodes"`
	Edges       []ApprovalEdge `json:"edges"`
}

type ApprovalContext struct {
	Applicant     string
	Days          int
	Reason        string
	Approved      bool
	Comments      map[string]string
	Decisions     map[string]*ApproverDecision
	CurrentNode   string
	ReturnedFrom  string
	ResubmitCount int
	Status        FlowStatus
	PausedAt      time.Time
	ResumedAt     time.Time
	ApprovalType  ApprovalType
	RequiredCount int
	ApprovedCount int
	mu            sync.RWMutex
}

func NewApprovalContext(applicant string, days int, reason string) *ApprovalContext {
	return &ApprovalContext{
		Applicant:     applicant,
		Days:          days,
		Reason:        reason,
		Approved:      false,
		Comments:      make(map[string]string),
		Decisions:     make(map[string]*ApproverDecision),
		Status:        FlowStatusIdle,
		ApprovalType:  ApprovalTypeSingle,
		RequiredCount: 1,
		ApprovedCount: 0,
	}
}

func (ctx *ApprovalContext) SetStatus(status FlowStatus) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.Status = status
}

func (ctx *ApprovalContext) GetStatus() FlowStatus {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	return ctx.Status
}

func (ctx *ApprovalContext) RecordDecision(approver string, approved bool, comment string) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.Decisions[approver] = &ApproverDecision{
		Approver:   approver,
		Approved:   approved,
		Comment:    comment,
		DecisionAt: time.Now(),
	}
	if approved {
		ctx.ApprovedCount++
	}
	ctx.Comments[approver] = comment
}

func (ctx *ApprovalContext) IsAllApproved() bool {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	return ctx.ApprovedCount >= ctx.RequiredCount && ctx.RequiredCount > 0
}

func (ctx *ApprovalContext) IsAnyApproved() bool {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	return ctx.ApprovedCount > 0
}

func (ctx *ApprovalContext) ResetForResubmit() {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.Approved = false
	ctx.ResubmitCount++
	ctx.Decisions = make(map[string]*ApproverDecision)
	ctx.ApprovedCount = 0
}

type ActionHandler func(*ApprovalContext) error

type ActionRegistry struct {
	handlers map[string]ActionHandler
}

func NewActionRegistry() *ActionRegistry {
	return &ActionRegistry{
		handlers: make(map[string]ActionHandler),
	}
}

func (r *ActionRegistry) Register(name string, handler ActionHandler) {
	r.handlers[name] = handler
}

func (r *ActionRegistry) Get(name string) (ActionHandler, bool) {
	h, ok := r.handlers[name]
	return h, ok
}

func (r *ActionRegistry) List() []string {
	names := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		names = append(names, name)
	}
	return names
}

type ApprovalFlowEngine struct {
	flow         *ApprovalFlow
	ctx          *ApprovalContext
	registry     *ActionRegistry
	graph        *flow.Graph
	pauseSignal  *flow.SimplePauseSignal
	checkpoint   *flow.MemoryCheckpointStore
	currentNode  string
	returnTarget string
	isReturned   bool
	mu           sync.RWMutex
}

func NewApprovalFlowEngine(approvalFlow *ApprovalFlow, ctx *ApprovalContext, registry *ActionRegistry) *ApprovalFlowEngine {
	return &ApprovalFlowEngine{
		flow:        approvalFlow,
		ctx:         ctx,
		registry:    registry,
		graph:       flow.NewGraph(),
		pauseSignal: flow.NewSimplePauseSignal(),
		checkpoint:  flow.NewMemoryCheckpointStore(),
	}
}

func (e *ApprovalFlowEngine) Pause() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pauseSignal.SetPaused(true)
	e.ctx.SetStatus(FlowStatusPaused)
	e.ctx.PausedAt = time.Now()
	fmt.Printf("\n  [System] Flow paused at node: %s\n", e.currentNode)
	return nil
}

func (e *ApprovalFlowEngine) Resume() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pauseSignal.SetPaused(false)
	e.ctx.SetStatus(FlowStatusRunning)
	e.ctx.ResumedAt = time.Now()
	fmt.Printf("\n  [System] Flow resumed from node: %s\n", e.currentNode)
	return e.graph.Resume(nil)
}

func (e *ApprovalFlowEngine) ReturnTo(targetNode string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.returnTarget = targetNode
	e.isReturned = true
	e.ctx.ReturnedFrom = e.currentNode
	e.ctx.SetStatus(FlowStatusReturned)
	fmt.Printf("\n  [System] Flow returned from %s to %s\n", e.currentNode, targetNode)
	return nil
}

func (e *ApprovalFlowEngine) Resubmit() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.isReturned {
		return errors.New("flow is not in returned state")
	}
	e.ctx.ResetForResubmit()
	e.ctx.SetStatus(FlowStatusRunning)
	e.isReturned = false
	e.returnTarget = ""
	fmt.Printf("\n  [System] Flow resubmitted (resubmit count: %d)\n", e.ctx.ResubmitCount)
	return e.Run()
}

func (e *ApprovalFlowEngine) SaveCheckpoint(key string) error {
	cp, err := e.graph.SaveCheckpoint()
	if err != nil {
		return err
	}
	cp.SetMetadata("currentNode", e.currentNode)
	cp.SetMetadata("applicant", e.ctx.Applicant)
	return e.checkpoint.Save(key, cp)
}

func (e *ApprovalFlowEngine) LoadCheckpoint(key string) error {
	cp, err := e.checkpoint.Load(key)
	if err != nil {
		return err
	}
	if err := e.graph.LoadCheckpoint(cp); err != nil {
		return err
	}
	if nodeName, ok := cp.GetMetadata("currentNode"); ok {
		e.currentNode = nodeName
	}
	return nil
}

func (e *ApprovalFlowEngine) buildGraph() error {
	g := flow.NewGraph()
	e.graph = g

	g.SetPauseSignal(e.pauseSignal)

	nodeMap := make(map[string]*ApprovalNode)
	for i := range e.flow.Nodes {
		nodeMap[e.flow.Nodes[i].ID] = &e.flow.Nodes[i]
	}

	for _, node := range e.flow.Nodes {
		handler, _ := e.registry.Get(node.Action)

		g.AddNode(node.ID, func() error {
			e.mu.Lock()
			e.currentNode = node.ID
			e.ctx.CurrentNode = node.ID
			e.mu.Unlock()

			fmt.Printf("\n[%s] %s\n", node.ID, node.Label)
			fmt.Println("  " + strings.Repeat("-", 40))

			if e.isReturned && e.returnTarget == node.ID {
				fmt.Printf("  [Returned] Flow returned to this node for reprocessing\n")
				e.isReturned = false
				e.returnTarget = ""
			}

			if handler != nil {
				if err := handler(e.ctx); err != nil {
					return err
				}
			}

			if node.Type == "condition" {
				fmt.Printf("  [Condition] %s\n", node.Condition)
			}

			if node.ApprovalType != "" {
				e.ctx.ApprovalType = node.ApprovalType
				e.ctx.RequiredCount = len(node.Approvers)
				fmt.Printf("  [Approval Type] %s, Required Approvers: %d\n", node.ApprovalType, len(node.Approvers))
			}

			return nil
		})
	}

	for _, edge := range e.flow.Edges {
		if edge.Condition != "" {
			condition := edge.Condition
			g.AddEdgeWithCondition(edge.From, edge.To, func() bool {
				result, err := evalCondition(condition, e.ctx)
				if err != nil {
					fmt.Printf("  [Warning] condition evaluation failed: %v\n", err)
					return false
				}
				return result
			})
		} else {
			g.AddEdge(edge.From, edge.To)
		}
	}

	return nil
}

func (e *ApprovalFlowEngine) Run() error {
	if err := e.buildGraph(); err != nil {
		return fmt.Errorf("failed to build graph: %w", err)
	}

	e.ctx.SetStatus(FlowStatusRunning)

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Printf("Flow Name: %s\n", e.flow.Name)
	fmt.Printf("Description: %s\n", e.flow.Description)
	fmt.Println(strings.Repeat("=", 60))

	err := e.graph.Run()
	if err != nil {
		if errors.Is(err, flow.ErrFlowPaused) {
			e.ctx.SetStatus(FlowStatusPaused)
			return nil
		}
		e.ctx.SetStatus(FlowStatusRejected)
		return fmt.Errorf("flow execution failed: %w", err)
	}

	e.ctx.SetStatus(FlowStatusCompleted)
	return nil
}

func (e *ApprovalFlowEngine) GetState() FlowStatus {
	return e.ctx.GetStatus()
}

func (e *ApprovalFlowEngine) CurrentNode() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.currentNode
}

func formatApproved(approved bool) string {
	if approved {
		return "Approved"
	}
	return "Rejected"
}

func getValue(key string, ctx *ApprovalContext) interface{} {
	switch key {
	case "days":
		return ctx.Days
	case "approved":
		return ctx.Approved
	case "applicant":
		return ctx.Applicant
	case "reason":
		return ctx.Reason
	case "resubmit_count":
		return ctx.ResubmitCount
	case "approved_count":
		return ctx.ApprovedCount
	case "required_count":
		return ctx.RequiredCount
	}
	return nil
}

func compareInt(left, right int, op string) bool {
	switch op {
	case ">":
		return left > right
	case ">=":
		return left >= right
	case "<":
		return left < right
	case "<=":
		return left <= right
	case "==":
		return left == right
	case "!=":
		return left != right
	}
	return false
}

func compareBool(left, right bool, op string) bool {
	switch op {
	case "==":
		return left == right
	case "!=":
		return left != right
	}
	return false
}

func evalSimpleCondition(condition string, approvalCtx *ApprovalContext) (bool, error) {
	condition = strings.TrimSpace(condition)

	operators := []string{">=", "<=", "!=", "==", ">", "<"}
	for _, op := range operators {
		if idx := strings.Index(condition, op); idx > 0 {
			left := strings.TrimSpace(condition[:idx])
			right := strings.TrimSpace(condition[idx+len(op):])

			leftVal := getValue(left, approvalCtx)
			if leftVal == nil {
				return false, fmt.Errorf("unknown variable: %s", left)
			}

			switch lv := leftVal.(type) {
			case int:
				rv, err := strconv.Atoi(right)
				if err != nil {
					return false, fmt.Errorf("cannot convert to integer: %s", right)
				}
				return compareInt(lv, rv, op), nil
			case bool:
				rv := right == "true"
				return compareBool(lv, rv, op), nil
			}
		}
	}

	return false, fmt.Errorf("cannot parse condition: %s", condition)
}

func evalCondition(condition string, approvalCtx *ApprovalContext) (bool, error) {
	condition = strings.TrimSpace(condition)

	if strings.Contains(condition, "&&") {
		parts := strings.Split(condition, "&&")
		for _, part := range parts {
			result, err := evalSimpleCondition(strings.TrimSpace(part), approvalCtx)
			if err != nil {
				return false, err
			}
			if !result {
				return false, nil
			}
		}
		return true, nil
	}

	if strings.Contains(condition, "||") {
		parts := strings.Split(condition, "||")
		for _, part := range parts {
			result, err := evalSimpleCondition(strings.TrimSpace(part), approvalCtx)
			if err != nil {
				return false, err
			}
			if result {
				return true, nil
			}
		}
		return false, nil
	}

	return evalSimpleCondition(condition, approvalCtx)
}

func ValidateFlow(flow *ApprovalFlow) error {
	if flow.Name == "" {
		return errors.New("flow name cannot be empty")
	}

	if len(flow.Nodes) == 0 {
		return errors.New("flow nodes cannot be empty")
	}

	nodeIDs := make(map[string]bool)
	for _, node := range flow.Nodes {
		if node.ID == "" {
			return errors.New("node ID cannot be empty")
		}
		if nodeIDs[node.ID] {
			return fmt.Errorf("duplicate node ID: %s", node.ID)
		}
		nodeIDs[node.ID] = true
	}

	for _, edge := range flow.Edges {
		if !nodeIDs[edge.From] {
			return fmt.Errorf("edge references non-existent source node: %s", edge.From)
		}
		if !nodeIDs[edge.To] {
			return fmt.Errorf("edge references non-existent target node: %s", edge.To)
		}
	}

	return nil
}

func LoadApprovalFlowFromData(data []byte) (*ApprovalFlow, error) {
	var flow ApprovalFlow
	if err := json.Unmarshal(data, &flow); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if err := ValidateFlow(&flow); err != nil {
		return nil, fmt.Errorf("flow configuration validation failed: %w", err)
	}

	return &flow, nil
}

func LoadApprovalFlow(path string) (*ApprovalFlow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file: %w", err)
	}
	return LoadApprovalFlowFromData(data)
}

func NewDefaultActionRegistry() *ActionRegistry {
	registry := NewActionRegistry()

	registry.Register("init_request", func(ctx *ApprovalContext) error {
		fmt.Printf("  [Init Request] %s requests %d days leave, reason: %s\n", ctx.Applicant, ctx.Days, ctx.Reason)
		return nil
	})

	registry.Register("submit_application", func(ctx *ApprovalContext) error {
		fmt.Println("  [Submit] Application submitted, awaiting approval...")
		return nil
	})

	registry.Register("team_leader_approve", func(ctx *ApprovalContext) error {
		fmt.Printf("  [Team Leader] Reviewing leave request for %s...\n", ctx.Applicant)
		ctx.Approved = true
		ctx.Comments["team_leader"] = "Approved, work arrangement confirmed"
		fmt.Printf("  [Team Leader] Decision: %s, Comment: %s\n", formatApproved(ctx.Approved), ctx.Comments["team_leader"])
		return nil
	})

	registry.Register("manager_approve", func(ctx *ApprovalContext) error {
		fmt.Printf("  [Manager] Reviewing leave request for %s (%d days)...\n", ctx.Applicant, ctx.Days)
		ctx.Approved = true
		ctx.Comments["manager"] = "Approved, please arrange handover"
		fmt.Printf("  [Manager] Decision: %s, Comment: %s\n", formatApproved(ctx.Approved), ctx.Comments["manager"])
		return nil
	})

	registry.Register("hr_review", func(ctx *ApprovalContext) error {
		fmt.Printf("  [HR Review] Reviewing leave request for %s...\n", ctx.Applicant)
		ctx.Comments["hr"] = "Recorded in system, sufficient leave balance"
		fmt.Printf("  [HR Review] Completed, Comment: %s\n", ctx.Comments["hr"])
		return nil
	})

	registry.Register("notify_approved", func(ctx *ApprovalContext) error {
		fmt.Printf("\n  Approved! %s's %d days leave request has been approved\n", ctx.Applicant, ctx.Days)
		return nil
	})

	registry.Register("notify_rejected", func(ctx *ApprovalContext) error {
		fmt.Printf("\n  Rejected! %s's leave request has been rejected\n", ctx.Applicant)
		return nil
	})

	registry.Register("countersign_approve", func(ctx *ApprovalContext) error {
		fmt.Printf("  [Countersign] Requires all %d approvers to approve\n", ctx.RequiredCount)
		for i, approver := range []string{"Dept Head", "Finance", "HR"} {
			if i >= ctx.RequiredCount {
				break
			}
			approved := true
			comment := fmt.Sprintf("%s approved", approver)
			ctx.RecordDecision(approver, approved, comment)
			fmt.Printf("    - [%s] %s\n", approver, comment)
		}
		ctx.Approved = ctx.IsAllApproved()
		fmt.Printf("  [Countersign Result] All approved: %v\n", ctx.Approved)
		return nil
	})

	registry.Register("parallel_approve", func(ctx *ApprovalContext) error {
		fmt.Printf("  [Parallel] Any of %d approvers can approve\n", ctx.RequiredCount)
		for i, approver := range []string{"Director A", "Director B", "Director C"} {
			if i >= ctx.RequiredCount {
				break
			}
			approved := i == 0
			comment := fmt.Sprintf("%s decision", approver)
			if approved {
				comment = "Approved"
			}
			ctx.RecordDecision(approver, approved, comment)
			if approved {
				fmt.Printf("    - [%s] %s (first approval, others skipped)\n", approver, comment)
				break
			}
			fmt.Printf("    - [%s] %s\n", approver, comment)
		}
		ctx.Approved = ctx.IsAnyApproved()
		fmt.Printf("  [Parallel Result] Any approved: %v\n", ctx.Approved)
		return nil
	})

	registry.Register("handle_return", func(ctx *ApprovalContext) error {
		fmt.Printf("  [Return] Application returned from %s\n", ctx.ReturnedFrom)
		fmt.Printf("  [Return] Current resubmit count: %d\n", ctx.ResubmitCount)
		if ctx.ResubmitCount >= 3 {
			ctx.Approved = false
			fmt.Println("  [Return] Max resubmit count reached, rejecting...")
			return errors.New("max resubmit count exceeded")
		}
		return nil
	})

	registry.Register("resubmit_request", func(ctx *ApprovalContext) error {
		fmt.Printf("  [Resubmit] Preparing resubmission (attempt %d)...\n", ctx.ResubmitCount+1)
		ctx.Reason = ctx.Reason + " (updated)"
		fmt.Printf("  [Resubmit] Updated reason: %s\n", ctx.Reason)
		return nil
	})

	registry.Register("ceo_approval", func(ctx *ApprovalContext) error {
		fmt.Printf("  [CEO Approval] Reviewing high-value request for %s (%d days)...\n", ctx.Applicant, ctx.Days)
		ctx.Approved = true
		ctx.Comments["ceo"] = "Approved for business needs"
		fmt.Printf("  [CEO] Decision: %s, Comment: %s\n", formatApproved(ctx.Approved), ctx.Comments["ceo"])
		return nil
	})

	return registry
}

func main() {
	approvalFlow, err := LoadApprovalFlowFromData(ApprovalFlowJSONData)
	if err != nil {
		fmt.Printf("Failed to load flow: %v\n", err)
		os.Exit(1)
	}

	registry := NewDefaultActionRegistry()

	testCases := []struct {
		name string
		ctx  *ApprovalContext
		run  func(*ApprovalFlowEngine) error
	}{
		{
			name: "Normal Flow (3 days)",
			ctx:  NewApprovalContext("Zhang San", 3, "Family matters"),
			run:  nil,
		},
		{
			name: "Normal Flow (5 days)",
			ctx:  NewApprovalContext("Li Si", 5, "Annual leave"),
			run:  nil,
		},
		{
			name: "Pause/Resume Test",
			ctx:  NewApprovalContext("Wang Wu", 2, "Sick leave"),
			run: func(engine *ApprovalFlowEngine) error {
				go func() {
					time.Sleep(100 * time.Millisecond)
					fmt.Println("\n  [External] Pausing flow...")
					engine.Pause()
				}()

				if err := engine.Run(); err != nil {
					return err
				}

				if engine.GetState() == FlowStatusPaused {
					fmt.Printf("  [Test] Flow paused at node: %s\n", engine.CurrentNode())
					time.Sleep(500 * time.Millisecond)
					fmt.Println("  [Test] Resuming flow...")
					return engine.Resume()
				}
				return nil
			},
		},
		{
			name: "Countersign Test (All approve)",
			ctx:  NewApprovalContext("Zhao Liu", 10, "Long vacation"),
			run:  nil,
		},
		{
			name: "Parallel Sign Test (Any approve)",
			ctx:  NewApprovalContext("Sun Qi", 7, "Business trip"),
			run:  nil,
		},
	}

	for i, tc := range testCases {
		fmt.Printf("\n\n")
		fmt.Println(strings.Repeat("#", 60))
		fmt.Printf("## Test Case %d: %s\n", i+1, tc.name)
		fmt.Println(strings.Repeat("#", 60))

		engine := NewApprovalFlowEngine(approvalFlow, tc.ctx, registry)

		if tc.run != nil {
			if err := tc.run(engine); err != nil {
				fmt.Printf("Execution failed: %v\n", err)
			}
		} else {
			if err := engine.Run(); err != nil {
				fmt.Printf("Execution failed: %v\n", err)
			}
		}

		fmt.Printf("\n  [Final State] %v\n", engine.GetState())
		fmt.Println("\n")
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("All test cases completed")
	fmt.Println(strings.Repeat("=", 60))
}
