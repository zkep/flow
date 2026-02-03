package flow

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

type TestData struct {
	Value  int
	Status string
}

func TestGraphRunParallelWithContext(t *testing.T) {
	graph := NewGraph()

	ctx := context.Background()

	graph.StartNode("start", func() int { return 10 })
	graph.ParallelNode("parallel1", func(n int) int {
		return n * 2
	})
	graph.ParallelNode("parallel2", func(n int) int {
		return n * 3
	})
	graph.Node("combine", func(a, b int) int {
		return a + b
	})

	graph.AddEdge("start", "parallel1")
	graph.AddEdge("start", "parallel2")
	graph.AddEdge("parallel1", "combine")
	graph.AddEdge("parallel2", "combine")

	err := graph.RunParallelWithContext(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	combineResult := graph.NodeResult("combine")
	if len(combineResult) != 1 {
		t.Fatalf("Expected combine result, got %v", combineResult)
	}
	if combineResult[0].(int) != 50 {
		t.Errorf("Expected 50, got %v", combineResult[0])
	}
}

func TestGraphRunParallelWithContextCanceled(t *testing.T) {
	graph := NewGraph()

	ctx, cancel := context.WithCancel(context.Background())

	graph.StartNode("start", func() int { return 10 })
	graph.ParallelNode("slow", func(n int) int {
		time.Sleep(100 * time.Millisecond)
		return n * 2
	})

	graph.AddEdge("start", "slow")

	cancel()

	err := graph.RunParallelWithContext(ctx)
	if err == nil {
		t.Fatalf("Expected context canceled error")
	}
	if !strings.Contains(err.Error(), "execution canceled") {
		t.Errorf("Expected canceled error, got %v", err.Error())
	}
}

func TestGraphEvaluateConditionWithNonFuncValue(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() int { return 10 })
	graph.Node("process", func(n int) int { return n * 2 })
	graph.AddEdgeWithCondition("start", "process", "truthy value")

	err := graph.RunSequential()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	status := graph.NodeStatus("process")
	if status != NodeStatusCompleted {
		t.Errorf("Expected process node to be completed")
	}
}

func TestGraphMermaidWithNoEdges(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() {})
	graph.EndNode("end", func() {})

	mermaidOutput := graph.Mermaid()
	if !strings.Contains(mermaidOutput, "graph TD") {
		t.Errorf("Expected mermaid output to contain 'graph TD'")
	}
	if !strings.Contains(mermaidOutput, "start") {
		t.Errorf("Expected mermaid output to contain 'start'")
	}
}

func (t *TestData) String() string {
	return fmt.Sprintf("TestData{%d, %q}", t.Value, t.Status)
}

func TestGraphValuePropagation(t *testing.T) {
	graph := NewGraph()

	input := TestData{Value: 10, Status: "input"}

	graph.StartNode("start", func() TestData {
		return input
	})

	graph.Node("multiply", func(d TestData) TestData {
		d.Value *= 2
		return d
	})

	graph.Node("add", func(d TestData) TestData {
		d.Value += 5
		return d
	})

	graph.Node("format", func(d TestData) string {
		return fmt.Sprintf("%d-%s", d.Value, d.Status)
	})

	graph.EndNode("end", func(s string) {
		fmt.Println("Result:", s)
	})

	graph.AddEdge("start", "multiply")
	graph.AddEdge("multiply", "add")
	graph.AddEdge("add", "format")
	graph.AddEdge("format", "end")

	err := graph.RunSequential()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	multiplyResult := graph.NodeResult("multiply")
	if len(multiplyResult) != 1 {
		t.Fatalf("Expected multiply to have 1 result, got %d", len(multiplyResult))
	}
	multiplyValue := multiplyResult[0].(TestData)
	if multiplyValue.Value != 20 {
		t.Errorf("Expected multiply result to be 20, got %d", multiplyValue.Value)
	}

	addResult := graph.NodeResult("add")
	if len(addResult) != 1 {
		t.Fatalf("Expected add to have 1 result, got %d", len(addResult))
	}
	addValue := addResult[0].(TestData)
	if addValue.Value != 25 {
		t.Errorf("Expected add result to be 25, got %d", addValue.Value)
	}
}

func TestBasicGraphCreation(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() string {
		return "start"
	})
	graph.Node("process1", func(s string) string {
		return s + " -> process1"
	})
	graph.Node("process2", func(s string) string {
		return s + " -> process2"
	})
	graph.EndNode("end", func(s string) {
		fmt.Println("Result:", s)
	})

	graph.AddEdge("start", "process1")
	graph.AddEdge("process1", "process2")
	graph.AddEdge("process2", "end")

	err := graph.RunSequential()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestGraphFindEndNodesWithEndType(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() string {
		return "start"
	})

	graph.Node("step1", func(s string) string {
		return s + " -> step1"
	})

	graph.Node("step2", func(s string) string {
		return s + " -> step2"
	})

	graph.EndNode("end", func(s string) {
		fmt.Println("Final:", s)
	})

	graph.AddEdge("start", "step1")
	graph.AddEdge("step1", "step2")
	graph.AddEdge("step2", "end")

	endNodes := graph.FindEndNodes()

	if len(endNodes) != 1 {
		t.Errorf("Expected 1 end node, got %d", len(endNodes))
	}

	if endNodes[0] != "end" {
		t.Errorf("Expected end node to be 'end', got %s", endNodes[0])
	}
}

