package ltree

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type SafeSet[K comparable] struct {
	data map[K]struct{}
	mu   sync.Mutex
}

func NewSafeSet[K comparable]() *SafeSet[K] {
	return &SafeSet[K]{data: make(map[K]struct{})}
}

func (s *SafeSet[K]) Add(k K) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[k] = struct{}{}
}

func (s *SafeSet[K]) Contains(k K) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.data[k]
	return ok
}

func (s *SafeSet[K]) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.data)
}

func TestTreeExecution(t *testing.T) {
	tests := []struct {
		name        string
		entries     []Executor[string, any]
		expected    []string
		notExpected []string
	}{
		{
			name: "single node no deps",
			entries: []Executor[string, any]{
				NewEntry[string, any]("a", "val", nil, false, nil),
			},
			expected: []string{"a"},
		},
		{
			name: "linear chain",
			entries: []Executor[string, any]{
				NewEntry[string, any]("a", "v", nil, false, nil),
				NewEntry[string, any]("b", "v", []string{"a"}, false, nil),
				NewEntry[string, any]("c", "v", []string{"b"}, false, nil),
			},
			expected: []string{"a", "b", "c"},
		},
		{
			name: "diamond dependency",
			entries: []Executor[string, any]{
				NewEntry[string, any]("root", "v", nil, false, nil),
				NewEntry[string, any]("left", "v", []string{"root"}, true, nil),
				NewEntry[string, any]("right", "v", []string{"root"}, true, nil),
				NewEntry[string, any]("sink", "v", []string{"left", "right"}, false, nil),
			},
			expected: []string{"root", "left", "right", "sink"},
		},
		{
			name: "skip functionality",
			entries: []Executor[string, any]{
				NewEntry[string, any]("run", "v", nil, false, nil),
				NewEntry[string, any]("skip-me", "v", nil, false, func() bool { return true }),
				NewEntry[string, any]("skip-async", "v", nil, true, func() bool { return true }),
			},
			expected:    []string{"run"},
			notExpected: []string{"skip-me", "skip-async"},
		},
		{
			name: "all async in one layer",
			entries: []Executor[string, any]{
				NewEntry[string, any]("a1", "v", nil, true, nil),
				NewEntry[string, any]("a2", "v", nil, true, nil),
				NewEntry[string, any]("a3", "v", nil, true, nil),
			},
			expected: []string{"a1", "a2", "a3"},
		},
		{
			name: "all sync in one layer",
			entries: []Executor[string, any]{
				NewEntry[string, any]("s1", "v", nil, false, nil),
				NewEntry[string, any]("s2", "v", nil, false, nil),
			},
			expected: []string{"s1", "s2"},
		},
		{
			name: "mixed async sync same layer",
			entries: []Executor[string, any]{
				NewEntry[string, any]("root", "v", nil, false, nil),
				NewEntry[string, any]("sync1", "v", []string{"root"}, false, nil),
				NewEntry[string, any]("async1", "v", []string{"root"}, true, nil),
				NewEntry[string, any]("async2", "v", []string{"root"}, true, nil),
			},
			expected: []string{"root", "sync1", "async1", "async2"},
		},
		{
			name: "missing dependency is ignored",
			entries: []Executor[string, any]{
				NewEntry[string, any]("a", "v", []string{"nonexistent"}, false, nil),
			},
			expected: []string{"a"},
		},
		{
			name: "multiple independent roots",
			entries: []Executor[string, any]{
				NewEntry[string, any]("r1", "v", nil, false, nil),
				NewEntry[string, any]("r2", "v", nil, true, nil),
				NewEntry[string, any]("child", "v", []string{"r1", "r2"}, false, nil),
			},
			expected: []string{"r1", "r2", "child"},
		},
		{
			name: "original test case",
			entries: []Executor[string, any]{
				NewEntry[string, any]("test", "max", []string{}, false, nil),
				NewEntry[string, any]("abc2", "max", []string{"test"}, false, nil),
				NewEntry[string, any]("test2", "max", []string{"abc"}, true, nil),
				NewEntry[string, any]("test2-skip", "max", []string{"abc"}, true, func() bool { return true }),
				NewEntry[string, any]("test3", "max", []string{"abc", "abc2"}, true, nil),
				NewEntry[string, any]("test4", "max", []string{"test3"}, true, nil),
				NewEntry[string, any]("test5", "max", []string{"test3", "abc2"}, true, nil),
				NewEntry[string, any]("abc", "max", []string{}, false, nil),
			},
			expected:    []string{"test", "abc", "abc2", "test2", "test3", "test4", "test5"},
			notExpected: []string{"test2-skip"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTreeLayer[string, any]()
			err := tr.Add(tt.entries...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			executed := NewSafeSet[string]()
			tr.Execute(context.Background(), func(_ context.Context, key string, _ any) {
				executed.Add(key)
			})

			for _, key := range tt.expected {
				if !executed.Contains(key) {
					t.Errorf("expected %q to be executed", key)
				}
			}
			for _, key := range tt.notExpected {
				if executed.Contains(key) {
					t.Errorf("expected %q NOT to be executed", key)
				}
			}
		})
	}
}

func TestTreeCircularDependencies(t *testing.T) {
	tests := []struct {
		name    string
		entries []Executor[string, any]
	}{
		{
			name: "direct cycle A-B",
			entries: []Executor[string, any]{
				NewEntry[string, any]("a", "v", []string{"b"}, false, nil),
				NewEntry[string, any]("b", "v", []string{"a"}, false, nil),
			},
		},
		{
			name: "self cycle",
			entries: []Executor[string, any]{
				NewEntry[string, any]("a", "v", []string{"a"}, false, nil),
			},
		},
		{
			name: "deep cycle A-B-C-D-A",
			entries: []Executor[string, any]{
				NewEntry[string, any]("a", "v", []string{"b"}, false, nil),
				NewEntry[string, any]("b", "v", []string{"c"}, false, nil),
				NewEntry[string, any]("c", "v", []string{"d"}, false, nil),
				NewEntry[string, any]("d", "v", []string{"a"}, false, nil),
			},
		},
		{
			name: "cycle not involving entry root",
			entries: []Executor[string, any]{
				NewEntry[string, any]("root", "v", []string{"a"}, false, nil),
				NewEntry[string, any]("a", "v", []string{"b"}, false, nil),
				NewEntry[string, any]("b", "v", []string{"c"}, false, nil),
				NewEntry[string, any]("c", "v", []string{"a"}, false, nil),
			},
		},
		{
			name: "original deep test",
			entries: []Executor[string, any]{
				NewEntry[string, any]("root", "root", nil, false, nil),
				NewEntry[string, any]("test", "test", []string{"root", "test4"}, false, nil),
				NewEntry[string, any]("test2", "test2", []string{"test"}, false, nil),
				NewEntry[string, any]("test3", "test3", []string{"test2"}, false, nil),
				NewEntry[string, any]("test4", "test4", []string{"root", "test"}, false, nil),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTreeLayer[string, any]()
			err := tr.Add(tt.entries...)
			if !errors.Is(err, ErrCircuitDependency) {
				t.Fatalf("expected ErrCircuitDependency, got: %v", err)
			}
		})
	}
}

func TestTreeLayerOrdering(t *testing.T) {
	tests := []struct {
		name           string
		entries        []Executor[string, any]
		wantLayers     int
		wantNodesLayer map[int]int
	}{
		{
			name: "three layer chain",
			entries: []Executor[string, any]{
				NewEntry[string, any]("a", "v", nil, false, nil),
				NewEntry[string, any]("b", "v", []string{"a"}, false, nil),
				NewEntry[string, any]("c", "v", []string{"b"}, false, nil),
			},
			wantLayers:     3,
			wantNodesLayer: map[int]int{0: 1, 1: 1, 2: 1},
		},
		{
			name: "wide single layer",
			entries: []Executor[string, any]{
				NewEntry[string, any]("a", "v", nil, true, nil),
				NewEntry[string, any]("b", "v", nil, true, nil),
				NewEntry[string, any]("c", "v", nil, true, nil),
			},
			wantLayers:     1,
			wantNodesLayer: map[int]int{0: 3},
		},
		{
			name: "diamond is 3 layers",
			entries: []Executor[string, any]{
				NewEntry[string, any]("root", "v", nil, false, nil),
				NewEntry[string, any]("left", "v", []string{"root"}, true, nil),
				NewEntry[string, any]("right", "v", []string{"root"}, true, nil),
				NewEntry[string, any]("sink", "v", []string{"left", "right"}, false, nil),
			},
			wantLayers:     3,
			wantNodesLayer: map[int]int{0: 1, 1: 2, 2: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTreeLayer[string, any]()
			err := tr.Add(tt.entries...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := tr.GetLayersCount(); got != tt.wantLayers {
				t.Errorf("GetLayersCount() = %d, want %d", got, tt.wantLayers)
			}

			for layer, wantCount := range tt.wantNodesLayer {
				if got := tr.GetNodesInLayer(layer); got != wantCount {
					t.Errorf("GetNodesInLayer(%d) = %d, want %d", layer, got, wantCount)
				}
			}
		})
	}
}

func TestTreeDependencyOrder(t *testing.T) {
	tr := NewTreeLayer[string, any]()
	err := tr.Add(
		NewEntry[string, any]("a", "v", nil, false, nil),
		NewEntry[string, any]("b", "v", []string{"a"}, false, nil),
		NewEntry[string, any]("c", "v", []string{"b"}, false, nil),
	)
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	order := make([]string, 0, 3)

	tr.Execute(context.Background(), func(_ context.Context, key string, _ any) {
		mu.Lock()
		order = append(order, key)
		mu.Unlock()
	})

	if len(order) != 3 {
		t.Fatalf("expected 3 executions, got %d", len(order))
	}

	indexOf := func(k string) int {
		for i, v := range order {
			if v == k {
				return i
			}
		}
		return -1
	}

	if indexOf("a") > indexOf("b") {
		t.Error("a must execute before b")
	}
	if indexOf("b") > indexOf("c") {
		t.Error("b must execute before c")
	}
}

func TestTreeContextCancellation(t *testing.T) {
	tests := []struct {
		name       string
		cancelWhen string
	}{
		{name: "cancel before execution", cancelWhen: "before"},
		{name: "cancel during execution", cancelWhen: "during"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTreeLayer[string, any]()
			err := tr.Add(
				NewEntry[string, any]("a", "v", nil, false, nil),
				NewEntry[string, any]("b", "v", []string{"a"}, false, nil),
				NewEntry[string, any]("c", "v", []string{"b"}, true, nil),
				NewEntry[string, any]("d", "v", []string{"b"}, true, nil),
			)
			if err != nil {
				t.Fatal(err)
			}

			ctx, cancel := context.WithCancel(context.Background())

			if tt.cancelWhen == "before" {
				cancel()
			}

			var count atomic.Int64
			tr.Execute(ctx, func(_ context.Context, _ string, _ any) {
				if tt.cancelWhen == "during" {
					cancel()
				}
				count.Add(1)
				time.Sleep(5 * time.Millisecond)
			})
			cancel() // ensure cleanup

			if tt.cancelWhen == "before" && count.Load() > 0 {
				t.Error("no tasks should execute after pre-cancellation")
			}
		})
	}
}

func TestEmptyTree(t *testing.T) {
	tr := NewTreeLayer[string, any]()

	err := tr.Add()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tr.Execute(context.Background(), func(_ context.Context, _ string, _ any) {
		t.Fatal("should not execute anything")
	})
}

func TestExecuteNilExecutor(t *testing.T) {
	tr := NewTreeLayer[string, any]()
	_ = tr.Add(NewEntry[string, any]("a", "v", nil, false, nil))

	// should not panic
	tr.Execute(context.Background(), nil)
}

func TestTreeQueryMethods(t *testing.T) {
	tr := NewTreeLayer[string, any]()
	err := tr.Add(
		NewEntry[string, any]("root", "v", nil, true, nil),
		NewEntry[string, any]("child1", "v", []string{"root"}, false, nil),
		NewEntry[string, any]("child2", "v", []string{"root"}, true, nil),
		NewEntry[string, any]("grand", "v", []string{"child1", "child2"}, false, nil),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Layers", func(t *testing.T) {
		if got := tr.Layers(); got != 2 {
			t.Errorf("Layers() = %d, want 2", got)
		}
	})

	t.Run("GetLayersCount", func(t *testing.T) {
		if got := tr.GetLayersCount(); got != 3 {
			t.Errorf("GetLayersCount() = %d, want 3", got)
		}
	})

	t.Run("AsyncCount", func(t *testing.T) {
		if got := tr.AsyncCount(0); got != 1 {
			t.Errorf("AsyncCount(0) = %d, want 1", got)
		}
		if got := tr.AsyncCount(99); got != 0 {
			t.Errorf("AsyncCount(99) = %d, want 0", got)
		}
	})

	t.Run("SyncCount", func(t *testing.T) {
		if got := tr.SyncCount(1); got != 1 {
			t.Errorf("SyncCount(1) = %d, want 1", got)
		}
		if got := tr.SyncCount(99); got != 0 {
			t.Errorf("SyncCount(99) = %d, want 0", got)
		}
	})

	t.Run("GetNodesInLayer", func(t *testing.T) {
		if got := tr.GetNodesInLayer(0); got != 1 {
			t.Errorf("GetNodesInLayer(0) = %d, want 1", got)
		}
		if got := tr.GetNodesInLayer(1); got != 2 {
			t.Errorf("GetNodesInLayer(1) = %d, want 2", got)
		}
		if got := tr.GetNodesInLayer(99); got != 0 {
			t.Errorf("GetNodesInLayer(99) = %d, want 0", got)
		}
	})

	t.Run("GetNodeKeys", func(t *testing.T) {
		keys := tr.GetNodeKeys()
		if len(keys) != 4 {
			t.Errorf("GetNodeKeys() returned %d keys, want 4", len(keys))
		}
	})

	t.Run("GetNodeDependencies", func(t *testing.T) {
		deps := tr.GetNodeDependencies("grand")
		if len(deps) != 2 {
			t.Errorf("GetNodeDependencies(grand) = %d deps, want 2", len(deps))
		}

		deps = tr.GetNodeDependencies("root")
		if len(deps) != 0 {
			t.Errorf("GetNodeDependencies(root) = %d deps, want 0", len(deps))
		}

		deps = tr.GetNodeDependencies("nonexistent")
		if deps != nil {
			t.Errorf("GetNodeDependencies(nonexistent) should be nil")
		}
	})

	t.Run("GetPretty", func(t *testing.T) {
		pretty := tr.GetPretty()
		if pretty == "" {
			t.Error("GetPretty() returned empty string")
		}
		if len(pretty) < 20 {
			t.Errorf("GetPretty() too short: %q", pretty)
		}
	})
}

