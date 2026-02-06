package flow

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

type TestData struct {
	Value  int
	Status string
}

func (t *TestData) String() string {
	return fmt.Sprintf("TestData{%d, %q}", t.Value, t.Status)
}

func TestGraphRunWithContext(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		graph := NewGraph()
		ctx := context.Background()

		graph.AddNode("start", func() int { return 10 })
		graph.AddNode("parallel1", func(n int) int { return n * 2 })
		graph.AddNode("parallel2", func(n int) int { return n * 3 })
		graph.AddNode("combine", func(a, b int) int { return a + b })

		graph.AddEdge("start", "parallel1")
		graph.AddEdge("start", "parallel2")
		graph.AddEdge("parallel1", "combine")
		graph.AddEdge("parallel2", "combine")

		assertNoError(t, graph.RunWithContext(ctx))
		assertNodeResult(t, graph, "combine", 50)
	})

	t.Run("Canceled", func(t *testing.T) {
		graph := NewGraph()
		ctx, cancel := context.WithCancel(context.Background())

		graph.AddNode("start", func() int { return 10 })
		graph.AddNode("slow", func(n int) int {
			time.Sleep(100 * time.Millisecond)
			return n * 2
		})
		graph.AddEdge("start", "slow")

		cancel()

		err := graph.RunWithContext(ctx)
		if err == nil {
			t.Fatalf("Expected context canceled error")
		}
		if !strings.Contains(err.Error(), "execution canceled") {
			t.Errorf("Expected canceled error, got %v", err.Error())
		}
	})
}

func TestGraphValuePropagation(t *testing.T) {
	graph := NewGraph()

	input := TestData{Value: 10, Status: "input"}

	graph.AddNode("start", func() TestData { return input })
	graph.AddNode("multiply", func(d TestData) TestData { d.Value *= 2; return d })
	graph.AddNode("add", func(d TestData) TestData { d.Value += 5; return d })
	graph.AddNode("format", func(d TestData) string { return fmt.Sprintf("%d-%s", d.Value, d.Status) })
	graph.AddNode("end", func(s string) {})

	graph.AddEdge("start", "multiply")
	graph.AddEdge("multiply", "add")
	graph.AddEdge("add", "format")
	graph.AddEdge("format", "end")

	runGraphSequential(t, graph)

	multiplyValue := graph.NodeResult("multiply")[0].(TestData)
	assertEqual(t, 20, multiplyValue.Value)

	addValue := graph.NodeResult("add")[0].(TestData)
	assertEqual(t, 25, addValue.Value)
}

func TestBasicGraphCreation(t *testing.T) {
	graph := NewGraph()

	graph.AddNode("start", func() string { return "start" })
	graph.AddNode("process1", func(s string) string { return s + " -> process1" })
	graph.AddNode("process2", func(s string) string { return s + " -> process2" })
	graph.AddNode("end", func(s string) {})

	graph.AddEdge("start", "process1")
	graph.AddEdge("process1", "process2")
	graph.AddEdge("process2", "end")

	runGraphSequential(t, graph)
}

func TestGraphRunMethods(t *testing.T) {
	t.Run("Run", func(t *testing.T) {
		graph := createSimpleLinearGraph(t)
		assertNoError(t, graph.RunWithContext(context.Background()))
		assertNodeStatus(t, graph, "double", NodeStatusCompleted)
	})
}

func TestGraphNodeError(t *testing.T) {
	graph := NewGraph()

	graph.AddNode("start", func() int { return 10 })
	graph.AddNode("error_step", func(n int) (int, error) {
		return 0, &ChainError{Message: "test error"}
	})

	graph.AddEdge("start", "error_step")

	assertError(t, graph.RunWithContext(context.Background()), "Expected error from graph with error node")
	assertError(t, graph.NodeError("error_step"), "Expected node error to be recorded")
}

func TestGraphNodeTypes(t *testing.T) {
	graph := NewGraph()

	graph.AddNode("start", func() string { return "start" })
	graph.AddNode("branch", func(s string) string { return s })
	graph.AddNode("parallel", func(s string) string { return s })
	graph.AddNode("normal", func(s string) string { return s })
	graph.AddNode("end", func(s string) {})

	graph.AddEdge("start", "branch")
	graph.AddEdge("branch", "parallel")
	graph.AddEdge("parallel", "normal")
	graph.AddEdge("normal", "end")

	runGraphSequential(t, graph)
	assertNodeStatus(t, graph, "start", NodeStatusCompleted)
	assertNodeStatus(t, graph, "end", NodeStatusCompleted)
}

