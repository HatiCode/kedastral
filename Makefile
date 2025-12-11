.PHONY: all build test clean proto help forecaster scaler

# Default target
all: build

# Build both executables
build: forecaster scaler

# Build forecaster
forecaster:
	@echo "Building forecaster..."
	@go build -o bin/forecaster ./cmd/forecaster

# Build scaler
scaler:
	@echo "Building scaler..."
	@go build -o bin/scaler ./cmd/scaler

# Run all tests
test:
	@echo "Running tests..."
	@go test ./... -v

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test ./... -coverprofile=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	@cd pkg/api/externalscaler && protoc \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		externalscaler.proto
	@echo "Protobuf code generated"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

# Install dev dependencies
install-tools:
	@echo "Installing development tools..."
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Tools installed"

# Run forecaster (for local dev)
run-forecaster:
	@echo "Running forecaster..."
	@go run ./cmd/forecaster \
		-workload=demo-api \
		-metric=http_rps \
		-prom-query='sum(rate(http_requests_total[1m]))' \
		-prom-url=http://localhost:9090 \
		-log-level=debug

# Run scaler (for local dev)
run-scaler:
	@echo "Running scaler..."
	@go run ./cmd/scaler \
		-forecaster-url=http://localhost:8081 \
		-log-level=debug

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	@go mod tidy

# Help
help:
	@echo "Kedastral Makefile targets:"
	@echo ""
	@echo "  make build           - Build both forecaster and scaler binaries"
	@echo "  make forecaster      - Build forecaster binary only"
	@echo "  make scaler          - Build scaler binary only"
	@echo "  make test            - Run all tests"
	@echo "  make test-coverage   - Run tests with coverage report"
	@echo "  make proto           - Regenerate protobuf code"
	@echo "  make clean           - Remove build artifacts"
	@echo "  make install-tools   - Install development tools"
	@echo "  make run-forecaster  - Run forecaster locally"
	@echo "  make run-scaler      - Run scaler locally"
	@echo "  make fmt             - Format code"
	@echo "  make lint            - Run linter"
	@echo "  make tidy            - Tidy Go modules"
	@echo "  make help            - Show this help message"
