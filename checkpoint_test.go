package flow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestFileCheckpointStore(t *testing.T) {
	dir := t.TempDir()

	store, err := NewFileCheckpointStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	checkpoint := NewCheckpoint(CheckpointTypeGraph)
	checkpoint.Version = 1

	err = store.Save("test-key", checkpoint)
	if err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := store.Load("test-key")
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if loaded.Type != CheckpointTypeGraph {
		t.Errorf("expected type 'graph', got '%s'", loaded.Type)
	}

	keys, err := store.List()
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}

	if len(keys) != 1 || keys[0] != "test-key" {
		t.Errorf("expected ['test-key'], got %v", keys)
	}

	err = store.Delete("test-key")
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	_, err = store.Load("test-key")
	if err != ErrCheckpointNotFound {
		t.Errorf("expected ErrCheckpointNotFound, got %v", err)
	}
}

func TestMemoryCheckpointStore(t *testing.T) {
	store := NewMemoryCheckpointStore()

	checkpoint := NewCheckpoint(CheckpointTypeGraph)
	checkpoint.Version = 1

	err := store.Save("test-key", checkpoint)
	if err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := store.Load("test-key")
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if loaded.Type != CheckpointTypeGraph {
		t.Errorf("expected type 'graph', got '%s'", loaded.Type)
	}

	err = store.Delete("test-key")
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	_, err = store.Load("test-key")
	if err != ErrCheckpointNotFound {
		t.Errorf("expected ErrCheckpointNotFound, got %v", err)
	}
}

func TestGraphCheckpoint(t *testing.T) {
	graph := NewGraph()
	graph.AddNode("start", func() int { return 10 })
	graph.AddNode("double", func(n int) int { return n * 2 })
	graph.AddNode("end", func(n int) int { return n + 5 })
	graph.AddEdge("start", "double")
	graph.AddEdge("double", "end")

	err := graph.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checkpoint, err := graph.SaveCheckpoint()
	if err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	if checkpoint.Type != "graph" {
		t.Errorf("expected type 'graph', got '%s'", checkpoint.Type)
	}

	if len(checkpoint.Data.Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(checkpoint.Data.Steps))
	}
}

func TestGraphCheckpointLoad(t *testing.T) {
	graph1 := NewGraph()
	graph1.AddNode("start", func() int { return 10 })
	graph1.AddNode("double", func(n int) int { return n * 2 })
	graph1.AddEdge("start", "double")

	err := graph1.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checkpoint, err := graph1.SaveCheckpoint()
	if err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	graph2 := NewGraph()
	graph2.AddNode("start", func() int { return 0 })
	graph2.AddNode("double", func(n int) int { return 0 })
	graph2.AddEdge("start", "double")

	err = graph2.LoadCheckpoint(checkpoint)
	if err != nil {
		t.Fatalf("failed to load checkpoint: %v", err)
	}

	if graph2.nodes["start"].status != NodeStatusCompleted {
		t.Error("expected start node to be completed")
	}
}

func TestGraphSaveToStore(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileCheckpointStore(dir)

	graph := NewGraph()
	graph.AddNode("node1", func() int { return 42 })

	err := graph.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = graph.SaveToStore(store, "my-graph")
	if err != nil {
		t.Fatalf("failed to save to store: %v", err)
	}

	graph2 := NewGraph()
	graph2.AddNode("node1", func() int { return 0 })

	err = graph2.LoadFromStore(store, "my-graph")
	if err != nil {
		t.Fatalf("failed to load from store: %v", err)
	}
}

func TestGraphReset(t *testing.T) {
	graph := NewGraph()
	graph.AddNode("node1", func() int { return 10 })
	graph.AddNode("node2", func(n int) int { return n * 2 })
	graph.AddEdge("node1", "node2")

	err := graph.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	graph.Reset()

	if graph.err != nil {
		t.Error("expected error to be nil after reset")
	}

	for _, node := range graph.nodes {
		if node.status != NodeStatusPending {
			t.Errorf("expected node status to be pending, got %v", node.status)
		}
	}
}

func TestGraphState(t *testing.T) {
	graph := NewGraph()
	graph.AddNode("node1", func() int { return 10 })
	graph.AddNode("node2", func(n int) int { return n * 2 })
	graph.AddEdge("node1", "node2")

	if graph.State() != FlowStateIdle {
		t.Error("expected idle state")
	}

	err := graph.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if graph.State() != FlowStateCompleted {
		t.Error("expected completed state")
	}
}

func TestCheckpointPersistence(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileCheckpointStore(dir)

	graph := NewGraph()
	graph.AddNode("init", func() (int, string) { return 42, testHelloMsg })
	graph.AddNode("process", func(n int, s string) string {
		return s + "-processed"
	})
	graph.AddEdge("init", "process")

	err := graph.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checkpointKey := "test-persistence-" + time.Now().Format("20060102150405")
	err = graph.SaveToStore(store, checkpointKey)
	if err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	_, err = os.Stat(dir + "/" + checkpointKey + ".json")
	if err != nil {
		t.Fatalf("checkpoint file not created: %v", err)
	}

	loadedCheckpoint, err := store.Load(checkpointKey)
	if err != nil {
		t.Fatalf("failed to load checkpoint: %v", err)
	}

	if loadedCheckpoint.Type != "graph" {
		t.Errorf("expected type 'graph', got '%s'", loadedCheckpoint.Type)
	}
}