func TestGraphWithMultiReturn(t *testing.T) {
	graph := NewGraph()

	graph.AddNode("start", func() (int, string) { return 10, "test" })
	graph.AddNode("process", func(a int, s string) (string, int, bool) { return s + "-processed", a * 2, true })
	graph.AddNode("verify", func(s string, a int, b bool) string {
		if b {
			return fmt.Sprintf("%s:%d", s, a)
		}
		return "invalid"
	})
	graph.AddNode("end", func(s string) { fmt.Println("Final:", s) })

	graph.AddEdge("start", "process")
	graph.AddEdge("process", "verify")
	graph.AddEdge("verify", "end")

	runGraphSequential(t, graph)

	processResult := graph.NodeResult("process")
	assertEqual(t, 3, len(processResult))
	assertEqual(t, "test-processed", processResult[0].(string))
	assertEqual(t, 20, processResult[1].(int))
}

func TestGraphWithConditionBranch(t *testing.T) {
	t.Run("SimpleBranch", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() int { return 42 })
		graph.AddNode("branch", func(n int) int { return n })
		graph.AddNode("success", func(n int) string { return "success" })
		graph.AddNode("error", func(n int) string { return "error" })
		graph.AddNode("end", func(s string) {})

		graph.AddEdge("start", "branch")
		graph.AddEdgeWithCondition("branch", "success", func(n int) bool { return n <= 50 })
		graph.AddEdgeWithCondition("branch", "error", func(n int) bool { return n > 50 })
		graph.AddEdge("success", "end")
		graph.AddEdge("error", "end")

		runGraphSequential(t, graph)
		assertNodeStatus(t, graph, "success", NodeStatusCompleted)
		assertNodeStatus(t, graph, "error", NodeStatusPending)
	})

	t.Run("MultipleConditions", func(t *testing.T) {
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
				graph.AddNode("start", func() int { return tc.input })
				graph.AddNode("branch", func(n int) int { return n })
				graph.AddNode("high", func(n int) string { return "high" })
				graph.AddNode("medium", func(n int) string { return "medium" })
				graph.AddNode("low", func(n int) string { return "low" })

				graph.AddEdge("start", "branch")
				graph.AddEdgeWithCondition("branch", "high", func(b int) bool { return b >= 50 })
				graph.AddEdgeWithCondition("branch", "medium", func(b int) bool { return b >= 20 && b < 50 })
				graph.AddEdgeWithCondition("branch", "low", func(b int) bool { return b < 20 })

				err := graph.RunWithContext(context.Background())
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
	})
}

func TestGraphWithNoCondition(t *testing.T) {
	graph := NewGraph()

	graph.AddNode("start", func() string {
		return "start"
	})

	graph.AddNode("step1", func(s string) string {
		return s + " -> step1"
	})

	graph.AddNode("step2", func(s string) string {
		return s + " -> step2"
	})

	graph.AddEdge("start", "step1")
	graph.AddEdge("step1", "step2")

	err := graph.RunWithContext(context.Background())
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

	graph.AddNode("start", func() int {
		return 10
	})

	graph.AddNode("parallel1", func(n int) int {
		time.Sleep(100 * time.Millisecond)
		return n * 2
	})

	graph.AddNode("parallel2", func(n int) int {
		time.Sleep(100 * time.Millisecond)
		return n * 3
	})

	graph.AddNode("combine", func(a, b int) int {
		return a + b
	})

	graph.AddEdge("start", "parallel1")
	graph.AddEdge("start", "parallel2")
	graph.AddEdge("parallel1", "combine")
	graph.AddEdge("parallel2", "combine")

	err := graph.RunWithContext(context.Background())
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

func TestGraphErrorPropagation(t *testing.T) {
	graph := NewGraph()

	graph.AddNode("start", func() int {
		return 10
	})

	graph.AddNode("process1", func(n int) int {
		return n * 2
	})

	graph.AddNode("error_node", func(n int) (int, error) {
		return 0, &ChainError{Message: "test error"}
	})

	graph.AddNode("process2", func(n int) int {
		return n + 5
	})

	graph.AddEdge("start", "process1")
	graph.AddEdge("process1", "error_node")
	graph.AddEdge("error_node", "process2")

	err := graph.RunWithContext(context.Background())
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

func TestGraphAddNodeWithError(t *testing.T) {
	t.Run("DuplicateNode", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("test", func() string { return "test" })
		graph.AddNode("test", func() string { return "duplicate" })

		if graph.Error() == nil {
			t.Error("Expected error for duplicate node")
		}
	})

	t.Run("ErrorPreserved", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("test", func() int { return 1 })
		graph.AddNode("test", func() int { return 2 })

		if graph.Error() == nil {
			t.Error("Expected error for duplicate node")
		}

		graph.AddNode("another", func() int { return 3 })

		if graph.Error() == nil {
			t.Error("Expected error to be preserved")
		}
	})
}

