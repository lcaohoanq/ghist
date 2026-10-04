.PHONY: build test check
build:
	go build -o bin/ghist ./cmd/ghist

test:
	go test -race ./...

check:
	go vet ./...
	go test -race ./...

# Example: make dev FILE=internal/tui/view.go
# Example: make dev PROJECT=/path/to/repo FILE=src/main.go
.PHONY: dev
export FILE PROJECT
dev:
	air -c .air.toml