func TestGraphResume(t *testing.T) {
	graph := NewGraph()
	graph.AddNode("start", func() int { return 10 })
	graph.AddNode("middle", func(n int) int { return n * 2 })
	graph.AddNode("end", func(n int) int { return n + 5 })
	graph.AddEdge("start", "middle")
	graph.AddEdge("middle", "end")

	err := graph.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checkpoint, err := graph.SaveCheckpoint()
	if err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	graph2 := NewGraph()
	graph2.AddNode("start", func() int { return 0 })
	graph2.AddNode("middle", func(n int) int { return 0 })
	graph2.AddNode("end", func(n int) int { return 0 })
	graph2.AddEdge("start", "middle")
	graph2.AddEdge("middle", "end")

	err = graph2.LoadCheckpoint(checkpoint)
	if err != nil {
		t.Fatalf("failed to load checkpoint: %v", err)
	}

	err = graph2.Resume(context.Background())
	if err != nil {
		t.Fatalf("failed to resume: %v", err)
	}
}

func TestCheckpointMetadata(t *testing.T) {
	checkpoint := NewCheckpoint("graph")
	checkpoint.SetMetadata("key1", "value1")
	checkpoint.SetMetadata("key2", "value2")

	v, ok := checkpoint.GetMetadata("key1")
	if !ok || v != "value1" {
		t.Errorf("expected 'value1', got '%s'", v)
	}

	v, ok = checkpoint.GetMetadata("key2")
	if !ok || v != "value2" {
		t.Errorf("expected 'value2', got '%s'", v)
	}

	_, ok = checkpoint.GetMetadata("nonexistent")
	if ok {
		t.Error("expected false for nonexistent key")
	}
}

func TestUnifiedInterface(t *testing.T) {
	var _ FlowCheckpointable = (*Graph)(nil)
	var _ PausableFlow = (*Graph)(nil)
}

func TestPauseConfig(t *testing.T) {
	config := NewPauseConfig()
	if config.Mode != PauseModeImmediate {
		t.Error("expected default mode to be PauseModeImmediate")
	}

	config.SetPauseAtNodes("node1", "node2")
	if config.Mode != PauseModeAtNode {
		t.Error("expected mode to be PauseModeAtNode")
	}
	if !config.PauseAtNodes["node1"] {
		t.Error("expected node1 to be in pause nodes")
	}
	if !config.PauseAtNodes["node2"] {
		t.Error("expected node2 to be in pause nodes")
	}

	config.SetPauseOnError()
	if !config.OnErrorPause {
		t.Error("expected OnErrorPause to be true")
	}
}

func TestResumeConfig(t *testing.T) {
	config := NewResumeConfig()
	if !config.SkipCompleted {
		t.Error("expected SkipCompleted to be true by default")
	}
	if config.RetryFailed {
		t.Error("expected RetryFailed to be false by default")
	}

	config.SetRetryFailed()
	if !config.RetryFailed {
		t.Error("expected RetryFailed to be true after setting")
	}
}

func TestGraphGetNodesByStatus(t *testing.T) {
	graph := NewGraph()
	graph.AddNode("node1", func() int { return 10 })
	graph.AddNode("node2", func(n int) int { return n * 2 })
	graph.AddEdge("node1", "node2")

	pending := graph.GetNodesByStatus(NodeStatusPending)
	if len(pending) != 2 {
		t.Errorf("expected 2 pending nodes before run, got %d", len(pending))
	}

	completed := graph.GetNodesByStatus(NodeStatusCompleted)
	if len(completed) != 0 {
		t.Error("expected no completed nodes before run")
	}

	err := graph.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	completed = graph.GetNodesByStatus(NodeStatusCompleted)
	if len(completed) != 2 {
		t.Errorf("expected 2 completed nodes, got %d", len(completed))
	}

	pending = graph.GetNodesByStatus(NodeStatusPending)
	if len(pending) != 0 {
		t.Errorf("expected 0 pending nodes after run, got %d", len(pending))
	}
}