func TestGraphAddEdgeErrors(t *testing.T) {
	t.Run("MissingNode", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() int { return 10 })
		err := graph.AddEdgeWithCondition("start", "nonexistent", func(n int) bool { return true })
		if err == nil {
			t.Error("Expected error for missing node")
		}
	})

	t.Run("FromMissingNode", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("end", func() int { return 10 })
		err := graph.AddEdge("nonexistent", "end")
		if err == nil {
			t.Error("Expected error for missing node")
		}
	})

	t.Run("SelfDependency", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("test", func() string { return "test" })
		graph.AddEdge("test", "test")
		if graph.Error() == nil {
			t.Error("Expected error for self dependency")
		}
	})
}

func TestGraphWithNoStartNode(t *testing.T) {
	graph := NewGraph()

	err := graph.RunWithContext(context.Background())
	if err == nil {
		t.Errorf("Expected error for no start node")
	}
}

func TestGraphStatusTracking(t *testing.T) {
	graph := NewGraph()

	graph.AddNode("start", func() string { return "start" })
	graph.AddNode("process", func(s string) string { return s + " -> processed" })
	graph.AddNode("end", func(s string) {})

	graph.AddEdge("start", "process")
	graph.AddEdge("process", "end")

	startStatus := graph.NodeStatus("start")
	if startStatus != NodeStatusPending {
		t.Errorf("Expected start node to be pending")
	}

	err := graph.RunWithContext(context.Background())
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

	graph.AddNode("start", func() string { return "start" })
	graph.AddNode("process", func(s string) string { return s })
	graph.AddEdge("start", "process")

	err := graph.RunWithContext(context.Background())
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

	graph.AddNode("start", func() string { return "start" })
	graph.AddNode("process", func(s string) string { return s })
	graph.AddNode("end", func(s string) {})

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

func TestGraphEvaluateCondition(t *testing.T) {
	testCases := []struct {
		name           string
		condition      any
		expectComplete bool
	}{
		{"BoolTrue", true, true},
		{"BoolFalse", false, false},
		{"StringValue", "condition", true},
		{"FuncReturnBool", func(n int) bool { return true }, true},
		{"FuncReturnBoolFalse", func(n int) bool { return false }, false},
		{"FuncReturnString", func(n int) string { return "condition" }, true},
		{"VariadicFunc", func(args ...int) bool { return true }, true},
		{"MultiArgFunc", func(a, b int) bool { return a+b > 15 }, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			graph := NewGraph()

			graph.AddNode("start", func() (int, int) { return 10, 20 })
			graph.AddNode("step1", func(a, b int) int { return a + b })

			graph.AddEdgeWithCondition("start", "step1", tc.condition)

			err := graph.RunWithContext(context.Background())
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			status := graph.NodeStatus("step1")
			if tc.expectComplete && status != NodeStatusCompleted {
				t.Errorf("Expected step1 to be completed")
			}
			if !tc.expectComplete && status != NodeStatusPending {
				t.Errorf("Expected step1 to be pending")
			}
		})
	}
}

func TestGraphMermaidOutput(t *testing.T) {
	t.Run("BasicEdges", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() string { return "start" })
		graph.AddNode("process", func(s string) string { return s })
		graph.AddNode("end", func(s string) {})
		graph.AddEdge("start", "process")
		graph.AddEdge("process", "end")

		mermaidOutput := graph.Mermaid()
		assertContains(t, mermaidOutput, "graph TD")
		assertContains(t, mermaidOutput, "start --> process")
		assertContains(t, mermaidOutput, "process --> end")
	})

	t.Run("NoEdges", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() {})
		graph.AddNode("end", func() {})

		mermaidOutput := graph.Mermaid()
		assertContains(t, mermaidOutput, "graph TD")
		assertContains(t, mermaidOutput, "start")
	})

	t.Run("WithCondition", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() int { return 10 })
		graph.AddNode("step1", func(n int) int { return n * 2 })
		graph.AddNode("step2", func(n int) int { return n * 3 })
		graph.AddEdge("start", "step1")
		graph.AddEdgeWithCondition("start", "step2", func(n int) bool { return n > 5 })

		mermaidOutput := graph.Mermaid()
		assertContains(t, mermaidOutput, "graph TD")
		assertContains(t, mermaidOutput, "start --> step1")
		assertContains(t, mermaidOutput, "start --> |cond|step2")
	})
}

