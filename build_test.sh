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

echo "--- Running Unit Tests with Race Detector and Coverage ---"
# Run tests verbosely, with race detector, generate coverage profile
# Exclude the E2E test package from the main run if desired, or let it run here too
# Example excluding: go test -v -race -coverprofile=coverage.out -covermode=atomic $(go list ./... | grep -v /test)
go test -v -race -coverprofile=coverage.out -covermode=atomic ./...

echo "--- Running E2E Test (Connect, Move, Disconnect) ---" # Updated description
# Run the specific E2E test, also with race detector for consistency
# Updated test name here:
go test -v -race ./test -run TestE2E_SinglePlayerConnectMoveDisconnect -count=1

echo "--- Generating Coverage Report (HTML) ---"
# Generate HTML coverage report (opens in browser)
# Note: Coverage from the E2E test won't be included unless run with coverage flags
go tool cover -html=coverage.out -o coverage.html
echo "Coverage report generated: coverage.html"

# Optional: Generate function coverage summary
# echo "--- Generating Coverage Report (Function Summary) ---"
# go tool cover -func=coverage.out

echo "--- Build and Test Complete ---"