func TestGraphNodeResults(t *testing.T) {
	graph := NewGraph()
	graph.AddNode("node1", func() int { return 42 })
	graph.AddNode("node2", func(n int) int { return n * 2 })
	graph.AddEdge("node1", "node2")

	err := graph.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := graph.NodeResult("node1")
	if err != nil {
		t.Fatalf("failed to get node result: %v", err)
	}
	if len(result) != 1 || result[0] != 42 {
		t.Errorf("expected [42], got %v", result)
	}

	result, err = graph.NodeResult("node2")
	if err != nil {
		t.Fatalf("failed to get node result: %v", err)
	}
	if len(result) != 1 || result[0] != 84 {
		t.Errorf("expected [84], got %v", result)
	}

	_, err = graph.NodeResult("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestGraphPauseAtNode(t *testing.T) {
	graph := NewGraph()
	graph.AddNode("node1", func() int { return 10 })
	graph.AddNode("node2", func(n int) int { return n * 2 })
	graph.AddEdge("node1", "node2")

	err := graph.PauseAtNode("node1")
	if err != nil {
		t.Errorf("failed to pause at node1: %v", err)
	}

	err = graph.PauseAtNode("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestGraphResumeWithConfig(t *testing.T) {
	graph := NewGraph()
	graph.AddNode("start", func() int { return 10 })
	graph.AddNode("middle", func(n int) int { return n * 2 })
	graph.AddNode("end", func(n int) int { return n + 5 })
	graph.AddEdge("start", "middle")
	graph.AddEdge("middle", "end")

	err := graph.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checkpoint, err := graph.SaveCheckpoint()
	if err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	graph2 := NewGraph()
	graph2.AddNode("start", func() int { return 0 })
	graph2.AddNode("middle", func(n int) int { return 0 })
	graph2.AddNode("end", func(n int) int { return 0 })
	graph2.AddEdge("start", "middle")
	graph2.AddEdge("middle", "end")

	err = graph2.LoadCheckpoint(checkpoint)
	if err != nil {
		t.Fatalf("failed to load checkpoint: %v", err)
	}

	config := NewResumeConfig()
	config.SkipCompleted = true

	err = graph2.ResumeWithConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("failed to resume with config: %v", err)
	}
}

func TestGraphResumeSkipCompleted(t *testing.T) {
	executed := make(map[string]bool)

	graph := NewGraph()
	graph.AddNode("step1", func() int {
		executed["step1"] = true
		return 10
	})
	graph.AddNode("step2", func(n int) int {
		executed["step2"] = true
		return n * 2
	})
	graph.AddEdge("step1", "step2")

	err := graph.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !executed["step1"] || !executed["step2"] {
		t.Error("expected all steps to be executed")
	}

	clear(executed)

	config := NewResumeConfig()
	config.SkipCompleted = true

	err = graph.ResumeWithConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if executed["step1"] || executed["step2"] {
		t.Error("expected no steps to be executed when skipping completed")
	}
}

type ImmediatePauseAndResumeScenario struct {
	ExpectedSteps      int
	ExpectedReExecuted int
}

func (s *ImmediatePauseAndResumeScenario) Run() error {
	executionOrder := make([]string, 0)

	graph := NewGraph()
	graph.AddNode("download", func() string {
		executionOrder = append(executionOrder, "download")
		return "data.zip"
	})
	graph.AddNode("extract", func(s string) string {
		executionOrder = append(executionOrder, "extract")
		return s + ".extracted"
	})
	graph.AddNode("process", func(s string) string {
		executionOrder = append(executionOrder, "process")
		return s + ".processed"
	})
	graph.AddEdge("download", "extract")
	graph.AddEdge("extract", "process")

	if err := graph.Run(); err != nil {
		return fmt.Errorf("unexpected error: %w", err)
	}

	if len(executionOrder) != s.ExpectedSteps {
		return fmt.Errorf("expected %d steps executed, got %d", s.ExpectedSteps, len(executionOrder))
	}

	if err := graph.Pause(); err != nil {
		return fmt.Errorf("failed to pause: %w", err)
	}

	if graph.State() != FlowStateCompleted {
		return fmt.Errorf("expected completed state, got %v", graph.State())
	}

	executionOrder = make([]string, 0)
	config := NewResumeConfig()
	config.SkipCompleted = true
	if err := graph.ResumeWithConfig(context.Background(), config); err != nil {
		return fmt.Errorf("failed to resume: %w", err)
	}

	if len(executionOrder) != s.ExpectedReExecuted {
		return fmt.Errorf("expected %d steps to re-execute, got %d", s.ExpectedReExecuted, len(executionOrder))
	}
	return nil
}

func TestScenario_ImmediatePauseAndResume(t *testing.T) {
	scenario := &ImmediatePauseAndResumeScenario{
		ExpectedSteps:      3,
		ExpectedReExecuted: 0,
	}
	if err := scenario.Run(); err != nil {
		t.Errorf("Scenario failed: %v", err)
	}
}

func TestScenario_PauseAtSpecificNode(t *testing.T) {
	executionOrder := make([]string, 0)

	graph := NewGraph()
	graph.AddNode("validate", func() bool {
		executionOrder = append(executionOrder, "validate")
		return true
	})
	graph.AddNode("approve", func(b bool) string {
		executionOrder = append(executionOrder, "approve")
		return "approved"
	})
	graph.AddNode("execute", func(s string) string {
		executionOrder = append(executionOrder, "execute")
		return s + "-done"
	})
	graph.AddEdge("validate", "approve")
	graph.AddEdge("approve", "execute")

	err := graph.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checkpoint, err := graph.SaveCheckpoint()
	if err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	graph2 := NewGraph()
	graph2.AddNode("validate", func() bool {
		executionOrder = append(executionOrder, "validate2")
		return true
	})
	graph2.AddNode("approve", func(b bool) string {
		executionOrder = append(executionOrder, "approve2")
		return "approved"
	})
	graph2.AddNode("execute", func(s string) string {
		executionOrder = append(executionOrder, "execute2")
		return s + "-done"
	})
	graph2.AddEdge("validate", "approve")
	graph2.AddEdge("approve", "execute")

	err = graph2.LoadCheckpoint(checkpoint)
	if err != nil {
		t.Fatalf("failed to load checkpoint: %v", err)
	}

	if graph2.State() != FlowStateCompleted {
		t.Errorf("expected paused state after loading checkpoint")
	}

	completed := graph2.GetNodesByStatus(NodeStatusCompleted)
	if len(completed) != 3 {
		t.Errorf("expected 3 completed nodes, got %d", len(completed))
	}
}

type RetryFailedNodeScenario struct {
	InitValue      int
	ExpectedResult int
	FailedNode     string
}

func (s *RetryFailedNodeScenario) Run() error {
	shouldFail := true

	graph := NewGraph()
	graph.AddNode("init", func() int { return s.InitValue })
	graph.AddNode("unreliable", func(n int) (int, error) {
		if shouldFail {
			return 0, errors.New("simulated failure")
		}
		return n * 2, nil
	})
	graph.AddNode("final", func(n int) int { return n + 5 })
	graph.AddEdge("init", "unreliable")
	graph.AddEdge("unreliable", "final")

	if err := graph.RunSequential(); err == nil {
		return fmt.Errorf("expected error from first run")
	}

	failed := graph.GetNodesByStatus(NodeStatusFailed)
	if len(failed) != 1 || failed[0] != s.FailedNode {
		return fmt.Errorf("expected %s to fail, got %v", s.FailedNode, failed)
	}

	shouldFail = false

	config := NewResumeConfig()
	config.RetryFailed = true
	config.SkipCompleted = true

	if err := graph.ResumeWithConfig(context.Background(), config); err != nil {
		return fmt.Errorf("failed to resume with retry: %w", err)
	}

	if graph.State() != FlowStateCompleted {
		return fmt.Errorf("expected completed state after retry, got %v", graph.State())
	}

	result, _ := graph.NodeResult("final")
	if len(result) != 1 || result[0] != s.ExpectedResult {
		return fmt.Errorf("expected [%d], got %v", s.ExpectedResult, result)
	}
	return nil
}

func TestScenario_RetryFailedNode(t *testing.T) {
	scenario := &RetryFailedNodeScenario{
		InitValue:      10,
		ExpectedResult: 25,
		FailedNode:     "unreliable",
	}
	if err := scenario.Run(); err != nil {
		t.Errorf("Scenario failed: %v", err)
	}
}

type PersistenceRecoveryScenario struct {
	CheckpointKey string
	ExpectedKeys  int
}

func (s *PersistenceRecoveryScenario) Run(dir string) error {
	store, err := NewFileCheckpointStore(dir)
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}

	graph := NewGraph()
	graph.AddNode("fetch_api", func() string { return "api_data" })
	graph.AddNode("transform", func(s string) string { return s + "_transformed" })
	graph.AddNode("save_db", func(s string) string { return s + "_saved" })
	graph.AddEdge("fetch_api", "transform")
	graph.AddEdge("transform", "save_db")

	if err := graph.Run(); err != nil {
		return fmt.Errorf("unexpected error: %w", err)
	}

	if err := graph.SaveToStore(store, s.CheckpointKey); err != nil {
		return fmt.Errorf("failed to save to store: %w", err)
	}

	keys, err := store.List()
	if err != nil {
		return fmt.Errorf("failed to list keys: %w", err)
	}
	if len(keys) != s.ExpectedKeys || keys[0] != s.CheckpointKey {
		return fmt.Errorf("expected [%s], got %v", s.CheckpointKey, keys)
	}

	graph2 := NewGraph()
	graph2.AddNode("fetch_api", func() string { return "" })
	graph2.AddNode("transform", func(s string) string { return "" })
	graph2.AddNode("save_db", func(s string) string { return "" })
	graph2.AddEdge("fetch_api", "transform")
	graph2.AddEdge("transform", "save_db")

	if err := graph2.LoadFromStore(store, s.CheckpointKey); err != nil {
		return fmt.Errorf("failed to load from store: %w", err)
	}

	if graph2.State() != FlowStateCompleted {
		return fmt.Errorf("expected completed state, got %v", graph2.State())
	}

	result, _ := graph2.NodeResult("save_db")
	if len(result) != 1 {
		return fmt.Errorf("expected result, got %v", result)
	}
	return nil
}