func TestGraphWithNoOpNode(t *testing.T) {
	graph := NewGraph()

	graph.AddNode("start", func() int { return 10 })

	graph.AddNode("noop", nil)

	graph.AddNode("end", func(n int) {
		fmt.Println("End:", n)
	})

	graph.AddEdge("start", "noop")
	graph.AddEdge("noop", "end")

	err := graph.RunWithContext(context.Background())
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

	graph.AddNode("start", func() *TestData {
		return &TestData{Value: 10, Status: "test"}
	})

	graph.AddNode("modify", func(d *TestData) *TestData {
		d.Value *= 2
		d.Status = "modified"
		return d
	})

	graph.AddNode("copy", func(d *TestData) TestData {
		return TestData{
			Value:  d.Value + 5,
			Status: d.Status + "-copied",
		}
	})

	graph.AddEdge("start", "modify")
	graph.AddEdge("modify", "copy")

	err := graph.RunWithContext(context.Background())
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

	graph.AddNode("start", func() int {
		return 10
	})

	graph.AddNode("process1", func(a int) int {
		return a * 2
	})

	graph.AddNode("process2", func(b int) int {
		return b * 3
	})

	graph.AddNode("combine", func(a int) int {
		return a
	})

	graph.AddEdge("start", "process1")
	graph.AddEdge("process1", "process2")
	graph.AddEdge("process2", "combine")

	err := graph.RunWithContext(context.Background())
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

	graph.AddNode("start", func() int {
		return 10
	})

	graph.AddNode("process", func(n int) (int, int) {
		return n * 2, n * 3
	})

	graph.AddNode("combine", func(a, b int) int {
		return a + b
	})

	graph.AddEdge("start", "process")
	graph.AddEdge("process", "combine")

	err := graph.RunWithContext(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	combineResult := graph.NodeResult("combine")
	if len(combineResult) != 1 {
		t.Fatalf("Expected combine to have 1 result, got %d", len(combineResult))
	}
	if combineResult[0].(int) != 50 {
		t.Errorf("Expected combine result to be 50, got %d", combineResult[0].(int))
	}
}

func TestGraphWithEdgeCondition(t *testing.T) {
	graph := NewGraph()

	graph.AddNode("start", func() int {
		return 10
	})

	graph.AddNode("process1", func(n int) int {
		return n * 2
	})

	graph.AddNode("process2", func(n int) int {
		return n * 3
	})

	graph.AddNode("end", func(a, b int) int {
		return a + b
	})

	graph.AddEdge("start", "process1")
	graph.AddEdgeWithCondition("start", "process2", func(n int) bool {
		return n > 5
	})
	graph.AddEdge("process1", "end")
	graph.AddEdge("process2", "end")

	err := graph.RunWithContext(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	endResult := graph.NodeResult("end")
	if len(endResult) != 1 {
		t.Fatalf("Expected end to have 1 result, got %d", len(endResult))
	}
	if endResult[0].(int) != 50 {
		t.Errorf("Expected end result to be 50, got %d", endResult[0].(int))
	}
}

func TestGraphRunWithError(t *testing.T) {
	graph := NewGraph()

	graph.AddNode("start", func() int { return -1 })
	graph.AddNode("error_step", func(n int) (int, error) {
		if n < 0 {
			return 0, &ChainError{Message: "negative number"}
		}
		return n, nil
	})

	graph.AddEdge("start", "error_step")

	err := graph.RunWithContext(context.Background())
	if err == nil {
		t.Fatal("Expected error")
	}
}

func TestGraphRunWithMixedNodeTypes(t *testing.T) {
	graph := NewGraph()

	graph.AddNode("start", func() int { return 10 })
	graph.AddNode("parallel1", func(n int) int { return n * 2 })
	graph.AddNode("parallel2", func(n int) int { return n * 3 })
	graph.AddNode("combine", func(a, b int) int { return a + b })
	graph.AddNode("end", func(n int) {})

	graph.AddEdge("start", "parallel1")
	graph.AddEdge("start", "parallel2")
	graph.AddEdge("parallel1", "combine")
	graph.AddEdge("parallel2", "combine")
	graph.AddEdge("combine", "end")

	err := graph.RunWithContext(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result := graph.NodeResult("combine")
	if len(result) != 1 || result[0].(int) != 50 {
		t.Errorf("Expected [50], got: %v", result)
	}
}

func TestGraphRunWithExistingError(t *testing.T) {
	testCases := []struct {
		name    string
		runFunc func(graph *Graph) error
	}{
		{"Sequential", func(g *Graph) error { return g.Run() }},
		{"Parallel", func(g *Graph) error { return g.Run() }},
		{"ParallelWithContext", func(g *Graph) error { return g.RunWithContext(context.Background()) }},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			graph := NewGraph()
			graph.err = fmt.Errorf("existing error")

			err := tc.runFunc(graph)
			if err == nil {
				t.Fatal("Expected error")
			}
		})
	}
}

func TestGraphExecuteNode(t *testing.T) {
	t.Run("NonFunction", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", "not a function")

		err := graph.RunWithContext(context.Background())
		if err == nil {
			t.Fatal("Expected error")
		}
	})

	t.Run("NilFunction", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", nil)
		graph.AddNode("end", func() {})
		graph.AddEdge("start", "end")

		err := graph.RunWithContext(context.Background())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		graph := NewGraph()
		_, err := graph.executeNode("nonexistent", nil)
		if err == nil {
			t.Fatal("Expected error")
		}
	})

	t.Run("WithErrorReturn", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() (int, error) {
			return 0, fmt.Errorf("test error")
		})

		err := graph.RunWithContext(context.Background())
		if err == nil {
			t.Fatal("Expected error")
		}
	})

	t.Run("ArgTypeMismatch", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() string { return "not an int" })
		graph.AddNode("end", func(n int) int { return n })
		graph.AddEdge("start", "end")

		err := graph.RunWithContext(context.Background())
		if err == nil {
			t.Fatal("Expected error")
		}
	})

	t.Run("WithSliceInput", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() []int { return []int{1, 2, 3} })
		graph.AddNode("step1", func(nums []int) int {
			sum := 0
			for _, n := range nums {
				sum += n
			}
			return sum
		})
		graph.AddEdge("start", "step1")

		err := graph.RunWithContext(context.Background())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		result := graph.NodeResult("step1")
		if len(result) != 1 || result[0].(int) != 6 {
			t.Errorf("Expected [6], got: %v", result)
		}
	})

	t.Run("WithMultipleReturns", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() (int, string) { return 10, "hello" })
		graph.AddNode("step1", func(n int, s string) string {
			return fmt.Sprintf("%s: %d", s, n)
		})
		graph.AddEdge("start", "step1")

		err := graph.RunWithContext(context.Background())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		result := graph.NodeResult("step1")
		if len(result) != 1 || result[0].(string) != "hello: 10" {
			t.Errorf("Expected [hello: 10], got: %v", result)
		}
	})
}

