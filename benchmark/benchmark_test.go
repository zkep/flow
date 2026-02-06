package flow

import (
	"context"
	"fmt"
	"testing"

	"github.com/zkep/flow"
)

func BenchmarkC32(b *testing.B) {
	graph := flow.NewGraph()
	graph.AddNode("N0", func() {})
	for j := 1; j < 32; j++ {
		graph.AddNode(fmt.Sprintf("N%d", j), func() {})
	}

	b.ResetTimer()
	for b.Loop() {
		_ = graph.RunWithContext(context.Background())
	}
}

func BenchmarkS32(b *testing.B) {
	graph := flow.NewGraph()
	graph.AddNode("N0", func() {})
	prev := "N0"
	for i := 1; i < 32; i++ {
		name := fmt.Sprintf("N%d", i)
		graph.AddNode(name, func() {})
		graph.AddEdge(prev, name)
		prev = name
	}

	b.ResetTimer()
	for b.Loop() {
		_ = graph.RunSequentialWithContext(context.Background())
	}
}

func BenchmarkC6(b *testing.B) {
	graph := flow.NewGraph()
	graph.AddNode("N0", func() {})
	graph.AddNode("N1", func() {})
	graph.AddNode("N2", func() {})
	graph.AddNode("N3", func() {})
	graph.AddNode("N4", func() {})
	graph.AddNode("N5", func() {})

	graph.AddEdge("N0", "N1")
	graph.AddEdge("N0", "N2")
	graph.AddEdge("N1", "N3")
	graph.AddEdge("N2", "N4")
	graph.AddEdge("N3", "N5")
	graph.AddEdge("N4", "N5")

	b.ResetTimer()
	for b.Loop() {
		_ = graph.RunWithContext(context.Background())
	}
}

func BenchmarkC8x8(b *testing.B) {
	graph := flow.NewGraph()
	layersCount := 8
	layerNodesCount := 8

	var curLayer, upperLayer []string

	for i := range layersCount {
		for j := range layerNodesCount {
			name := fmt.Sprintf("N%d", i*layerNodesCount+j)
			graph.AddNode(name, func() {})

			for _, upper := range upperLayer {
				graph.AddEdge(upper, name)
			}

			curLayer = append(curLayer, name)
		}

		upperLayer = curLayer
		curLayer = []string{}
	}

	b.ResetTimer()
	for b.Loop() {
		_ = graph.RunWithContext(context.Background())
	}
}