func TestRetrieve(t *testing.T) {
	tests := []struct {
		name      string
		entries   []Executor[string, any]
		layer     int
		wantAsync int
		wantSync  int
		wantNil   bool
	}{
		{
			name: "mixed layer",
			entries: []Executor[string, any]{
				NewEntry[string, any]("s1", "v", nil, false, nil),
				NewEntry[string, any]("a1", "v", nil, true, nil),
				NewEntry[string, any]("a2", "v", nil, true, nil),
			},
			layer:     0,
			wantAsync: 2,
			wantSync:  1,
		},
		{
			name: "invalid layer returns nil",
			entries: []Executor[string, any]{
				NewEntry[string, any]("a", "v", nil, false, nil),
			},
			layer:   99,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTreeLayer[string, any]()
			_ = tr.Add(tt.entries...)

			asyncCh, syncCh := tr.Retrieve(tt.layer)

			if tt.wantNil {
				if asyncCh != nil || syncCh != nil {
					t.Fatal("expected nil channels")
				}
				return
			}

			asyncCount := 0
			for range asyncCh {
				asyncCount++
			}
			syncCount := 0
			for range syncCh {
				syncCount++
			}

			if asyncCount != tt.wantAsync {
				t.Errorf("async count = %d, want %d", asyncCount, tt.wantAsync)
			}
			if syncCount != tt.wantSync {
				t.Errorf("sync count = %d, want %d", syncCount, tt.wantSync)
			}
		})
	}
}