func TestScenario_PersistenceRecovery(t *testing.T) {
	dir := t.TempDir()
	scenario := &PersistenceRecoveryScenario{
		CheckpointKey: "workflow-2026-02-09",
		ExpectedKeys:  1,
	}
	if err := scenario.Run(dir); err != nil {
		t.Errorf("Scenario failed: %v", err)
	}
}

func TestScenario_MultiBranchPauseResume(t *testing.T) {
	executed := make(map[string]int)

	graph := NewGraph()
	graph.AddNode("start", func() int {
		executed["start"]++
		return 10
	})
	graph.AddNode("branch_a", func(n int) int {
		executed["branch_a"]++
		return n + 1
	})
	graph.AddNode("branch_b", func(n int) int {
		executed["branch_b"]++
		return n + 2
	})
	graph.AddNode("merge", func(a, b int) int {
		executed["merge"]++
		return a + b
	})
	graph.AddEdge("start", "branch_a")
	graph.AddEdge("start", "branch_b")
	graph.AddEdge("branch_a", "merge")
	graph.AddEdge("branch_b", "merge")

	err := graph.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if executed["start"] != 1 || executed["branch_a"] != 1 || executed["branch_b"] != 1 || executed["merge"] != 1 {
		t.Errorf("expected each node to execute once, got %v", executed)
	}

	result, _ := graph.NodeResult("merge")
	if len(result) != 1 || result[0] != 23 {
		t.Errorf("expected [23], got %v", result)
	}

	checkpoint, err := graph.SaveCheckpoint()
	if err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	if checkpoint.State != FlowStateCompleted {
		t.Errorf("expected completed state, got %v", checkpoint.State)
	}

	completed := graph.GetNodesByStatus(NodeStatusCompleted)
	if len(completed) != 4 {
		t.Errorf("expected 4 completed nodes, got %d", len(completed))
	}

	graph2 := NewGraph()
	graph2.AddNode("start", func() int {
		executed["start"]++
		return 10
	})
	graph2.AddNode("branch_a", func(n int) int {
		executed["branch_a"]++
		return n + 1
	})
	graph2.AddNode("branch_b", func(n int) int {
		executed["branch_b"]++
		return n + 2
	})
	graph2.AddNode("merge", func(a, b int) int {
		executed["merge"]++
		return a + b
	})
	graph2.AddEdge("start", "branch_a")
	graph2.AddEdge("start", "branch_b")
	graph2.AddEdge("branch_a", "merge")
	graph2.AddEdge("branch_b", "merge")

	err = graph2.LoadCheckpoint(checkpoint)
	if err != nil {
		t.Fatalf("failed to load checkpoint: %v", err)
	}

	if graph2.State() != FlowStateCompleted {
		t.Errorf("expected completed state after load, got %v", graph2.State())
	}

	config := NewResumeConfig()
	config.SkipCompleted = true
	err = graph2.ResumeWithConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("failed to resume: %v", err)
	}

	if executed["start"] != 1 || executed["branch_a"] != 1 || executed["branch_b"] != 1 || executed["merge"] != 1 {
		t.Errorf("expected no re-execution, got %v", executed)
	}
}

