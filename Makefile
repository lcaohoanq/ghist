.PHONY: build test check
build:
	go build -o bin/ghist ./cmd/ghist

test:
	go test -race ./...

check:
	go vet ./...
	go test -race ./...

.PHONY: extension-build extension-test extension-webview-test extension-package
extension-build:
	npm --prefix extensions/vscode run build

extension-test:
	npm --prefix extensions/vscode test

extension-webview-test:
	npm --prefix extensions/vscode run test:webview

extension-package:
	npm --prefix extensions/vscode run package

# Example: make dev FILE=internal/tui/view.go
# Example: make dev PROJECT=/path/to/repo FILE=src/main.go
.PHONY: dev
export FILE PROJECT
dev:
	air -c .air.toml
