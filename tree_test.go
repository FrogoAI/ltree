package ltree

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/FrogoAI/ltree/set"
)

func TestTree(t *testing.T) {
	tr := NewTreeLayer[string, any]()

	err := tr.Add(
		NewEntry[string, any]("test", "max", []string{}, false, nil),       // 0
		NewEntry[string, any]("abc2", "max", []string{"test"}, false, nil), // 1
		NewEntry[string, any]("test2", "max", []string{"abc"}, true, nil),  // 1
		NewEntry[string, any]("test2-skip", "max", []string{"abc"}, true, func() bool {
			return true
		}), // 1
		NewEntry[string, any]("test3", "max", []string{"abc", "abc2"}, true, nil),   // 2
		NewEntry[string, any]("test4", "max", []string{"test3"}, true, nil),         // 3
		NewEntry[string, any]("test5", "max", []string{"test3", "abc2"}, true, nil), // 4
		NewEntry[string, any]("abc", "max", []string{}, false, nil),                 // 0
	)
	if err != nil {
		t.Fatal(err)
	}

	executed := set.NewGenericDataSet[string]()

	tr.Execute(context.Background(), func(_ context.Context, key string, _ any) {
		// just measure go one-by-one without calculation
		executed.Add(key)
	})

	if !executed.Contains("test2") {
		t.Fatal("Does not contains test2")
	}

	if executed.Contains("test2-skip") {
		t.Fatal("Contains test2-skip, but should not")
	}
}

func TestTreeCircuitDependencies(t *testing.T) {
	tr := NewTreeLayer[string, any]()
	err := tr.Add(
		NewEntry[string, any]("test", "max", []string{"abc"}, false, nil), // 1
		NewEntry[string, any]("abc", "max", []string{"test"}, false, nil), // 1
	)

	if !errors.Is(err, ErrCircuitDependency) {
		t.Fatal("We expect to have error here")
	}
}

func TestEmptyTree(t *testing.T) {
	tr := NewTreeLayer[string, any]()
	err := tr.Add()
	if err == nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	tr.Execute(context.Background(), func(_ context.Context, _ string, _ any) {})
}

func TestTreeDeepCircuitDependencies(t *testing.T) {
	tr := NewTreeLayer[string, any]()
	err := tr.Add(
		NewEntry[string, any]("root", "root", []string{}, false, nil),                 // 0
		NewEntry[string, any]("test", "test", []string{"root", "test4"}, false, nil),  // 1
		NewEntry[string, any]("test2", "test2", []string{"test"}, false, nil),         // 2
		NewEntry[string, any]("test3", "test3", []string{"test2"}, false, nil),        // 3
		NewEntry[string, any]("test4", "test4", []string{"root", "test"}, false, nil), // 4
	)

	if !errors.Is(err, ErrCircuitDependency) {
		t.Fatal("We expect to have error here")
	}
}

func TestTreeBreak(t *testing.T) {
	tr := NewTreeLayer[string, any]()

	err := tr.Add(
		NewEntry[string, any]("test", "max", []string{}, false, nil),                // 0
		NewEntry[string, any]("abc2", "max", []string{"test"}, false, nil),          // 1
		NewEntry[string, any]("test2", "max", []string{"abc"}, true, nil),           // 1
		NewEntry[string, any]("test3", "max", []string{"abc", "abc2"}, true, nil),   // 2
		NewEntry[string, any]("test4", "max", []string{"test3"}, true, nil),         // 3
		NewEntry[string, any]("test5", "max", []string{"test3", "abc2"}, true, nil), // 4
		NewEntry[string, any]("abc", "max", []string{}, false, nil),                 // 0
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.TODO())

	go func() {
		cancel()
	}()

	tr.Execute(ctx, func(_ context.Context, _ string, _ any) {
		time.Sleep(10 * time.Millisecond)
	})
}

/*
BenchmarkTree-32                  236611              5474 ns/op            2177 B/op         45 allocs/op
BenchmarkTree4by100-32            142002              8362 ns/op            4928 B/op         32 allocs/op
*/

func BenchmarkTree(b *testing.B) {
	b.StopTimer()

	tr := NewTreeLayer[string, any]()

	err := tr.Add(
		NewEntry[string, any]("test", "max", []string{}, false, nil),              // 0
		NewEntry[string, any]("abc2", "max", []string{"test"}, false, nil),        // 1
		NewEntry[string, any]("test2", "max", []string{"abc"}, true, nil),         // 1
		NewEntry[string, any]("test3", "max", []string{"abc", "abc2"}, true, nil), // 2
		NewEntry[string, any]("test4", "max", []string{"test3"}, true, nil),       // 3
		NewEntry[string, any]("test5", "max", []string{"test3"}, true, nil),       // 3
		NewEntry[string, any]("test6", "max", []string{"test3"}, true, nil),       // 3
		NewEntry[string, any]("abc", "max", []string{}, false, nil),               // 0
	)
	if err != nil {
		b.Fatal(err)
	}

	b.StartTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tr.Execute(context.Background(), func(_ context.Context, _ string, _ any) {
				// just measure go one-by-one without calculation
			})
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
			tr.Execute(context.Background(), func(_ context.Context, _ string, _ any) {
				// just measure go one-by-one without calculation
			})
		}
	})
}