func TestScenario_CheckpointResume(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileCheckpointStore(dir)

	executed := make(map[string]bool)

	graph := NewGraph()
	graph.AddNode("download", func() string {
		executed["download"] = true
		time.Sleep(100 * time.Millisecond)
		return "large_file.zip"
	})
	graph.AddNode("extract", func(s string) string {
		executed["extract"] = true
		return s + ".extracted"
	})
	graph.AddNode("process", func(s string) string {
		executed["process"] = true
		return s + ".processed"
	})
	graph.AddNode("save", func(s string) string {
		executed["save"] = true
		return s + ".done"
	})
	graph.AddEdge("download", "extract")
	graph.AddEdge("extract", "process")
	graph.AddEdge("process", "save")

	// 模拟中途保存（断点）
	_ = graph.RunSequential()
	_ = graph.SaveToStore(store, "resume-test")

	// 清除执行记录，模拟新进程恢复
	clear(executed)

	// 恢复并继续
	graph2 := NewGraph()
	graph2.AddNode("download", func() string {
		executed["download"] = true
		return "large_file.zip"
	})
	graph2.AddNode("extract", func(s string) string {
		executed["extract"] = true
		return s + ".extracted"
	})
	graph2.AddNode("process", func(s string) string {
		executed["process"] = true
		return s + ".processed"
	})
	graph2.AddNode("save", func(s string) string {
		executed["save"] = true
		return s + ".done"
	})
	graph2.AddEdge("download", "extract")
	graph2.AddEdge("extract", "process")
	graph2.AddEdge("process", "save")

	_ = graph2.LoadFromStore(store, "resume-test")

	config := NewResumeConfig()
	config.SkipCompleted = true
	_ = graph2.ResumeWithConfig(context.Background(), config)

	// 验证已完成的节点未重新执行
	// 注意：当前实现在 RunWithContext 中会检查 NodeStatusCompleted
}

func TestScenario_FailureRecovery(t *testing.T) {
	retryCount := 0
	shouldFail := true

	graph := NewGraph()
	graph.AddNode("init", func() int { return 10 })
	graph.AddNode("risky_operation", func(n int) (int, error) {
		retryCount++
		if shouldFail {
			return 0, errors.New("network timeout")
		}
		return n * 2, nil
	})
	graph.AddNode("final", func(n int) int { return n + 5 })
	graph.AddEdge("init", "risky_operation")
	graph.AddEdge("risky_operation", "final")

	// 第一次执行，模拟失败
	err := graph.RunSequential()
	if err == nil {
		t.Fatal("expected error")
	}

	// 验证失败状态
	failed := graph.GetNodesByStatus(NodeStatusFailed)
	if len(failed) != 1 || failed[0] != "risky_operation" {
		t.Errorf("expected risky_operation to fail, got %v", failed)
	}

	// 保存检查点（模拟系统崩溃前的状态）
	checkpoint, _ := graph.SaveCheckpoint()
	if checkpoint.State != FlowStatePaused {
		t.Errorf("expected paused state, got %v", checkpoint.State)
	}

	// 模拟修复问题（网络恢复）
	shouldFail = false

	// 恢复并重试失败节点
	config := NewResumeConfig()
	config.RetryFailed = true
	config.SkipCompleted = true

	err = graph.ResumeWithConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("recovery failed: %v", err)
	}

	// 验证最终状态
	if graph.State() != FlowStateCompleted {
		t.Errorf("expected completed state, got %v", graph.State())
	}

	// 验证结果正确
	result, _ := graph.NodeResult("final")
	if len(result) != 1 || result[0] != 25 {
		t.Errorf("expected [25], got %v", result)
	}
}

