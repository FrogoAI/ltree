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

		err := t.visit(entry.Key(), node)
		if err != nil {
			return err
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

func (t *TreeLayer[K, V]) visit(key K, node *Node[K, V]) error {
	for _, need := range node.value.DependsOn() {
		parentNode, ok := t.dictionary[need]
		if !ok {
			continue
		}

		if parentNode.HasParent(node) || parentNode.value.Key() == key {
			return fmt.Errorf("%w: with parent %v", ErrCircuitDependency, need)
		}

		if !parentNode.IsVisited() {
			err := t.visit(key, parentNode)
			if err != nil {
				return err
			}
		}

		node.AddParent(parentNode)
	}

	node.SetVisited(true)

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

func (t *TreeLayer[K, V]) Retrieve(layer int) (async chan *Node[K, V], sync chan *Node[K, V]) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	l, ok := t.layers[layer]
	if !ok {
		return nil, nil
	}

	count := t.layers[layer].AsyncCount
	async = make(chan *Node[K, V], count)             // size of buffer equal async elements
	sync = make(chan *Node[K, V], len(l.Nodes)-count) // size of buffer equal all other elements

	go func() {
		for _, node := range t.layers[layer].Nodes {
			if node.value.IsAsync() {
				async <- node
			} else {
				sync <- node
			}
		}

		close(async)
		close(sync)
	}()

	return async, sync
}

func (t *TreeLayer[K, V]) Execute(ctx context.Context, executor func(ctx context.Context, k K, n V)) {
	count := t.Layers()

	for i := 0; i <= count; i++ {
		asyncCh, syncCh := t.Retrieve(i)
		size := t.AsyncCount(i)
		syncSize := t.SyncCount(i)

		var wg sync.WaitGroup

		if syncSize > 0 {
			wg.Add(1)

			go func() {
				defer wg.Done()

				for {
					select {
					case n, ok := <-syncCh:
						if !ok {
							return
						}

						if n.value.Skip() {
							continue
						}

						if executor != nil {
							executor(ctx, n.value.Key(), n.value.Value())
						}
					case <-ctx.Done():
						return
					}
				}
			}()
		}

		if size > 0 {
			wg.Add(size)

			for i := 0; i < size; i++ {
				go func() {
					defer wg.Done()

					for {
						select {
						case n, ok := <-asyncCh:
							if !ok {
								return
							}

							if n.value.Skip() {
								continue
							}

							if executor != nil {
								executor(ctx, n.value.Key(), n.value.Value())
							}
						case <-ctx.Done():
							return
						}
					}
				}()
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

	// Use a map to ensure uniqueness
	uniqueDeps := make(map[K]struct{})
	for _, parent := range node.parent {
		uniqueDeps[parent.value.Key()] = struct{}{}
	}

	deps := make([]K, 0, len(uniqueDeps))
	for dep := range uniqueDeps {
		deps = append(deps, dep)
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
