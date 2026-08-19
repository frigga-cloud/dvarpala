# Dvarpala - tasks for working on the code.
#
# Installing onto a real machine is not done from here: that is
# scripts/install/install-dvarpala.sh, which builds from source on the target.

BINARY_DIR  = bin
CONFIG_FILE = configs/environment.yaml

.PHONY: build clean deps dev test test-coverage fmt lint help

# Build the two binaries that are actually deployed.
build:
	@mkdir -p $(BINARY_DIR)
	go build -o $(BINARY_DIR)/dvarpala-server ./cmd/dvarpala-server
	go build -o $(BINARY_DIR)/dvarpala-cli    ./cmd/dvarpala-cli

clean:
	rm -rf $(BINARY_DIR)
	go clean

deps:
	go mod download
	go mod tidy

# Run the server locally. Needs PostgreSQL and Redis already running.
dev: build
	./$(BINARY_DIR)/dvarpala-server --config $(CONFIG_FILE)

# Tests that need Redis skip themselves when it is not running.
test:
	go test ./...

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

fmt:
	go fmt ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

help:
	@echo "Dvarpala"
	@echo ""
	@echo "  build          build dvarpala-server and dvarpala-cli into $(BINARY_DIR)/"
	@echo "  deps           download and tidy modules"
	@echo "  dev            build, then run the server against $(CONFIG_FILE)"
	@echo "  test           run the test suite"
	@echo "  test-coverage  run tests and write coverage.html"
	@echo "  fmt            gofmt the tree"
	@echo "  lint           golangci-lint, if installed"
	@echo "  clean          remove build artefacts"
	@echo ""
	@echo "To install onto a server:"
	@echo "  sudo scripts/install/install-dvarpala.sh --source . --host <address>"