func TestGraphFindEndNodesWithOutDegree(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() int {
		return 10
	})

	graph.Node("process1", func(a int) int {
		return a * 2
	})

	graph.Node("process2", func(b int) int {
		return b * 3
	})

	graph.AddEdge("start", "process1")
	graph.AddEdge("process1", "process2")

	endNodes := graph.FindEndNodes()

	if len(endNodes) != 1 {
		t.Errorf("Expected 1 end node, got %d", len(endNodes))
	}

	if endNodes[0] != "process2" {
		t.Errorf("Expected end node to be 'process2', got %s", endNodes[0])
	}
}

func TestGraphRunMethod(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() int {
		return 10
	})

	graph.Node("double", func(n int) int {
		return n * 2
	})

	graph.AddEdge("start", "double")

	err := graph.Run()
	if err != nil {
		t.Errorf("Expected no error from Run(), got %v", err)
	}

	doubleStatus := graph.NodeStatus("double")
	if doubleStatus != NodeStatusCompleted {
		t.Errorf("Expected double node to be completed")
	}
}

func TestGraphRunWithStrategy(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() int {
		return 10
	})

	graph.Node("double", func(n int) int {
		return n * 2
	})

	graph.AddEdge("start", "double")

	err := graph.RunWithStrategy(func() error {
		return graph.RunSequential()
	})

	if err != nil {
		t.Errorf("Expected no error from RunWithStrategy(), got %v", err)
	}

	doubleStatus := graph.NodeStatus("double")
	if doubleStatus != NodeStatusCompleted {
		t.Errorf("Expected double node to be completed")
	}
}

func TestGraphNodeError(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() int {
		return 10
	})

	graph.Node("error_step", func(n int) (int, error) {
		return 0, &ChainError{Message: "test error"}
	})

	graph.AddEdge("start", "error_step")

	err := graph.RunSequential()
	if err == nil {
		t.Errorf("Expected error from graph with error node")
	}

	nodeError := graph.NodeError("error_step")
	if nodeError == nil {
		t.Errorf("Expected node error to be recorded")
	}
}

func TestGraphNodeTypes(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() string { return "start" })
	graph.BranchNode("branch", func(s string) string { return s })
	graph.ParallelNode("parallel", func(s string) string { return s })
	graph.LoopNode("loop", func(s string) string { return s })
	graph.Node("normal", func(s string) string { return s })
	graph.EndNode("end", func(s string) {})

	graph.AddEdge("start", "branch")
	graph.AddEdge("branch", "parallel")
	graph.AddEdge("parallel", "loop")
	graph.AddEdge("loop", "normal")
	graph.AddEdge("normal", "end")

	err := graph.RunSequential()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	startStatus := graph.NodeStatus("start")
	if startStatus != NodeStatusCompleted {
		t.Errorf("Expected start node to be completed")
	}

	endStatus := graph.NodeStatus("end")
	if endStatus != NodeStatusCompleted {
		t.Errorf("Expected end node to be completed")
	}
}

func TestGraphWithMultiReturn(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() (int, string) {
		return 10, "test"
	})

	graph.Node("process", func(a int, s string) (string, int, bool) {
		return s + "-processed", a * 2, true
	})

	graph.Node("verify", func(s string, a int, b bool) string {
		if b {
			return fmt.Sprintf("%s:%d", s, a)
		}
		return "invalid"
	})

	graph.EndNode("end", func(s string) {
		fmt.Println("Final:", s)
	})

	graph.AddEdge("start", "process")
	graph.AddEdge("process", "verify")
	graph.AddEdge("verify", "end")

	err := graph.RunSequential()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	processResult := graph.NodeResult("process")
	if len(processResult) != 3 {
		t.Fatalf("Expected process to have 3 results, got %d", len(processResult))
	}
	processValue1 := processResult[0].(string)
	if processValue1 != "test-processed" {
		t.Errorf("Expected process result 0 to be 'test-processed', got %q", processValue1)
	}

	processValue2 := processResult[1].(int)
	if processValue2 != 20 {
		t.Errorf("Expected process result 1 to be 20, got %d", processValue2)
	}
}

func TestGraphWithConditionBranch(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() int {
		return 42
	})

	graph.BranchNode("branch", func(n int) int {
		return n
	})

	graph.Node("success", func(n int) string {
		return "success"
	})

	graph.Node("error", func(n int) string {
		return "error"
	})

	graph.EndNode("end", func(s string) {
		fmt.Println("Result:", s)
	})

	graph.AddEdge("start", "branch")
	graph.AddEdgeWithCondition("branch", "success", func(n int) bool { return n <= 50 })
	graph.AddEdgeWithCondition("branch", "error", func(n int) bool { return n > 50 })
	graph.AddEdge("success", "end")
	graph.AddEdge("error", "end")

	err := graph.RunSequential()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	successStatus := graph.NodeStatus("success")
	errorStatus := graph.NodeStatus("error")

	if successStatus != NodeStatusCompleted {
		t.Errorf("Expected success node to be completed")
	}

	if errorStatus != NodeStatusPending {
		t.Errorf("Expected error node to be pending, got %v", errorStatus)
	}
}