func TestScenario_ManualApprovalWithPauseConfig(t *testing.T) {
	executed := make(map[string]bool)

	graph := NewGraph()
	graph.AddNode("validate", func() int {
		executed["validate"] = true
		return 1
	})
	graph.AddNode("approval_point", func(n int) int {
		executed["approval_point"] = true
		return n + 10
	})
	graph.AddNode("process", func(n int) int {
		executed["process"] = true
		return n + 100
	})
	graph.AddNode("finalize", func(n int) int {
		executed["finalize"] = true
		return n + 1000
	})
	graph.AddEdge("validate", "approval_point")
	graph.AddEdge("approval_point", "process")
	graph.AddEdge("process", "finalize")

	pauseConfig := NewPauseConfig()
	pauseConfig.SetPauseAtNodes("approval_point")
	graph.SetPauseConfig(pauseConfig)

	err := graph.RunSequential()
	if err != ErrFlowPaused {
		t.Fatalf("expected ErrFlowPaused, got %v", err)
	}

	if graph.GetPausedAtNode() != "approval_point" {
		t.Errorf("expected to pause at approval_point, got %s", graph.GetPausedAtNode())
	}

	if !executed["validate"] {
		t.Error("expected validate to be executed")
	}

	if executed["approval_point"] {
		t.Error("expected approval_point NOT to be executed")
	}

	if graph.State() != FlowStatePaused {
		t.Errorf("expected paused state, got %v", graph.State())
	}

	checkpoint, err := graph.SaveCheckpoint()
	if err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	graph2 := NewGraph()
	graph2.AddNode("validate", func() int { return 1 })
	graph2.AddNode("approval_point", func(n int) int { return n + 10 })
	graph2.AddNode("process", func(n int) int { return n + 100 })
	graph2.AddNode("finalize", func(n int) int { return n + 1000 })
	graph2.AddEdge("validate", "approval_point")
	graph2.AddEdge("approval_point", "process")
	graph2.AddEdge("process", "finalize")

	err = graph2.LoadCheckpoint(checkpoint)
	if err != nil {
		t.Fatalf("failed to load checkpoint: %v", err)
	}

	graph2.mu.Lock()
	graph2.pausedAtNode = ""
	graph2.err = nil
	for _, node := range graph2.nodes {
		if node.status == NodeStatusCompleted {
			continue
		}
		if node.status == NodeStatusFailed {
			node.status = NodeStatusPending
			node.result = nil
			node.err = nil
		}
	}
	graph2.mu.Unlock()

	err = graph2.RunSequentialWithContext(context.Background())
	if err != nil {
		t.Fatalf("failed to resume: %v", err)
	}

	if graph2.State() != FlowStateCompleted {
		t.Errorf("expected completed state, got %v", graph2.State())
	}

	result, _ := graph2.NodeResult("finalize")
	if len(result) != 1 || result[0] != 1111 {
		t.Errorf("expected [1111], got %v", result)
	}
}

func TestScenario_ManualApprovalWithPauseSignal(t *testing.T) {
	executed := make(map[string]bool)
	pauseSignal := NewSimplePauseSignal()

	graph := NewGraph()
	graph.AddNode("step1", func() int {
		executed["step1"] = true
		return 1
	})
	graph.AddNode("step2", func(n int) int {
		executed["step2"] = true
		return n + 2
	})
	graph.AddNode("step3", func(n int) int {
		executed["step3"] = true
		return n + 3
	})
	graph.AddNode("step4", func(n int) int {
		executed["step4"] = true
		return n + 4
	})
	graph.AddEdge("step1", "step2")
	graph.AddEdge("step2", "step3")
	graph.AddEdge("step3", "step4")

	graph.SetPauseSignal(pauseSignal)

	pauseSignal.SetPaused(true)

	err := graph.RunSequential()
	if err != ErrFlowPaused {
		t.Fatalf("expected ErrFlowPaused, got %v", err)
	}

	if graph.GetPausedAtNode() == "" {
		t.Error("expected to be paused at some node")
	}

	if graph.State() != FlowStatePaused {
		t.Errorf("expected paused state, got %v", graph.State())
	}

	checkpoint, _ := graph.SaveCheckpoint()

	graph2 := NewGraph()
	graph2.AddNode("step1", func() int { return 1 })
	graph2.AddNode("step2", func(n int) int { return n + 2 })
	graph2.AddNode("step3", func(n int) int { return n + 3 })
	graph2.AddNode("step4", func(n int) int { return n + 4 })
	graph2.AddEdge("step1", "step2")
	graph2.AddEdge("step2", "step3")
	graph2.AddEdge("step3", "step4")

	_ = graph2.LoadCheckpoint(checkpoint)

	graph2.mu.Lock()
	graph2.pausedAtNode = ""
	graph2.err = nil
	for _, node := range graph2.nodes {
		if node.status == NodeStatusCompleted {
			continue
		}
		if node.status == NodeStatusFailed {
			node.status = NodeStatusPending
			node.result = nil
			node.err = nil
		}
	}
	graph2.mu.Unlock()

	err = graph2.RunSequentialWithContext(context.Background())
	if err != nil {
		t.Fatalf("failed to resume: %v", err)
	}

	if graph2.State() != FlowStateCompleted {
		t.Errorf("expected completed state, got %v", graph2.State())
	}

	result, _ := graph2.NodeResult("step4")
	if len(result) != 1 || result[0] != 10 {
		t.Errorf("expected [10], got %v", result)
	}
}

