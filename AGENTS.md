# ltree — Layered DAG Executor

## What this is

A high-performance Go library that resolves a dependency graph (DAG) into execution layers and runs tasks layer-by-layer with concurrency control. Used in hot paths at 1000+ TPS — performance and correctness are critical.

## Architecture

```
Entry (implements Executor) → Add() → TreeLayer (builds DAG + layers) → Execute()
```

- **`Executor`** (`executor.go`) — interface: `Key()`, `Value()`, `DependsOn()`, `IsAsync()`, `Skip()`
- **`Entry`** (`entry.go`) — default `Executor` implementation
- **`Node`** (`node.go`) — internal DAG node with parent links and DFS state (`stateUnvisited`/`stateVisiting`/`stateVisited`)
- **`TreeLayer`** (`layer.go`) — the core: builds the layered DAG via `Add()`, executes via `Execute()`
- **`Layer`** (`layer.go`) — groups nodes at the same depth, tracks async/sync counts

### Key invariant

A node's layer = max(parent layers) + 1. `Execute()` processes layers 0..N sequentially. Within a layer, async nodes each get their own goroutine; sync nodes run inline in the caller's goroutine. `sync.WaitGroup` gates layer transitions. A single `RLock` is held for the full Execute duration.

### Cycle detection

Uses DFS 3-color marking in `visit()`. A node in `stateVisiting` encountered during traversal means a back-edge (cycle) → returns `ErrCircuitDependency`. This catches all cycles including those not involving the DFS root.

## Commands

```
make test              # go test -race -count=1 ./...
make lint              # golangci-lint run ./...
make coverage          # tests + go-test-coverage (threshold: 90%)
make bench             # 6-count benchmem → benchmarks/current.txt
make bench-baseline    # same → benchmarks/baseline.txt (committed)
make bench-compare     # benchstat baseline vs current
make ci                # lint + coverage
```

## Conventions

- Go 1.25+, generics throughout (`[K comparable, V any]`)
- Tests are table-driven with `t.Run` subtests
- Benchmarks use `b.RunParallel` for execution and `b.Loop()` for build
- No external test dependencies — `SafeSet` is defined locally in tests
- Coverage must stay >= 90% (enforced by `.testcoverage.yml`)
- `benchmarks/baseline.txt` is committed; `benchmarks/current.txt` is gitignored

## Performance notes

- `Execute()` uses direct goroutine spawning (no channels) — one goroutine per async node, sync nodes run inline in caller (zero-alloc per layer for sync-only layers)
- `Execute()` holds a single RLock for its full duration — no per-layer lock thrashing, and prevents `Add()` from corrupting mid-execution state
- No external dependencies (zero entries in go.mod)
- `Retrieve()` uses buffered channels sized exactly to node counts (no goroutine leak)
- `Add()` is not safe to call concurrently with `Execute()`; `Execute()` is safe to call concurrently from multiple goroutines (read-only after build)

## Rules
1. **Use `#AI-assisted` in commit messages**.
2. **Always run linter, tests, and benchmarks after every change** — `make lint && make test && make bench`. Fix all issues before committing. Never push code that fails lint or tests.