func TestGraphWithDifferentConditions(t *testing.T) {
	testCases := []struct {
		name     string
		input    int
		expected string
	}{
		{"high", 60, "high"},
		{"medium", 30, "medium"},
		{"low", 10, "low"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			graph := NewGraph()

			graph.StartNode("start", func() int {
				return tc.input
			})

			graph.BranchNode("branch", func(n int) int {
				return n
			})

			graph.Node("high", func(n int) string { return "high" })
			graph.Node("medium", func(n int) string { return "medium" })
			graph.Node("low", func(n int) string { return "low" })

			graph.AddEdge("start", "branch")
			graph.AddEdgeWithCondition("branch", "high", func(b int) bool {
				return b >= 50
			})
			graph.AddEdgeWithCondition("branch", "medium", func(b int) bool {
				return b >= 20 && b < 50
			})
			graph.AddEdgeWithCondition("branch", "low", func(b int) bool {
				return b < 20
			})

			err := graph.RunSequential()
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			highStatus := graph.NodeStatus("high")
			mediumStatus := graph.NodeStatus("medium")
			lowStatus := graph.NodeStatus("low")

			switch tc.expected {
			case "high":
				if highStatus != NodeStatusCompleted {
					t.Errorf("Expected high node to be completed")
				}
			case "medium":
				if mediumStatus != NodeStatusCompleted {
					t.Errorf("Expected medium node to be completed")
				}
			case "low":
				if lowStatus != NodeStatusCompleted {
					t.Errorf("Expected low node to be completed")
				}
			}
		})
	}
}

func TestGraphWithNoCondition(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() string {
		return "start"
	})

	graph.Node("step1", func(s string) string {
		return s + " -> step1"
	})

	graph.Node("step2", func(s string) string {
		return s + " -> step2"
	})

	graph.AddEdge("start", "step1")
	graph.AddEdge("step1", "step2")

	err := graph.RunSequential()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	step2Status := graph.NodeStatus("step2")
	if step2Status != NodeStatusCompleted {
		t.Errorf("Expected step2 node to be completed")
	}
}

func TestGraphParallelExecution(t *testing.T) {
	graph := NewGraph()

	startTime := time.Now()

	graph.StartNode("start", func() int {
		return 10
	})

	graph.ParallelNode("parallel1", func(n int) int {
		time.Sleep(100 * time.Millisecond)
		return n * 2
	})

	graph.ParallelNode("parallel2", func(n int) int {
		time.Sleep(100 * time.Millisecond)
		return n * 3
	})

	graph.Node("combine", func(a, b int) int {
		return a + b
	})

	graph.AddEdge("start", "parallel1")
	graph.AddEdge("start", "parallel2")
	graph.AddEdge("parallel1", "combine")
	graph.AddEdge("parallel2", "combine")

	err := graph.RunParallel()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	elapsed := time.Since(startTime)
	if elapsed > 150*time.Millisecond {
		t.Logf("Warning: parallel execution took %v", elapsed)
	}

	combineResult := graph.NodeResult("combine")
	if len(combineResult) != 1 {
		t.Fatalf("Expected combine to have 1 result, got %d", len(combineResult))
	}
	resultValue := combineResult[0].(int)
	if resultValue != 50 {
		t.Errorf("Expected combine result to be 50, got %d", resultValue)
	}
}

func TestGraphParallelMode(t *testing.T) {
	graph := NewGraph()

	startTime := time.Now()

	graph.StartNode("start", func() int {
		return 10
	})

	graph.ParallelNode("parallel1", func(n int) int {
		time.Sleep(100 * time.Millisecond)
		return n * 2
	})

	graph.ParallelNode("parallel2", func(n int) int {
		time.Sleep(100 * time.Millisecond)
		return n * 3
	})

	graph.Node("combine", func(a, b int) int {
		return a + b
	})

	graph.AddEdge("start", "parallel1")
	graph.AddEdge("start", "parallel2")
	graph.AddEdge("parallel1", "combine")
	graph.AddEdge("parallel2", "combine")

	err := graph.RunParallel()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	elapsed := time.Since(startTime)
	if elapsed > 150*time.Millisecond {
		t.Logf("Parallel execution took %v", elapsed)
	}

	combineResult := graph.NodeResult("combine")
	if len(combineResult) != 1 {
		t.Fatalf("Expected combine to have 1 result, got %d", len(combineResult))
	}
	resultValue := combineResult[0].(int)
	if resultValue != 50 {
		t.Errorf("Expected combine result to be 50, got %d", resultValue)
	}
}

func TestGraphErrorPropagation(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() int {
		return 10
	})

	graph.Node("process1", func(n int) int {
		return n * 2
	})

	graph.Node("error_node", func(n int) (int, error) {
		return 0, &ChainError{Message: "test error"}
	})

	graph.Node("process2", func(n int) int {
		return n + 5
	})

	graph.AddEdge("start", "process1")
	graph.AddEdge("process1", "error_node")
	graph.AddEdge("error_node", "process2")

	err := graph.RunSequential()
	if err == nil {
		t.Errorf("Expected error to be propagated")
	}

	errorNodeStatus := graph.NodeStatus("error_node")
	if errorNodeStatus != NodeStatusFailed {
		t.Errorf("Expected error node to be failed, got %v", errorNodeStatus)
	}

	process2Status := graph.NodeStatus("process2")
	if process2Status != NodeStatusPending {
		t.Errorf("Expected process2 node to be pending, got %v", process2Status)
	}
}

func TestGraphWithDuplicateNode(t *testing.T) {
	graph := NewGraph()

	graph.Node("test", func() string { return "test" })

	graph.Node("test", func() string { return "duplicate" })

	if graph.Error() == nil {
		t.Errorf("Expected error for duplicate node")
	}
}

