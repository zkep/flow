package flow

import (
	"fmt"
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

func TestBasicGraphCreation(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() string {
		return "start"
	})
	graph.AddNode("process1", func(s string) string {
		return s + " -> process1"
	}, NodeTypeNormal)
	graph.AddNode("process2", func(s string) string {
		return s + " -> process2"
	}, NodeTypeNormal)
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

	endNodeStatus := graph.NodeStatus("end")
	if endNodeStatus != NodeStatusCompleted {
		t.Errorf("Expected end node to be completed, got %v", endNodeStatus)
	}
}

func TestGraphNodeTypes(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() string { return "start" })
	graph.BranchNode("branch", func(s string) string { return s })
	graph.ParallelNode("parallel", func(s string) string { return s })
	graph.LoopNode("loop", func(s string) string { return s })
	graph.AddNode("normal", func(s string) string { return s }, NodeTypeNormal)
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

func TestGraphValuePropagation(t *testing.T) {
	graph := NewGraph()

	input := TestData{Value: 10, Status: "input"}

	graph.StartNode("start", func() TestData {
		return input
	})

	graph.AddNode("multiply", func(d TestData) TestData {
		d.Value *= 2
		return d
	}, NodeTypeNormal)

	graph.AddNode("add", func(d TestData) TestData {
		d.Value += 5
		return d
	}, NodeTypeNormal)

	graph.AddNode("format", func(d TestData) string {
		return fmt.Sprintf("%d-%s", d.Value, d.Status)
	}, NodeTypeNormal)

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

func TestGraphWithMultiReturn(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() (int, string) {
		return 10, "test"
	})

	graph.AddNode("process", func(a int, s string) (string, int, bool) {
		return s + "-processed", a * 2, true
	}, NodeTypeNormal)

	graph.AddNode("verify", func(s string, a int, b bool) string {
		if b {
			return fmt.Sprintf("%s:%d", s, a)
		}
		return "invalid"
	}, NodeTypeNormal)

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

	graph.AddNode("success", func(n int) string {
		return "success"
	}, NodeTypeNormal)

	graph.AddNode("error", func(n int) string {
		return "error"
	}, NodeTypeNormal)

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

			graph.AddNode("high", func(n int) string { return "high" }, NodeTypeNormal)
			graph.AddNode("medium", func(n int) string { return "medium" }, NodeTypeNormal)
			graph.AddNode("low", func(n int) string { return "low" }, NodeTypeNormal)

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

	graph.AddNode("step1", func(s string) string {
		return s + " -> step1"
	}, NodeTypeNormal)

	graph.AddNode("step2", func(s string) string {
		return s + " -> step2"
	}, NodeTypeNormal)

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

	graph.AddNode("combine", func(a, b int) int {
		return a + b
	}, NodeTypeNormal)

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

	graph.AddNode("combine", func(a, b int) int {
		return a + b
	}, NodeTypeNormal)

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

	graph.AddNode("process1", func(n int) int {
		return n * 2
	}, NodeTypeNormal)

	graph.AddNode("error_node", func(n int) (int, error) {
		return 0, &ChainError{Message: "test error"}
	}, NodeTypeNormal)

	graph.AddNode("process2", func(n int) int {
		return n + 5
	}, NodeTypeNormal)

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

	graph.AddNode("test", func() string { return "test" }, NodeTypeNormal)

	graph.AddNode("test", func() string { return "duplicate" }, NodeTypeNormal)

	if graph.Error() == nil {
		t.Errorf("Expected error for duplicate node")
	}
}

func TestGraphWithSelfDependency(t *testing.T) {
	graph := NewGraph()

	graph.AddNode("test", func() string { return "test" }, NodeTypeNormal)

	graph.AddEdge("test", "test")

	if graph.Error() == nil {
		t.Errorf("Expected error for self dependency")
	}
}

func TestGraphCyclicDependency(t *testing.T) {
	graph := NewGraph()

	graph.AddNode("a", func() string { return "a" }, NodeTypeNormal)
	graph.AddNode("b", func() string { return "b" }, NodeTypeNormal)
	graph.AddNode("c", func() string { return "c" }, NodeTypeNormal)

	graph.AddEdge("a", "b")
	graph.AddEdge("b", "c")

	err := graph.AddEdge("c", "a")
	if err == nil {
		t.Errorf("Expected error for cyclic dependency")
	}
}

func TestGraphWithNoStartNode(t *testing.T) {
	graph := NewGraph()

	graph.AddNode("a", func() string { return "a" }, NodeTypeNormal)
	graph.AddNode("b", func() string { return "b" }, NodeTypeNormal)

	graph.AddEdge("a", "b")

	err := graph.RunSequential()
	if err == nil {
		t.Errorf("Expected error for no start node")
	}
}

func TestGraphStatusTracking(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() string { return "start" })
	graph.AddNode("process", func(s string) string { return s + " -> processed" }, NodeTypeNormal)
	graph.AddNode("end", func(s string) {}, NodeTypeNormal)

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
	graph.AddNode("process", func(s string) string { return s }, NodeTypeNormal)
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
	graph.AddNode("process", func(s string) string { return s }, NodeTypeNormal)
	graph.AddNode("end", func(s string) {}, NodeTypeNormal)

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

func TestGraphMermaidOutput(t *testing.T) {
	graph := NewGraph()

	graph.StartNode("start", func() string { return "start" })
	graph.AddNode("process", func(s string) string { return s }, NodeTypeNormal)
	graph.AddNode("end", func(s string) {}, NodeTypeNormal)

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

	graph.AddNode("noop", nil, NodeTypeNormal)

	graph.AddNode("end", func(n int) {
		fmt.Println("End:", n)
	}, NodeTypeNormal)

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

	graph.AddNode("modify", func(d *TestData) *TestData {
		d.Value *= 2
		d.Status = "modified"
		return d
	}, NodeTypeNormal)

	graph.AddNode("copy", func(d *TestData) TestData {
		return TestData{
			Value:  d.Value + 5,
			Status: d.Status + "-copied",
		}
	}, NodeTypeNormal)

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

	graph.AddNode("process1", func(a int) int {
		return a * 2
	}, NodeTypeNormal)

	graph.AddNode("process2", func(b int) int {
		return b * 3
	}, NodeTypeNormal)

	graph.AddNode("combine", func(a int) int {
		return a
	}, NodeTypeNormal)

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

	graph.AddNode("process", func(n int) (int, string, bool) {
		return n * 2, "test", true
	}, NodeTypeNormal)

	graph.AddNode("verify", func(a int, s string, b bool) string {
		if b {
			return fmt.Sprintf("%s-%d", s, a)
		}
		return "invalid"
	}, NodeTypeNormal)

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

	graph.AddNode("low", func(n int) string { return "low" }, NodeTypeNormal)
	graph.AddNode("high", func(n int) string { return "high" }, NodeTypeNormal)

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