func TestIntKeys(t *testing.T) {
	tr := NewTreeLayer[int, string]()
	err := tr.Add(
		NewEntry[int, string](1, "first", nil, false, nil),
		NewEntry[int, string](2, "second", []int{1}, true, nil),
		NewEntry[int, string](3, "third", []int{1, 2}, false, nil),
	)
	if err != nil {
		t.Fatal(err)
	}

	executed := NewSafeSet[int]()
	tr.Execute(context.Background(), func(_ context.Context, key int, _ string) {
		executed.Add(key)
	})

	if executed.Len() != 3 {
		t.Errorf("expected 3 executions, got %d", executed.Len())
	}
}

func TestDuplicateDependency(t *testing.T) {
	tr := NewTreeLayer[string, any]()
	err := tr.Add(
		NewEntry[string, any]("parent", "v", nil, false, nil),
		NewEntry[string, any]("child", "v", []string{"parent", "parent"}, false, nil),
	)
	if err != nil {
		t.Fatal(err)
	}

	deps := tr.GetNodeDependencies("child")
	if len(deps) != 1 {
		t.Errorf("expected 1 unique dependency, got %d", len(deps))
	}

	if tr.GetNodesInLayer(1) != 1 {
		t.Error("child should be in layer 1")
	}
}

func TestConcurrentExecute(t *testing.T) {
	tr := NewTreeLayer[string, any]()
	_ = tr.Add(
		NewEntry[string, any]("a", "v", nil, true, nil),
		NewEntry[string, any]("b", "v", nil, true, nil),
		NewEntry[string, any]("c", "v", []string{"a", "b"}, false, nil),
	)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var count atomic.Int64
			tr.Execute(context.Background(), func(_ context.Context, _ string, _ any) {
				count.Add(1)
			})
			if count.Load() != 3 {
				t.Errorf("expected 3 executions, got %d", count.Load())
			}
		}()
	}
	wg.Wait()
}