func TestGraphWithSelfDependency(t *testing.T) {
	graph := NewGraph()

	graph.Node("test", func() string { return "test" })

	graph.AddEdge("test", "test")

	if graph.Error() == nil {
		t.Errorf("Expected error for self dependency")
	}
}

func TestGraphCyclicDependency(t *testing.T) {
	graph := NewGraph()

	graph.Node("a", func() string { return "a" })
	graph.Node("b", func() string { return "b" })
	graph.Node("c", func() string { return "c" })

	graph.AddEdge("a", "b")
	graph.AddEdge("b", "c")

	err := graph.AddEdge("c", "a")
	if err == nil {
		t.Errorf("Expected error for cyclic dependency")
	}
}

func TestGraphWithNoStartNode(t *testing.T) {
	graph := NewGraph()

	err := graph.RunSequential()
	if err == nil {
		t.Errorf("Expected error for no start node")
	}
}

func TestGraphStatusTracking(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() string { return "start" })
	graph.Node("process", func(s string) string { return s + " -> processed" })
	graph.Node("end", func(s string) {})

	graph.AddEdge("start", "process")
	graph.AddEdge("process", "end")

	startStatus := graph.NodeStatus("start")
	if startStatus != NodeStatusPending {
		t.Errorf("Expected start node to be pending")
	}

	err := graph.RunSequential()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	startStatus = graph.NodeStatus("start")
	if startStatus != NodeStatusCompleted {
		t.Errorf("Expected start node to be completed")
	}

	processStatus := graph.NodeStatus("process")
	if processStatus != NodeStatusCompleted {
		t.Errorf("Expected process node to be completed")
	}

	endStatus := graph.NodeStatus("end")
	if endStatus != NodeStatusCompleted {
		t.Errorf("Expected end node to be completed")
	}
}

func TestGraphClearStatus(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() string { return "start" })
	graph.Node("process", func(s string) string { return s })
	graph.AddEdge("start", "process")

	err := graph.RunSequential()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	startStatus := graph.NodeStatus("start")
	if startStatus != NodeStatusCompleted {
		t.Errorf("Expected start node to be completed before clear")
	}

	graph.ClearStatus()

	startStatus = graph.NodeStatus("start")
	if startStatus != NodeStatusPending {
		t.Errorf("Expected start node to be pending after clear")
	}

	processStatus := graph.NodeStatus("process")
	if processStatus != NodeStatusPending {
		t.Errorf("Expected process node to be pending after clear")
	}

	if graph.Error() != nil {
		t.Errorf("Expected error to be cleared")
	}
}

func TestGraphStringOutput(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() string { return "start" })
	graph.Node("process", func(s string) string { return s })
	graph.Node("end", func(s string) {})

	graph.AddEdge("start", "process")
	graph.AddEdge("process", "end")

	dotOutput := graph.String()
	if !strings.Contains(dotOutput, "digraph Graph {") {
		t.Errorf("Expected dot output to contain 'digraph Graph {'")
	}

	if !strings.Contains(dotOutput, "start") {
		t.Errorf("Expected dot output to contain 'start'")
	}

	if !strings.Contains(dotOutput, "process") {
		t.Errorf("Expected dot output to contain 'process'")
	}

	if !strings.Contains(dotOutput, "end") {
		t.Errorf("Expected dot output to contain 'end'")
	}
}

func TestGraphEvaluateConditionWithFuncReturnNonBool(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() int { return 10 })
	graph.Node("process", func(n int) int { return n * 2 })

	// Condition returns string, should be treated as true
	graph.AddEdgeWithCondition("start", "process", func(n int) string { return "condition" })

	err := graph.RunSequential()

	if err == nil {
		status := graph.NodeStatus("process")
		if status != NodeStatusCompleted {
			t.Errorf("Expected process node to be completed")
		}
	}
}

func TestGraphEvaluateConditionWithInterfaceReturnFalse(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() int { return 10 })
	graph.Node("process", func(n int) int { return n * 2 })

	// Condition returns false
	graph.AddEdgeWithCondition("start", "process", func(n int) bool { return false })

	err := graph.RunSequential()

	if err == nil {
		status := graph.NodeStatus("process")
		if status != NodeStatusPending {
			t.Errorf("Expected process node to be pending")
		}
	}
}

func TestGraphMermaidOutput(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() string { return "start" })
	graph.Node("process", func(s string) string { return s })
	graph.Node("end", func(s string) {})

	graph.AddEdge("start", "process")
	graph.AddEdge("process", "end")

	mermaidOutput := graph.Mermaid()
	if !strings.Contains(mermaidOutput, "graph TD") {
		t.Errorf("Expected mermaid output to contain 'graph TD'")
	}

	if !strings.Contains(mermaidOutput, "start --> process") {
		t.Errorf("Expected mermaid output to contain 'start --> process'")
	}

	if !strings.Contains(mermaidOutput, "process --> end") {
		t.Errorf("Expected mermaid output to contain 'process --> end'")
	}
}

func TestGraphWithNoOpNode(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() int { return 10 })

	graph.Node("noop", nil)

	graph.Node("end", func(n int) {
		fmt.Println("End:", n)
	})

	graph.AddEdge("start", "noop")
	graph.AddEdge("noop", "end")

	err := graph.RunSequential()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	endStatus := graph.NodeStatus("end")
	if endStatus != NodeStatusCompleted {
		t.Errorf("Expected end node to be completed")
	}
}