func TestGraphBuildExecutionPlan(t *testing.T) {
	t.Run("Cached", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() int { return 10 })
		graph.AddNode("step1", func(n int) int { return n * 2 })
		graph.AddNode("end", func(n int) {})

		graph.AddEdge("start", "step1")
		graph.AddEdge("step1", "end")

		plan1, err := graph.buildExecutionPlan()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		plan2, err := graph.buildExecutionPlan()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if len(plan1) != len(plan2) {
			t.Errorf("Expected plans to have same length")
		}
	})

	t.Run("CyclicDependency", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("a", func() int { return 1 })
		graph.AddNode("b", func() int { return 2 })
		graph.AddNode("c", func() int { return 3 })

		graph.AddEdge("a", "b")
		graph.AddEdge("b", "c")
		graph.AddEdge("c", "a")

		err := graph.RunWithContext(context.Background())
		if err == nil {
			t.Fatal("Expected error for cyclic dependency")
		}
	})

	t.Run("WithMultipleStartNodes", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start1", func() int { return 10 })
		graph.AddNode("start2", func() int { return 20 })
		graph.AddNode("combine", func(a, b int) int { return a + b })
		graph.AddEdge("start1", "combine")
		graph.AddEdge("start2", "combine")

		err := graph.RunWithContext(context.Background())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		result := graph.NodeResult("combine")
		if len(result) != 1 || result[0].(int) != 30 {
			t.Errorf("Expected [30], got: %v", result)
		}
	})

	t.Run("WithBranching", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() int { return 10 })
		graph.AddNode("branch1", func(n int) int { return n * 2 })
		graph.AddNode("branch2", func(n int) int { return n * 3 })
		graph.AddNode("combine", func(a, b int) int { return a + b })
		graph.AddEdge("start", "branch1")
		graph.AddEdge("start", "branch2")
		graph.AddEdge("branch1", "combine")
		graph.AddEdge("branch2", "combine")

		err := graph.RunWithContext(context.Background())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		result := graph.NodeResult("combine")
		if len(result) != 1 || result[0].(int) != 50 {
			t.Errorf("Expected [50], got: %v", result)
		}
	})
}

func TestGraphHasCycle(t *testing.T) {
	t.Run("True", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("a", func() int { return 1 })
		graph.AddNode("b", func() int { return 2 })
		graph.AddNode("c", func() int { return 3 })

		graph.AddEdge("a", "b")
		graph.AddEdge("b", "c")

		if graph.HasCycle("c", "a") != true {
			t.Error("Expected cycle to be detected")
		}
	})

	t.Run("False", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("a", func() int { return 1 })
		graph.AddNode("b", func() int { return 2 })
		graph.AddNode("c", func() int { return 3 })

		graph.AddEdge("a", "b")

		if graph.HasCycle("b", "c") != false {
			t.Error("Expected no cycle to be detected")
		}
	})
}