func TestMultipleAddCalls(t *testing.T) {
	tr := NewTreeLayer[string, any]()

	err := tr.Add(
		NewEntry[string, any]("a", "v", nil, false, nil),
		NewEntry[string, any]("b", "v", []string{"a"}, false, nil),
	)
	if err != nil {
		t.Fatal(err)
	}

	err = tr.Add(
		NewEntry[string, any]("c", "v", []string{"b"}, true, nil),
	)
	if err != nil {
		t.Fatal(err)
	}

	if tr.GetLayersCount() != 3 {
		t.Errorf("expected 3 layers, got %d", tr.GetLayersCount())
	}

	executed := NewSafeSet[string]()
	tr.Execute(context.Background(), func(_ context.Context, key string, _ any) {
		executed.Add(key)
	})

	for _, key := range []string{"a", "b", "c"} {
		if !executed.Contains(key) {
			t.Errorf("expected %q to be executed", key)
		}
	}
}

func TestGetPrettyContent(t *testing.T) {
	tr := NewTreeLayer[string, any]()
	_ = tr.Add(
		NewEntry[string, any]("root", "v", nil, true, nil),
		NewEntry[string, any]("child", "v", []string{"root"}, false, nil),
	)

	pretty := tr.GetPretty()

	for _, want := range []string{"Layer 0:", "Layer 1:", "root", "child", "Is Async: true", "Is Async: false"} {
		if !strings.Contains(pretty, want) {
			t.Errorf("GetPretty() missing %q", want)
		}
	}
}