func TestGraphWithComplexValueTypes(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() *TestData {
		return &TestData{Value: 10, Status: "test"}
	})

	graph.Node("modify", func(d *TestData) *TestData {
		d.Value *= 2
		d.Status = "modified"
		return d
	})

	graph.Node("copy", func(d *TestData) TestData {
		return TestData{
			Value:  d.Value + 5,
			Status: d.Status + "-copied",
		}
	})

	graph.AddEdge("start", "modify")
	graph.AddEdge("modify", "copy")

	err := graph.RunSequential()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	copyResult := graph.NodeResult("copy")
	if len(copyResult) != 1 {
		t.Fatalf("Expected copy to have 1 result, got %d", len(copyResult))
	}
	resultValue := copyResult[0].(TestData)
	if resultValue.Value != 25 {
		t.Errorf("Expected copy result to be 25, got %d", resultValue.Value)
	}
	if resultValue.Status != "modified-copied" {
		t.Errorf("Expected copy status to be 'modified-copied', got %q", resultValue.Status)
	}
}

func TestGraphWithMultipleInputs(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() int {
		return 10
	})

	graph.Node("process1", func(a int) int {
		return a * 2
	})

	graph.Node("process2", func(b int) int {
		return b * 3
	})

	graph.Node("combine", func(a int) int {
		return a
	})

	graph.AddEdge("start", "process1")
	graph.AddEdge("process1", "process2")
	graph.AddEdge("process2", "combine")

	err := graph.RunSequential()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	combineResult := graph.NodeResult("combine")
	if len(combineResult) != 1 {
		t.Fatalf("Expected combine to have 1 result, got %d", len(combineResult))
	}
	resultValue := combineResult[0].(int)
	if resultValue != 60 {
		t.Errorf("Expected combine result to be 60, got %d", resultValue)
	}
}

func TestGraphWithMultipleOutputs(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() int {
		return 10
	})

	graph.Node("process", func(n int) (int, string, bool) {
		return n * 2, "test", true
	})

	// Modify the parameter order of the verify function to match the return order of the process node
	graph.Node("verify", func(a int, s string, b bool) string {
		if b {
			return fmt.Sprintf("%s-%d", s, a)
		}
		return "invalid"
	})

	graph.AddEdge("start", "process")
	graph.AddEdge("process", "verify")

	err := graph.RunSequential()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	verifyResult := graph.NodeResult("verify")
	if len(verifyResult) != 1 {
		t.Fatalf("Expected verify to have 1 result, got %d", len(verifyResult))
	}
	resultValue := verifyResult[0].(string)
	if resultValue != "test-20" {
		t.Errorf("Expected verify result to be 'test-20', got %q", resultValue)
	}
}

func TestGraphWithEdgeCondition(t *testing.T) {
	graph := NewGraph()

	input := 30

	graph.StartNode("start", func() int {
		return input
	})

	graph.BranchNode("branch", func(n int) int {
		return n
	})

	graph.Node("low", func(n int) string { return "low" })
	graph.Node("high", func(n int) string { return "high" })

	graph.AddEdge("start", "branch")
	graph.AddEdgeWithCondition("branch", "low", func(b int) bool {
		return b < 50
	})
	graph.AddEdgeWithCondition("branch", "high", func(b int) bool {
		return b >= 50
	})

	err := graph.RunSequential()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	lowStatus := graph.NodeStatus("low")
	if lowStatus != NodeStatusCompleted {
		t.Errorf("Expected low node to be completed")
	}

	highStatus := graph.NodeStatus("high")
	if highStatus != NodeStatusPending {
		t.Errorf("Expected high node to be pending")
	}
}

func TestGraphEvaluateConditionVariadic(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() (int, int) {
		return 10, 20
	})

	graph.BranchNode("branch", func(a, b int) (int, int) {
		return a, b
	})

	graph.Node("sum", func(a, b int) int { return a + b })
	graph.Node("diff", func(a, b int) int { return a - b })

	graph.AddEdge("start", "branch")
	graph.AddEdgeWithCondition("branch", "sum", func(a, b int) bool {
		return a+b > 25
	})
	graph.AddEdgeWithCondition("branch", "diff", func(a, b int) bool {
		return b-a < 15
	})

	err := graph.RunSequential()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	sumStatus := graph.NodeStatus("sum")
	if sumStatus != NodeStatusCompleted {
		t.Errorf("Expected sum node to be completed")
	}

	diffStatus := graph.NodeStatus("diff")
	if diffStatus != NodeStatusCompleted {
		t.Errorf("Expected diff node to be completed")
	}
}

func TestGraphEvaluateConditionWithDifferentArity(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() int {
		return 42
	})

	graph.BranchNode("branch", func(n int) int {
		return n
	})

	graph.Node("odd", func(n int) string { return "odd" })
	graph.Node("even", func(n int) string { return "even" })
	graph.Node("multiple", func(n int) string { return "multiple" })

	graph.AddEdge("start", "branch")
	graph.AddEdgeWithCondition("branch", "odd", func(n int) bool {
		return true
	})
	graph.AddEdgeWithCondition("branch", "even", func(n int) bool {
		return n%2 == 0
	})
	graph.AddEdgeWithCondition("branch", "multiple", func(n int) bool {
		return n%3 == 0
	})

	err := graph.RunSequential()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	oddStatus := graph.NodeStatus("odd")
	if oddStatus != NodeStatusCompleted {
		t.Errorf("Expected odd node to be completed")
	}

	evenStatus := graph.NodeStatus("even")
	if evenStatus != NodeStatusCompleted {
		t.Errorf("Expected even node to be completed")
	}

	multipleStatus := graph.NodeStatus("multiple")
	if multipleStatus != NodeStatusCompleted {
		t.Errorf("Expected multiple node to be completed")
	}
}

