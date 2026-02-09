package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/zkep/flow"
)

//go:embed approval_flow.json
var ApprovalFlowJSONData []byte

type ApprovalNode struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Label     string `json:"label"`
	Action    string `json:"action,omitempty"`
	Condition string `json:"condition,omitempty"`
	TrueEdge  string `json:"true_edge,omitempty"`
	FalseEdge string `json:"false_edge,omitempty"`
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
	Applicant string
	Days      int
	Reason    string
	Approved  bool
	Comments  map[string]string
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

func BuildAndRunFlow(approvalFlow *ApprovalFlow, ctx *ApprovalContext, registry *ActionRegistry) error {
	g := flow.NewGraph()

	for _, node := range approvalFlow.Nodes {
		handler, _ := registry.Get(node.Action)
		g.AddNode(node.ID, func() error {
			fmt.Printf("\n[%s] %s\n", node.ID, node.Label)
			fmt.Println("  " + strings.Repeat("-", 40))

			if handler != nil {
				return handler(ctx)
			}

			if node.Type == "condition" {
				fmt.Printf("  [Condition] %s\n", node.Condition)
			}
			return nil
		})
	}

	for _, edge := range approvalFlow.Edges {
		if edge.Condition != "" {
			condition := edge.Condition
			g.AddEdgeWithCondition(edge.From, edge.To, func() bool {
				result, err := evalCondition(condition, ctx)
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

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Printf("Flow Name: %s\n", approvalFlow.Name)
	fmt.Printf("Description: %s\n", approvalFlow.Description)
	fmt.Println(strings.Repeat("=", 60))

	err := g.Run()
	if err != nil {
		return fmt.Errorf("flow execution failed: %w", err)
	}

	return nil
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

	return registry
}

func main() {
	approvalFlow, err := LoadApprovalFlowFromData(ApprovalFlowJSONData)
	if err != nil {
		fmt.Printf("Failed to load flow: %v\n", err)
		os.Exit(1)
	}

	registry := NewDefaultActionRegistry()

	testCases := []*ApprovalContext{
		{
			Applicant: "Zhang San",
			Days:      2,
			Reason:    "Family matters",
			Comments:  make(map[string]string),
		},
		{
			Applicant: "Li Si",
			Days:      5,
			Reason:    "Annual leave",
			Comments:  make(map[string]string),
		},
	}

	for i, testCase := range testCases {
		fmt.Printf("\n\n")
		fmt.Println(strings.Repeat("#", 60))
		fmt.Printf("## Test Case %d\n", i+1)
		fmt.Println(strings.Repeat("#", 60))

		testCase.Comments = make(map[string]string)

		err := BuildAndRunFlow(approvalFlow, testCase, registry)
		if err != nil {
			fmt.Printf("Execution failed: %v\n", err)
		}

		fmt.Println("\n")
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("All test cases completed")
	fmt.Println(strings.Repeat("=", 60))
}