func TestGraphExecuteGraphSequential(t *testing.T) {
	t.Run("WithPlanNoEdges", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() int { return 10 })

		err := graph.RunWithContext(context.Background())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	})

	t.Run("WithPlanMultipleInputs", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start1", func() int { return 10 })
		graph.AddNode("start2", func() int { return 20 })
		graph.AddNode("combine", func(a, b int) int { return a + b })

		graph.AddEdge("start1", "combine")
		graph.AddEdge("start2", "combine")

		err := graph.RunWithContext(context.Background())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		result := graph.NodeResult("combine")
		if len(result) != 1 || result[0].(int) != 30 {
			t.Errorf("Expected [30], got: %v", result)
		}
	})

	t.Run("WithMultipleIncomingEdges", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start1", func() int { return 10 })
		graph.AddNode("start2", func() int { return 20 })
		graph.AddNode("combine", func(a, b int) int { return a + b })

		graph.AddEdge("start1", "combine")
		graph.AddEdge("start2", "combine")

		err := graph.RunWithContext(context.Background())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		result := graph.NodeResult("combine")
		if len(result) != 1 || result[0].(int) != 30 {
			t.Errorf("Expected [30], got: %v", result)
		}
	})
}

func TestGraphNodeDescription(t *testing.T) {
	graph := NewGraph()

	graph.AddNode("start", func() int { return 10 })

	graph.mu.RLock()
	node := graph.nodes["start"]
	graph.mu.RUnlock()

	if node.description != "" {
		t.Errorf("Expected empty description, got: %s", node.description)
	}
}

func TestGraphEdgeWeight(t *testing.T) {
	graph := NewGraph()

	graph.AddNode("start", func() int { return 10 })
	graph.AddNode("step1", func(n int) int { return n * 2 })

	graph.AddEdge("start", "step1")

	graph.mu.RLock()
	edges := graph.edges["start"]
	graph.mu.RUnlock()

	if len(edges) != 1 {
		t.Errorf("Expected 1 edge, got: %d", len(edges))
	}

	if edges[0].weight != 0 {
		t.Errorf("Expected weight 0, got: %d", edges[0].weight)
	}
}

func TestGraphNodeResult(t *testing.T) {
	t.Run("Pending", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() int { return 10 })

		result := graph.NodeResult("start")
		if len(result) != 0 {
			t.Errorf("Expected empty result, got: %v", result)
		}
	})

	t.Run("AfterClear", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() int { return 10 })
		graph.AddNode("step1", func(n int) int { return n * 2 })
		graph.AddEdge("start", "step1")

		err := graph.RunWithContext(context.Background())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		result := graph.NodeResult("step1")
		if len(result) != 1 || result[0].(int) != 20 {
			t.Errorf("Expected [20], got: %v", result)
		}

		graph.ClearStatus()

		result = graph.NodeResult("step1")
		if len(result) != 0 {
			t.Errorf("Expected empty result after clear, got: %v", result)
		}
	})
}

func TestGraphNodeQueryNotFound(t *testing.T) {
	t.Run("Status", func(t *testing.T) {
		graph := NewGraph()
		status := graph.NodeStatus("nonexistent")
		if status != NodeStatusPending {
			t.Errorf("Expected pending status, got: %v", status)
		}
	})

	t.Run("Error", func(t *testing.T) {
		graph := NewGraph()
		err := graph.NodeError("nonexistent")
		if err != nil {
			t.Errorf("Expected nil error, got: %v", err)
		}
	})

	t.Run("Result", func(t *testing.T) {
		graph := NewGraph()
		result := graph.NodeResult("nonexistent")
		if result != nil {
			t.Errorf("Expected nil result, got: %v", result)
		}
	})
}

func TestGraphOutputWithNodeTypes(t *testing.T) {
	t.Run("StringWithBranch", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() int { return 10 })
		graph.AddNode("branch", func(n int) int { return n })
		graph.AddNode("end", func(n int) {})

		graph.AddEdge("start", "branch")
		graph.AddEdge("branch", "end")

		dotOutput := graph.String()
		if !strings.Contains(dotOutput, "branch") {
			t.Errorf("Expected dot output to contain 'branch'")
		}
	})

	t.Run("MermaidWithBranch", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() int { return 10 })
		graph.AddNode("branch", func(n int) int { return n })
		graph.AddNode("end", func(n int) {})

		graph.AddEdge("start", "branch")
		graph.AddEdge("branch", "end")

		mermaidOutput := graph.Mermaid()
		if !strings.Contains(mermaidOutput, "branch") {
			t.Errorf("Expected mermaid output to contain 'branch'")
		}
	})

	t.Run("StringWithParallel", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() int { return 10 })
		graph.AddNode("parallel", func(n int) int { return n * 2 })
		graph.AddNode("end", func(n int) {})

		graph.AddEdge("start", "parallel")
		graph.AddEdge("parallel", "end")

		dotOutput := graph.String()
		if !strings.Contains(dotOutput, "parallel") {
			t.Errorf("Expected dot output to contain 'parallel'")
		}
	})

	t.Run("MermaidWithParallel", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() int { return 10 })
		graph.AddNode("parallel", func(n int) int { return n * 2 })
		graph.AddNode("end", func(n int) {})

		graph.AddEdge("start", "parallel")
		graph.AddEdge("parallel", "end")

		mermaidOutput := graph.Mermaid()
		if !strings.Contains(mermaidOutput, "parallel") {
			t.Errorf("Expected mermaid output to contain 'parallel'")
		}
	})
}