func TestGraphNodeNotFoundStatus(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() string { return "start" })

	status := graph.NodeStatus("nonexistent")
	if status != NodeStatusPending {
		t.Errorf("Expected NodeStatusPending for non-existent node, got %v", status)
	}
}

func TestGraphNodeNotFoundResult(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() string { return "start" })

	result := graph.NodeResult("nonexistent")
	if result != nil {
		t.Errorf("Expected nil result for non-existent node, got %v", result)
	}
}

func TestGraphNodeNotFoundError(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() string { return "start" })

	err := graph.NodeError("nonexistent")
	if err != nil {
		t.Errorf("Expected nil error for non-existent node, got %v", err)
	}
}

func TestGraphWithLoopNode(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() int {
		return 5
	})

	graph.LoopNode("loop", func(n int) int {
		return n + 1
	})

	graph.Node("check", func(n int) int {
		return n
	})

	graph.EndNode("end", func(n int) {})

	graph.AddEdge("start", "loop")
	graph.AddEdge("loop", "check")
	graph.AddEdge("check", "end")

	err := graph.RunSequential()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	endStatus := graph.NodeStatus("end")
	if endStatus != NodeStatusCompleted {
		t.Errorf("Expected end node to be completed")
	}
}

func TestGraphAddEdgeWithConditionMissingNode(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() string { return "start" })

	graph.AddEdgeWithCondition("start", "nonexistent", nil)
	if graph.Error() == nil {
		t.Errorf("Expected error when adding edge to non-existent node")
	}

	if graph.Error() != nil {
		ce, ok := graph.Error().(*ChainError)
		if !ok {
			t.Errorf("Expected *ChainError, got %T", graph.Error())
		} else if ce.Message == "" {
			t.Errorf("Expected error message to not be empty")
		} else if !strings.Contains(ce.Message, ErrNodeNotFound) {
			t.Errorf("Expected error message containing '%s', got '%s'", ErrNodeNotFound, ce.Message)
		}
	}
}

func TestGraphAddEdgeFromMissingNode(t *testing.T) {
	graph := NewGraph()

	graph.Node("end", func(s string) {})

	graph.AddEdgeWithCondition("nonexistent", "end", nil)
	if graph.Error() == nil {
		t.Errorf("Expected error when adding edge from non-existent node")
	}

	if graph.Error() != nil {
		ce, ok := graph.Error().(*ChainError)
		if !ok {
			t.Errorf("Expected *ChainError, got %T", graph.Error())
		} else if ce.Message == "" {
			t.Errorf("Expected error message to not be empty")
		} else if !strings.Contains(ce.Message, ErrNodeNotFound) {
			t.Errorf("Expected error message containing '%s', got '%s'", ErrNodeNotFound, ce.Message)
		}
	}
}

func TestGraphFindStartNodeWithZeroInDegree(t *testing.T) {
	graph := NewGraph()

	graph.Node("a", func() string { return "a" })
	graph.Node("b", func(s string) string { return s })

	graph.AddEdge("a", "b")

	startNode := graph.FindStartNode()
	if startNode != "a" {
		t.Errorf("Expected 'a' as start node with zero in-degree, got '%s'", startNode)
	}
}

func TestGraphBuildExecutionPlanWithMultipleStarts(t *testing.T) {
	graph := NewGraph()

	graph.Node("a", func() string { return "a" })
	graph.Node("b", func() string { return "b" })
	graph.Node("c", func(a, b string) string { return a + b })

	graph.AddEdge("a", "c")
	graph.AddEdge("b", "c")

	plan, err := graph.buildExecutionPlan()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(plan) != 3 {
		t.Errorf("Expected plan with 3 nodes, got %d", len(plan))
	}
}

func TestGraphRunParallelWithError(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() int {
		return 10
	})

	graph.ParallelNode("parallel1", func(n int) int {
		return n * 2
	})

	graph.ParallelNode("parallel2", func(n int) (int, error) {
		return 0, &ChainError{Message: "test error"}
	})

	graph.Node("combine", func(a int) int {
		return a
	})

	graph.AddEdge("start", "parallel1")
	graph.AddEdge("start", "parallel2")
	graph.AddEdge("parallel1", "combine")

	err := graph.RunParallel()
	if err == nil {
		t.Errorf("Expected error from implementation")
	}
}

func TestGraphRunParallelWithMixedNodeTypes(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() int {
		return 10
	})

	graph.ParallelNode("parallel1", func(n int) int {
		return n * 2
	})

	graph.ParallelNode("parallel2", func(n int) int {
		return n * 3
	})

	graph.BranchNode("branch", func(a, b int) int {
		return a + b
	})

	graph.Node("end", func(n int) {})

	graph.AddEdge("start", "parallel1")
	graph.AddEdge("start", "parallel2")
	graph.AddEdge("parallel1", "branch")
	graph.AddEdge("parallel2", "branch")
	graph.AddEdge("branch", "end")

	err := graph.RunParallel()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	endStatus := graph.NodeStatus("end")
	if endStatus != NodeStatusCompleted {
		t.Errorf("Expected end node to be completed")
	}
}

