#!/bin/bash

# Exit immediately if a command exits with a non-zero status.
set -e

echo "--- Cleaning Caches ---"
go clean -cache
go clean -testcache
go clean -modcache # More aggressive module cache cleaning

echo "--- Tidying and Vendoring Modules ---"
go mod tidy
go mod vendor

# Optional: Run linter (uncomment if you have golangci-lint installed)
# echo "--- Running Linter ---"
# golangci-lint run ./...

echo "--- Building Project ---"
go build -v ./... # Build all packages verbosely

echo "--- Running Tests with Race Detector and Coverage ---"
# Run tests verbosely, with race detector, generate coverage profile
go test -v -race -coverprofile=coverage.out -covermode=atomic ./...

echo "--- Generating Coverage Report (HTML) ---"
# Generate HTML coverage report (opens in browser)
go tool cover -html=coverage.out -o coverage.html
echo "Coverage report generated: coverage.html"

# Optional: Generate function coverage summary
# echo "--- Generating Coverage Report (Function Summary) ---"
# go tool cover -func=coverage.out

echo "--- Build and Test Complete ---"