func TestGraphPrepareArgsWithType(t *testing.T) {
	testCases := []struct {
		name        string
		values      []any
		argTypes    []reflect.Type
		expectError bool
		validate    func(t *testing.T, args []reflect.Value)
	}{
		{
			name:     "BasicTypes",
			values:   []any{10, "hello"},
			argTypes: []reflect.Type{reflect.TypeOf(0), reflect.TypeOf("")},
			validate: func(t *testing.T, args []reflect.Value) {
				if args[0].Int() != 10 {
					t.Errorf("Expected 10, got: %v", args[0].Int())
				}
				if args[1].String() != "hello" {
					t.Errorf("Expected 'hello', got: %v", args[1].String())
				}
			},
		},
		{
			name:     "TypeConversion",
			values:   []any{10},
			argTypes: []reflect.Type{reflect.TypeOf(float64(0))},
			validate: func(t *testing.T, args []reflect.Value) {
				if args[0].Float() != 10.0 {
					t.Errorf("Expected 10.0, got: %v", args[0].Float())
				}
			},
		},
		{
			name:     "FromSlice",
			values:   []any{[]int{1, 2, 3}},
			argTypes: []reflect.Type{reflect.TypeOf(0), reflect.TypeOf(0), reflect.TypeOf(0)},
			validate: func(t *testing.T, args []reflect.Value) {
				if len(args) != 3 {
					t.Errorf("Expected 3 arguments, got: %d", len(args))
				}
			},
		},
		{
			name:        "EmptyValues",
			values:      []any{},
			argTypes:    []reflect.Type{reflect.TypeOf(0)},
			expectError: true,
		},
		{
			name:        "CountMismatch",
			values:      []any{10},
			argTypes:    []reflect.Type{reflect.TypeOf(0), reflect.TypeOf(0)},
			expectError: true,
		},
		{
			name:        "FromSliceCountMismatch",
			values:      []any{[]int{1, 2, 3}},
			argTypes:    []reflect.Type{reflect.TypeOf(0), reflect.TypeOf(0)},
			expectError: true,
		},
		{
			name:     "NoArgTypes",
			values:   []any{},
			argTypes: []reflect.Type{},
			validate: func(t *testing.T, args []reflect.Value) {
				if len(args) != 0 {
					t.Errorf("Expected empty args, got: %v", args)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reflectValues := make([]reflect.Value, len(tc.values))
			for i, v := range tc.values {
				reflectValues[i] = reflect.ValueOf(v)
			}
			args, err := prepareArgsWithType(reflectValues, tc.argTypes)
			if tc.expectError {
				if err == nil {
					t.Fatal("Expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if tc.validate != nil {
				tc.validate(t, args)
			}
		})
	}
}

func TestGraphWithFuncError(t *testing.T) {
	graph := NewGraph()

	graph.AddNode("start", func() int { return 10 })
	graph.AddNode("error", func(n int) (int, error) {
		return 0, fmt.Errorf("func error")
	})
	graph.AddEdge("start", "error")

	err := graph.RunWithContext(context.Background())
	if err == nil {
		t.Fatal("Expected error")
	}
}

func assertNoError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()
	if err != nil {
		if len(msgAndArgs) > 0 {
			t.Fatalf("%s: %v", msgAndArgs[0], err)
		} else {
			t.Fatalf("Unexpected error: %v", err)
		}
	}
}

func assertError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()
	if err == nil {
		if len(msgAndArgs) > 0 {
			t.Fatalf("%s", msgAndArgs[0])
		} else {
			t.Fatal("Expected error, got nil")
		}
	}
}

func assertEqual(t *testing.T, expected, actual any, msgAndArgs ...any) {
	t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		if len(msgAndArgs) > 0 {
			t.Fatalf("%s: expected %v, got %v", msgAndArgs[0], expected, actual)
		} else {
			t.Fatalf("Expected %v, got %v", expected, actual)
		}
	}
}

func assertContains(t *testing.T, s, substr string, msgAndArgs ...any) {
	t.Helper()
	if !strings.Contains(s, substr) {
		if len(msgAndArgs) > 0 {
			t.Fatalf("%s: expected %q to contain %q", msgAndArgs[0], s, substr)
		} else {
			t.Fatalf("Expected %q to contain %q", s, substr)
		}
	}
}

func assertNodeStatus(t *testing.T, graph *Graph, nodeName string, expected NodeStatus) {
	t.Helper()
	actual := graph.NodeStatus(nodeName)
	if actual != expected {
		t.Fatalf("Expected node %q status to be %v, got %v", nodeName, expected, actual)
	}
}