func TestGraphMermaidOutputComplete(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() string { return "start" })
	graph.Node("process", func(s string) string { return s })
	graph.EndNode("end", func(s string) {})

	graph.AddEdge("start", "process")
	graph.AddEdge("process", "end")

	mermaidOutput := graph.Mermaid()
	if mermaidOutput == "" {
		t.Errorf("Expected mermaid output to not be empty")
	}

	if !strings.Contains(mermaidOutput, "start --> process") {
		t.Errorf("Expected mermaid output to contain 'start --> process'")
	}

	if !strings.Contains(mermaidOutput, "process --> end") {
		t.Errorf("Expected mermaid output to contain 'process --> end'")
	}

	if !strings.Contains(mermaidOutput, "graph TD") {
		t.Errorf("Expected mermaid output to contain 'graph TD'")
	}
}

func TestGraphMermaidOutputWithCondition(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() int { return 10 })
	graph.BranchNode("branch", func(n int) int { return n })
	graph.Node("low", func() string { return "low" })
	graph.Node("high", func() string { return "high" })

	graph.AddEdge("start", "branch")
	graph.AddEdgeWithCondition("branch", "low", func(n int) bool {
		return n < 50
	})
	graph.AddEdgeWithCondition("branch", "high", func(n int) bool {
		return n >= 50
	})

	mermaidOutput := graph.Mermaid()
	if !strings.Contains(mermaidOutput, "|cond|") {
		t.Errorf("Expected mermaid output to contain '|cond|' label for conditional edges")
	}
}

func TestGraphEvaluateConditionWithFalseCondition(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() int {
		return 10
	})

	graph.BranchNode("branch", func(n int) int {
		return n
	})

	graph.Node("trueBranch", func(n int) string { return "true" })
	graph.Node("falseBranch", func(n int) string { return "false" })

	graph.AddEdge("start", "branch")
	graph.AddEdgeWithCondition("branch", "trueBranch", func(n int) bool {
		return n > 5
	})
	graph.AddEdgeWithCondition("branch", "falseBranch", func(n int) bool {
		return n < 5
	})

	err := graph.RunSequential()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	trueBranchStatus := graph.NodeStatus("trueBranch")
	if trueBranchStatus != NodeStatusCompleted {
		t.Errorf("Expected trueBranch node to be completed")
	}

	falseBranchStatus := graph.NodeStatus("falseBranch")
	if falseBranchStatus != NodeStatusPending {
		t.Errorf("Expected falseBranch node to be pending")
	}
}

func TestGraphFindStartNodeNoStartNode(t *testing.T) {
	graph := NewGraph()

	// Test case for empty graph
	startNode := graph.FindStartNode()
	if startNode != "" {
		t.Errorf("Expected empty string for empty graph, got '%s'", startNode)
	}
}

func TestGraphFindStartNodeWithZeroInDegreeNodes(t *testing.T) {
	graph := NewGraph()

	graph.Node("a", func() string { return "a" })
	graph.Node("b", func() string { return "b" })

	startNode := graph.FindStartNode()
	// The order of map iteration is not deterministic, so the result could be either 'a' or 'b'
	if startNode != "a" && startNode != "b" {
		t.Errorf("Expected node with zero in-degree (a or b), got '%s'", startNode)
	}
}

func TestGraphRunSequentialWithExistingError(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() string { return "start" })
	graph.Node("duplicate", func() string { return "duplicate" })
	graph.Node("duplicate", func() string { return "duplicate2" })

	err := graph.RunSequential()
	if err == nil {
		t.Errorf("Expected error to be returned from RunSequential")
	}
}

func TestGraphRunParallelWithExistingError(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() string { return "start" })
	graph.Node("duplicate", func() string { return "duplicate" })
	graph.Node("duplicate", func() string { return "duplicate2" })

	err := graph.RunParallel()
	if err == nil {
		t.Errorf("Expected error to be returned from RunParallel")
	}
}

func TestGraphRunParallelWithContextExistingError(t *testing.T) {
	graph := NewGraph()
	ctx := context.Background()

	graph.StartNode("start", func() string { return "start" })
	graph.Node("duplicate", func() string { return "duplicate" })
	graph.Node("duplicate", func() string { return "duplicate2" })

	err := graph.RunParallelWithContext(ctx)
	if err == nil {
		t.Errorf("Expected error to be returned from RunParallelWithContext")
	}
}

func TestGraphRunWithStrategyExistingError(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() string { return "start" })
	graph.Node("duplicate", func() string { return "duplicate" })
	graph.Node("duplicate", func() string { return "duplicate2" })

	err := graph.RunWithStrategy(func() error {
		return graph.RunSequential()
	})
	if err == nil {
		t.Errorf("Expected error to be returned from RunWithStrategy")
	}
}

