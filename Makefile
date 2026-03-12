.PHONY: test lint coverage bench bench-baseline bench-compare clean ci

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

coverage:
	go test -coverprofile=coverage.out -cover -race ./...
	go-test-coverage --config=./.testcoverage.yml

bench:
	@mkdir -p benchmarks
	go test -bench=. -benchmem -count=6 -run="^$$" -timeout=300s ./... \
		| tee benchmarks/current.txt

bench-baseline:
	@mkdir -p benchmarks
	go test -bench=. -benchmem -count=6 -run="^$$" -timeout=300s ./... \
		| tee benchmarks/baseline.txt
	@echo "\nBaseline updated. Run 'git add benchmarks/baseline.txt && git commit' to persist."

bench-compare:
	@if [ ! -f benchmarks/current.txt ]; then echo "Run 'make bench' first."; exit 1; fi
	benchstat benchmarks/baseline.txt benchmarks/current.txt

ci: lint coverage

clean:
	rm -f coverage.out
	rm -f benchmarks/current.txt
