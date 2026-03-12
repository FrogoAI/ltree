package ltree

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type Layer[K comparable, V any] struct {
	Nodes      []*Node[K, V]
	AsyncCount int
	SyncCount  int
}

type TreeLayer[K comparable, V any] struct {
	layers     map[int]*Layer[K, V]
	dictionary map[K]*Node[K, V]
	maxLayer   int
	mu         sync.RWMutex
}

func NewTreeLayer[K comparable, V any]() *TreeLayer[K, V] {
	return &TreeLayer[K, V]{
		layers:     map[int]*Layer[K, V]{},
		dictionary: map[K]*Node[K, V]{},
		maxLayer:   0,
	}
}

func (t *TreeLayer[K, V]) Add(entries ...Executor[K, V]) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, entry := range entries {
		t.dictionary[entry.Key()] = NewNode[K, V](entry)
	}

	for _, entry := range entries {
		node := t.dictionary[entry.Key()]

		if node.state == stateUnvisited {
			if err := t.visit(node); err != nil {
				return err
			}
		}
	}

	for _, entry := range entries {
		node := t.dictionary[entry.Key()]
		if t.maxLayer < node.level {
			t.maxLayer = node.level
		}

		_, ok := t.layers[node.level]
		if !ok {
			t.layers[node.level] = &Layer[K, V]{}
		}

		if node.value.IsAsync() {
			t.layers[node.level].AsyncCount++
		} else {
			t.layers[node.level].SyncCount++
		}

		t.layers[node.level].Nodes = append(t.layers[node.level].Nodes, node)
	}

	return nil
}

func (t *TreeLayer[K, V]) visit(node *Node[K, V]) error {
	node.state = stateVisiting

	for _, need := range node.value.DependsOn() {
		parentNode, ok := t.dictionary[need]
		if !ok {
			continue
		}

		switch parentNode.state {
		case stateVisiting:
			return fmt.Errorf("%w: %v and %v", ErrCircuitDependency, node.value.Key(), need)
		case stateVisited:
			if !node.HasParent(parentNode) {
				node.AddParent(parentNode)
			}
		default:
			if err := t.visit(parentNode); err != nil {
				return err
			}

			node.AddParent(parentNode)
		}
	}

	node.state = stateVisited

	return nil
}

func (t *TreeLayer[K, V]) Layers() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.maxLayer
}

func (t *TreeLayer[K, V]) AsyncCount(layer int) int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	l, ok := t.layers[layer]
	if !ok {
		return 0
	}

	return l.AsyncCount
}

func (t *TreeLayer[K, V]) SyncCount(layer int) int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	l, ok := t.layers[layer]
	if !ok {
		return 0
	}

	return l.SyncCount
}

func (t *TreeLayer[K, V]) Retrieve(layer int) (async chan *Node[K, V], syncCh chan *Node[K, V]) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	l, ok := t.layers[layer]
	if !ok {
		return nil, nil
	}

	count := l.AsyncCount
	async = make(chan *Node[K, V], count)
	syncCh = make(chan *Node[K, V], len(l.Nodes)-count)

	nodes := l.Nodes

	go func() {
		for _, node := range nodes {
			if node.value.IsAsync() {
				async <- node
			} else {
				syncCh <- node
			}
		}

		close(async)
		close(syncCh)
	}()

	return async, syncCh
}

func (t *TreeLayer[K, V]) Execute(ctx context.Context, executor func(ctx context.Context, k K, n V)) {
	if executor == nil {
		return
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	for i := 0; i <= t.maxLayer; i++ {
		layer, ok := t.layers[i]
		if !ok {
			continue
		}

		var wg sync.WaitGroup

		for _, node := range layer.Nodes {
			if !node.value.IsAsync() || node.value.Skip() {
				continue
			}

			wg.Add(1)

			go func(n *Node[K, V]) {
				defer wg.Done()

				select {
				case <-ctx.Done():
				default:
					executor(ctx, n.value.Key(), n.value.Value())
				}
			}(node)
		}

		for _, node := range layer.Nodes {
			if node.value.IsAsync() || node.value.Skip() {
				continue
			}

			select {
			case <-ctx.Done():
				wg.Wait()
				return
			default:
				executor(ctx, node.value.Key(), node.value.Value())
			}
		}

		wg.Wait()
	}
}

// GetLayersCount returns the number of layers in the tree
func (t *TreeLayer[K, V]) GetLayersCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.maxLayer + 1
}

// GetNodesInLayer returns the number of nodes in a specific layer
func (t *TreeLayer[K, V]) GetNodesInLayer(layer int) int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if l, ok := t.layers[layer]; ok {
		return len(l.Nodes)
	}

	return 0
}

// GetNodeKeys returns all the keys of nodes in the tree
func (t *TreeLayer[K, V]) GetNodeKeys() []K {
	t.mu.RLock()
	defer t.mu.RUnlock()

	keys := make([]K, 0, len(t.dictionary))
	for k := range t.dictionary {
		keys = append(keys, k)
	}

	return keys
}

// GetNodeDependencies returns the dependencies of a given node
func (t *TreeLayer[K, V]) GetNodeDependencies(key K) []K {
	t.mu.RLock()
	defer t.mu.RUnlock()

	node, ok := t.dictionary[key]
	if !ok {
		return nil
	}

	deps := make([]K, 0, len(node.parent))
	for _, parent := range node.parent {
		deps = append(deps, parent.value.Key())
	}

	return deps
}

// GetPretty returns a readable representation of the LTree
func (t *TreeLayer[K, V]) GetPretty() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result strings.Builder
	result.WriteString("LTree Structure:\n")

	for layer := 0; layer <= t.maxLayer; layer++ {
		result.WriteString(fmt.Sprintf("Layer %d:\n", layer))

		if nodes, ok := t.layers[layer]; ok {
			for _, node := range nodes.Nodes {
				result.WriteString(fmt.Sprintf("  Node: %v\n", node.value.Key()))
				result.WriteString(fmt.Sprintf("    Dependencies: %v\n", node.value.DependsOn()))
				result.WriteString(fmt.Sprintf("    Is Async: %v\n", node.value.IsAsync()))
			}
		} else {
			result.WriteString("  (empty)\n")
		}
	}

	return result.String()
}