func TestGraphEvaluateCondition(t *testing.T) {
	graph := NewGraph()

	// Test case 1: nil condition
	result := graph.evaluateCondition(nil, []any{10})
	if !result {
		t.Error("Expected nil condition to return true")
	}

	// Test case 2: boolean condition
	result = graph.evaluateCondition(true, []any{10})
	if !result {
		t.Error("Expected true condition to return true")
	}

	result = graph.evaluateCondition(false, []any{10})
	if result {
		t.Error("Expected false condition to return false")
	}

	// Test case 3: function condition with single parameter
	result = graph.evaluateCondition(func(x int) bool {
		return x > 5
	}, []any{10})
	if !result {
		t.Error("Expected function condition to return true")
	}

	// Test case 4: function condition with multiple parameters
	result = graph.evaluateCondition(func(x, y int) bool {
		return x+y > 15
	}, []any{10, 6})
	if !result {
		t.Error("Expected function condition with multiple parameters to return true")
	}

	// Test case 5: function condition with variadic parameters
	result = graph.evaluateCondition(func(nums ...int) bool {
		sum := 0
		for _, num := range nums {
			sum += num
		}
		return sum > 10
	}, []any{10, 20, 30})
	if !result {
		t.Error("Expected variadic function condition to return true")
	}

	// Test case 6: function condition with no parameters
	result = graph.evaluateCondition(func() bool {
		return true
	}, []any{10})
	if !result {
		t.Error("Expected function condition with no parameters to return true")
	}

	// Test case 7: function condition returning non-boolean
	result = graph.evaluateCondition(func() int {
		return 10
	}, []any{10})
	if !result {
		t.Error("Expected function condition returning non-boolean to return true")
	}

	// Test case 8: function condition with more parameters than results
	result = graph.evaluateCondition(func(x, y, z int) bool {
		return x+y+z > 0
	}, []any{10})
	if !result {
		t.Error("Expected function condition with more parameters to return true")
	}

	// Test case 9: function condition with fewer parameters than results
	result = graph.evaluateCondition(func(x int) bool {
		return x > 5
	}, []any{10, 20, 30})
	if !result {
		t.Error("Expected function condition with fewer parameters to return true")
	}

	// Test case 10: function condition with interface return type
	result = graph.evaluateCondition(func() interface{} {
		return true
	}, []any{10})
	if !result {
		t.Error("Expected function condition with interface return to return true")
	}

	// Test case 11: function condition with nil interface return
	result = graph.evaluateCondition(func() interface{} {
		return nil
	}, []any{10})
	if !result {
		t.Error("Expected function condition with nil interface return to return true")
	}

	// Test case 12: non-function, non-boolean condition
	result = graph.evaluateCondition("test", []any{10})
	if !result {
		t.Error("Expected non-function, non-boolean condition to return true")
	}

	// Test case 13: function condition with no results
	result = graph.evaluateCondition(func() bool {
		return true
	}, nil)
	if !result {
		t.Error("Expected function condition with no results to return true")
	}

	// Test case 14: function condition with no results and no parameters
	result = graph.evaluateCondition(func() bool {
		return true
	}, nil)
	if !result {
		t.Error("Expected function condition with no results and no parameters to return true")
	}
}

func TestGraphExecuteNodeNonFunction(t *testing.T) {
	graph := NewGraph()
	graph.Node("test", "not a function")

	_, err := graph.executeNode("test", []any{10})
	if err == nil {
		t.Error("Expected error when executing non-function node")
	}
}

func TestGraphExecuteNodeNilInputs(t *testing.T) {
	graph := NewGraph()
	graph.Node("test", func() int {
		return 42
	})

	result, err := graph.executeNode("test", nil)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(result) != 1 || result[0].(int) != 42 {
		t.Errorf("Expected result [42], got %v", result)
	}
}

func TestGraphExecuteNodeNotFound(t *testing.T) {
	graph := NewGraph()

	_, err := graph.executeNode("nonexistent", []any{10})
	if err == nil {
		t.Error("Expected error when executing non-existent node")
	}
}

func TestGraphExecuteNodeNilFunction(t *testing.T) {
	graph := NewGraph()
	graph.Node("test", nil)

	result, err := graph.executeNode("test", []any{10})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(result) != 1 || result[0].(int) != 10 {
		t.Errorf("Expected result [10], got %v", result)
	}
}

func TestGraphStringOutputWithAllNodeTypes(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() string { return "start" })
	graph.BranchNode("branch", func(s string) string { return s })
	graph.ParallelNode("parallel", func(s string) string { return s })
	graph.LoopNode("loop", func(s string) string { return s })
	graph.Node("normal", func(s string) string { return s })
	graph.EndNode("end", func(s string) {})

	graph.AddEdge("start", "branch")
	graph.AddEdge("branch", "parallel")
	graph.AddEdge("parallel", "loop")
	graph.AddEdge("loop", "normal")
	graph.AddEdge("normal", "end")

	dotOutput := graph.String()
	if dotOutput == "" {
		t.Error("Expected non-empty dot output")
	}
	if !strings.Contains(dotOutput, "digraph Graph {") {
		t.Error("Expected dot output to contain 'digraph Graph {'")
	}
	if !strings.Contains(dotOutput, "start") {
		t.Error("Expected dot output to contain 'start'")
	}
	if !strings.Contains(dotOutput, "branch") {
		t.Error("Expected dot output to contain 'branch'")
	}
	if !strings.Contains(dotOutput, "parallel") {
		t.Error("Expected dot output to contain 'parallel'")
	}
	if !strings.Contains(dotOutput, "loop") {
		t.Error("Expected dot output to contain 'loop'")
	}
	if !strings.Contains(dotOutput, "normal") {
		t.Error("Expected dot output to contain 'normal'")
	}
	if !strings.Contains(dotOutput, "end") {
		t.Error("Expected dot output to contain 'end'")
	}
}