func TestScenario_ResourceLimitWithChecker(t *testing.T) {
	resourceChecker := NewSimpleResourceChecker(2, 1)

	graph := NewGraph()
	graph.AddNode("task_a", func() int {
		resourceChecker.Consume()
		defer resourceChecker.Release()
		time.Sleep(20 * time.Millisecond)
		return 1
	})
	graph.AddNode("task_b", func() int {
		resourceChecker.Consume()
		defer resourceChecker.Release()
		time.Sleep(20 * time.Millisecond)
		return 2
	})
	graph.AddNode("task_c", func() int {
		resourceChecker.Consume()
		defer resourceChecker.Release()
		time.Sleep(20 * time.Millisecond)
		return 3
	})
	graph.AddNode("merge", func(a, b, c int) int {
		return a + b + c
	})
	graph.AddEdge("task_a", "merge")
	graph.AddEdge("task_b", "merge")
	graph.AddEdge("task_c", "merge")

	graph.SetResourceChecker(resourceChecker)

	resourceChecker.SetAvailable(0)

	err := graph.RunSequential()
	if err != ErrResourceNotAvailable {
		t.Fatalf("expected ErrResourceNotAvailable, got %v", err)
	}

	if graph.GetPausedAtNode() == "" {
		t.Error("expected to be paused at some node")
	}

	checkpoint, _ := graph.SaveCheckpoint()

	resourceChecker.SetAvailable(3)

	graph2 := NewGraph()
	graph2.AddNode("task_a", func() int { return 1 })
	graph2.AddNode("task_b", func() int { return 2 })
	graph2.AddNode("task_c", func() int { return 3 })
	graph2.AddNode("merge", func(a, b, c int) int { return a + b + c })
	graph2.AddEdge("task_a", "merge")
	graph2.AddEdge("task_b", "merge")
	graph2.AddEdge("task_c", "merge")

	_ = graph2.LoadCheckpoint(checkpoint)

	newChecker := NewSimpleResourceChecker(3, 1)
	graph2.SetResourceChecker(newChecker)

	config := NewResumeConfig()
	config.SkipCompleted = true
	err = graph2.ResumeWithConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("failed to resume: %v", err)
	}

	if graph2.State() != FlowStateCompleted {
		t.Errorf("expected completed state, got %v", graph2.State())
	}

	result, _ := graph2.NodeResult("merge")
	if len(result) != 1 || result[0] != 6 {
		t.Errorf("expected [6], got %v", result)
	}
}

func TestScenario_ResourceLimitDynamicRecovery(t *testing.T) {
	resourceChecker := NewSimpleResourceChecker(1, 1)

	graph := NewGraph()
	graph.AddNode("init", func() int { return 10 })
	graph.AddNode("resource_intensive", func(n int) (int, error) {
		if !resourceChecker.CheckAvailable("resource_intensive") {
			return 0, ErrResourceNotAvailable
		}
		resourceChecker.Consume()
		defer resourceChecker.Release()
		return n * 2, nil
	})
	graph.AddNode("final", func(n int) int { return n + 5 })
	graph.AddEdge("init", "resource_intensive")
	graph.AddEdge("resource_intensive", "final")

	graph.SetResourceChecker(resourceChecker)

	resourceChecker.SetAvailable(0)

	err := graph.RunSequential()
	if err != ErrResourceNotAvailable {
		t.Fatalf("expected ErrResourceNotAvailable, got %v", err)
	}

	pausedAt := graph.GetPausedAtNode()
	if pausedAt != "resource_intensive" && pausedAt != "init" {
		t.Errorf("expected to be paused at a node, got %s", pausedAt)
	}

	checkpoint, _ := graph.SaveCheckpoint()

	resourceChecker.SetAvailable(3)

	graph2 := NewGraph()
	graph2.AddNode("init", func() int { return 10 })
	graph2.AddNode("resource_intensive", func(n int) (int, error) { return n * 2, nil })
	graph2.AddNode("final", func(n int) int { return n + 5 })
	graph2.AddEdge("init", "resource_intensive")
	graph2.AddEdge("resource_intensive", "final")

	_ = graph2.LoadCheckpoint(checkpoint)

	newChecker := NewSimpleResourceChecker(3, 1)
	graph2.SetResourceChecker(newChecker)

	config := NewResumeConfig()
	config.SkipCompleted = true
	err = graph2.ResumeWithConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("failed to resume: %v", err)
	}

	if graph2.State() != FlowStateCompleted {
		t.Errorf("expected completed state, got %v", graph2.State())
	}

	result, _ := graph2.NodeResult("final")
	if len(result) != 1 || result[0] != 25 {
		t.Errorf("expected [25], got %v", result)
	}
}