func assertNodeResult(t *testing.T, graph *Graph, nodeName string, expected any) {
	t.Helper()
	result := graph.NodeResult(nodeName)
	if len(result) != 1 {
		t.Fatalf("Expected node %q to have 1 result, got %d", nodeName, len(result))
	}
	if !reflect.DeepEqual(expected, result[0]) {
		t.Fatalf("Expected node %q result to be %v, got %v", nodeName, expected, result[0])
	}
}

func runGraphSequential(t *testing.T, graph *Graph) {
	t.Helper()
	err := graph.RunWithContext(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func createSimpleLinearGraph(t *testing.T) *Graph {
	t.Helper()
	graph := NewGraph()
	graph.AddNode("start", func() int { return 10 })
	graph.AddNode("double", func(n int) int { return n * 2 })
	graph.AddEdge("start", "double")
	return graph
}

func TestGraphAddLoop(t *testing.T) {
	t.Run("LoopWithCount", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() int { return 1 })
		graph.AddNode("loop", func(n int) int { return n * 2 })
		graph.AddNode("end", func(n int) int { return n + 1 })
		graph.AddEdge("start", "loop")
		graph.AddLoopEdge("loop", func(n int) bool { return n < 8 }, 10)
		graph.AddEdge("loop", "end")

		err := graph.RunWithContext(context.Background())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		result := graph.NodeResult("end")
		if len(result) != 1 || result[0].(int) != 9 {
			t.Errorf("Expected [9], got: %v", result)
		}
	})

	t.Run("LoopWithMaxIterations", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() int { return 1 })
		graph.AddNode("loop", func(n int) int { return n + 1 })
		graph.AddEdge("start", "loop")
		graph.AddLoopEdge("loop", func(n int) bool { return true }, 5)

		err := graph.RunWithContext(context.Background())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		result := graph.NodeResult("loop")
		if len(result) != 1 || result[0].(int) != 6 {
			t.Errorf("Expected [6], got: %v", result)
		}
	})

	t.Run("LoopWithError", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() int { return 1 })
		graph.AddNode("loop", func(n int) (int, error) {
			if n > 3 {
				return 0, &ChainError{Message: "loop error"}
			}
			return n + 1, nil
		})
		graph.AddEdge("start", "loop")
		graph.AddLoopEdge("loop", func(n int) bool { return true }, 10)

		err := graph.RunWithContext(context.Background())
		if err == nil {
			t.Fatal("Expected error")
		}
		if !strings.Contains(err.Error(), "loop error") {
			t.Errorf("Expected 'loop error', got: %v", err)
		}
	})
}

func TestGraphAddBranch(t *testing.T) {
	t.Run("BranchWithCondition", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() int { return 15 })
		graph.AddNode("large", func(n int) int { return n * 2 })
		graph.AddNode("small", func(n int) int { return n * 3 })
		graph.AddNode("end", func(n int) int { return n + 1 })
		graph.AddBranchEdge("start", map[string]any{
			"large": func(n int) bool { return n > 10 },
			"small": func(n int) bool { return n <= 10 },
		})
		graph.AddEdge("large", "end")
		graph.AddEdge("small", "end")

		err := graph.RunWithContext(context.Background())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		result := graph.NodeResult("end")
		if len(result) != 1 || result[0].(int) != 31 {
			t.Errorf("Expected [31], got: %v", result)
		}
	})

	t.Run("BranchWithSmallValue", func(t *testing.T) {
		graph := NewGraph()
		graph.AddNode("start", func() int { return 5 })
		graph.AddNode("large", func(n int) int { return n * 2 })
		graph.AddNode("small", func(n int) int { return n * 3 })
		graph.AddNode("end", func(n int) int { return n + 1 })
		graph.AddBranchEdge("start", map[string]any{
			"large": func(n int) bool { return n > 10 },
			"small": func(n int) bool { return n <= 10 },
		})
		graph.AddEdge("large", "end")
		graph.AddEdge("small", "end")

		err := graph.RunWithContext(context.Background())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		result := graph.NodeResult("end")
		if len(result) != 1 || result[0].(int) != 16 {
			t.Errorf("Expected [16], got: %v", result)
		}
	})
}

func TestGraphLoopParallel(t *testing.T) {
	graph := NewGraph()
	graph.AddNode("start", func() int { return 1 })
	graph.AddNode("loop", func(n int) int { return n * 2 })
	graph.AddNode("end", func(n int) int { return n + 1 })
	graph.AddEdge("start", "loop")
	graph.AddLoopEdge("loop", func(n int) bool { return n < 8 }, 10)
	graph.AddEdge("loop", "end")

	err := graph.RunWithContext(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result := graph.NodeResult("end")
	if len(result) != 1 || result[0].(int) != 9 {
		t.Errorf("Expected [9], got: %v", result)
	}
}
