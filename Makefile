.PHONY: build test check
build:
	go build -o bin/ghist ./cmd/ghist

test:
	go test -race ./...

check:
	go vet ./...
	go test -race ./...

# Example: make dev FILE=internal/tui/view.go
.PHONY: dev
export FILE
dev:
	@test -n "$$FILE" || { echo 'Usage: make dev FILE=path/to/tracked-file'; exit 1; }
	air -c .air.toml