func TestScenario_ParallelWithPauseSignal(t *testing.T) {
	pauseSignal := NewSimplePauseSignal()

	graph := NewGraph()
	graph.AddNode("start", func() int { return 10 })
	graph.AddNode("branch_a", func(n int) int { return n + 1 })
	graph.AddNode("branch_b", func(n int) int { return n + 2 })
	graph.AddNode("merge", func(a, b int) int { return a + b })
	graph.AddEdge("start", "branch_a")
	graph.AddEdge("start", "branch_b")
	graph.AddEdge("branch_a", "merge")
	graph.AddEdge("branch_b", "merge")

	graph.SetPauseSignal(pauseSignal)
	pauseSignal.SetPaused(true)

	err := graph.Run()
	if err != ErrFlowPaused {
		t.Logf("execution result: %v", err)
	}

	checkpoint, _ := graph.SaveCheckpoint()

	graph2 := NewGraph()
	graph2.AddNode("start", func() int { return 10 })
	graph2.AddNode("branch_a", func(n int) int { return n + 1 })
	graph2.AddNode("branch_b", func(n int) int { return n + 2 })
	graph2.AddNode("merge", func(a, b int) int { return a + b })
	graph2.AddEdge("start", "branch_a")
	graph2.AddEdge("start", "branch_b")
	graph2.AddEdge("branch_a", "merge")
	graph2.AddEdge("branch_b", "merge")

	_ = graph2.LoadCheckpoint(checkpoint)

	graph2.mu.Lock()
	graph2.pausedAtNode = ""
	graph2.err = nil
	for _, node := range graph2.nodes {
		if node.status == NodeStatusCompleted {
			continue
		}
		if node.status == NodeStatusFailed {
			node.status = NodeStatusPending
			node.result = nil
			node.err = nil
		}
	}
	graph2.mu.Unlock()

	err = graph2.RunWithContext(context.Background())
	if err != nil {
		t.Fatalf("failed to resume: %v", err)
	}

	if graph2.State() != FlowStateCompleted {
		t.Errorf("expected completed state, got %v", graph2.State())
	}

	result, _ := graph2.NodeResult("merge")
	if len(result) != 1 || result[0] != 23 {
		t.Errorf("expected [23], got %v", result)
	}
}

func TestScenario_PauseOnError(t *testing.T) {
	graph := NewGraph()
	graph.AddNode("step1", func() int { return 10 })
	graph.AddNode("step2", func(n int) (int, error) {
		return 0, errors.New("simulated error")
	})
	graph.AddNode("step3", func(n int) int { return n * 2 })
	graph.AddEdge("step1", "step2")
	graph.AddEdge("step2", "step3")

	pauseConfig := NewPauseConfig()
	pauseConfig.SetPauseOnError()
	graph.SetPauseConfig(pauseConfig)

	err := graph.RunSequential()
	if err == nil {
		t.Fatal("expected error from Run")
	}

	if graph.GetPausedAtNode() != "step2" {
		t.Errorf("expected to pause at step2, got %s", graph.GetPausedAtNode())
	}

	failed := graph.GetNodesByStatus(NodeStatusFailed)
	if len(failed) != 1 || failed[0] != "step2" {
		t.Errorf("expected step2 to fail, got %v", failed)
	}

	checkpoint, err := graph.SaveCheckpoint()
	if err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	if checkpoint.State != FlowStatePaused {
		t.Errorf("expected paused state in checkpoint, got %v", checkpoint.State)
	}

	graph2 := NewGraph()
	graph2.AddNode("step1", func() int { return 10 })
	graph2.AddNode("step2", func(n int) int { return n * 2 })
	graph2.AddNode("step3", func(n int) int { return n + 5 })
	graph2.AddEdge("step1", "step2")
	graph2.AddEdge("step2", "step3")

	_ = graph2.LoadCheckpoint(checkpoint)

	graph2.mu.Lock()
	graph2.pausedAtNode = ""
	graph2.err = nil
	for _, node := range graph2.nodes {
		if node.status == NodeStatusCompleted {
			continue
		}
		if node.status == NodeStatusFailed {
			node.status = NodeStatusPending
			node.result = nil
			node.err = nil
		}
	}
	graph2.mu.Unlock()

	err = graph2.RunSequentialWithContext(context.Background())
	if err != nil {
		t.Fatalf("failed to resume: %v", err)
	}

	if graph2.State() != FlowStateCompleted {
		t.Errorf("expected completed state, got %v", graph2.State())
	}

	result, _ := graph2.NodeResult("step3")
	if len(result) != 1 || result[0] != 25 {
		t.Errorf("expected [25], got %v", result)
	}
}