func TestValuePassthrough(t *testing.T) {
	type payload struct{ data int }
	tr := NewTreeLayer[string, payload]()
	_ = tr.Add(
		NewEntry[string, payload]("a", payload{42}, nil, false, nil),
		NewEntry[string, payload]("b", payload{99}, []string{"a"}, true, nil),
	)

	values := NewSafeSet[int]()
	tr.Execute(context.Background(), func(_ context.Context, _ string, v payload) {
		values.Add(v.data)
	})

	if !values.Contains(42) || !values.Contains(99) {
		t.Error("values not passed through correctly")
	}
}

func TestLargeTreeCorrectness(t *testing.T) {
	tr := NewTreeLayer[int, int]()

	var entries []Executor[int, int]
	n := 200

	for i := 0; i < n; i++ {
		entries = append(entries, NewEntry[int, int](i, i, nil, i%2 == 0, nil))
	}

	for i := 0; i < n; i++ {
		entries = append(entries, NewEntry[int, int](n+i, n+i, []int{i}, i%3 == 0, nil))
	}

	err := tr.Add(entries...)
	if err != nil {
		t.Fatal(err)
	}

	var count atomic.Int64
	tr.Execute(context.Background(), func(_ context.Context, _ int, _ int) {
		count.Add(1)
	})

	if count.Load() != int64(2*n) {
		t.Errorf("expected %d executions, got %d", 2*n, count.Load())
	}
}

/*
BenchmarkTree-32                  236611              5474 ns/op            2177 B/op         45 allocs/op
BenchmarkTree4by100-32            142002              8362 ns/op            4928 B/op         32 allocs/op
*/

func BenchmarkTree(b *testing.B) {
	b.StopTimer()

	tr := NewTreeLayer[string, any]()

	err := tr.Add(
		NewEntry[string, any]("test", "max", []string{}, false, nil),
		NewEntry[string, any]("abc2", "max", []string{"test"}, false, nil),
		NewEntry[string, any]("test2", "max", []string{"abc"}, true, nil),
		NewEntry[string, any]("test3", "max", []string{"abc", "abc2"}, true, nil),
		NewEntry[string, any]("test4", "max", []string{"test3"}, true, nil),
		NewEntry[string, any]("test5", "max", []string{"test3"}, true, nil),
		NewEntry[string, any]("test6", "max", []string{"test3"}, true, nil),
		NewEntry[string, any]("abc", "max", []string{}, false, nil),
	)
	if err != nil {
		b.Fatal(err)
	}

	b.StartTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tr.Execute(context.Background(), func(_ context.Context, _ string, _ any) {})
		}
	})
}

func BenchmarkTree4by100(b *testing.B) {
	b.StopTimer()

	tr := NewTreeLayer[string, any]()

	var entries []Executor[string, any]
	for i := 0; i < 100; i++ {
		entries = append(entries, NewEntry[string, any]("r"+strconv.Itoa(i), "max", []string{}, true, nil))
	}

	for i := 0; i < 100; i++ {
		entries = append(entries, NewEntry[string, any]("lvl1"+strconv.Itoa(i), "max", []string{"r" + strconv.Itoa(i)}, false, nil))
	}

	for i := 0; i < 100; i++ {
		entries = append(entries, NewEntry[string, any]("lvl2"+strconv.Itoa(i), "max", []string{"lvl1" + strconv.Itoa(i)}, false, nil))
	}

	for i := 0; i < 100; i++ {
		entries = append(entries, NewEntry[string, any]("lvl3"+strconv.Itoa(i), "max", []string{"lvl2" + strconv.Itoa(i)}, false, nil))
	}

	err := tr.Add(entries...)
	if err != nil {
		b.Fatal(err)
	}

	b.StartTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tr.Execute(context.Background(), func(_ context.Context, _ string, _ any) {})
		}
	})
}

func BenchmarkTreeAdd(b *testing.B) {
	for b.Loop() {
		tr := NewTreeLayer[string, any]()

		_ = tr.Add(
			NewEntry[string, any]("test", "max", nil, false, nil),
			NewEntry[string, any]("abc", "max", nil, false, nil),
			NewEntry[string, any]("abc2", "max", []string{"test"}, false, nil),
			NewEntry[string, any]("test2", "max", []string{"abc"}, true, nil),
			NewEntry[string, any]("test3", "max", []string{"abc", "abc2"}, true, nil),
			NewEntry[string, any]("test4", "max", []string{"test3"}, true, nil),
			NewEntry[string, any]("test5", "max", []string{"test3"}, true, nil),
		)
	}
}
