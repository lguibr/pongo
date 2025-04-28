#!/bin/bash

# Exit immediately if a command exits with a non-zero status.
set -e

# Define colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# --- Configuration ---
COVERAGE_FILE="coverage.out"
COVERAGE_HTML="coverage.html"
STRESS_LOG_FILE="stress_metrics.log"
# Define Performance Threshold (in milliseconds)
# Adjust this value based on expected performance on your CI/target machine
MAX_AVG_TICK_MS=10 # Example: Fail if average tick time exceeds 10ms

# --- Helper Functions ---
print_step() {
  echo -e "\n${YELLOW}--- $1 ---${NC}"
}

print_success() {
  echo -e "${GREEN}Success: $1${NC}"
}

print_error() {
  echo -e "${RED}Error: $1${NC}" >&2 # Print errors to stderr
}

# --- Script Steps ---

# 1. Clean Caches
print_step "Cleaning Caches"
go clean -cache -testcache -modcache

# 2. Tidy & Vendor Modules
print_step "Tidying and Vendoring Modules"
go mod tidy
# go mod vendor # Optional: uncomment if you use vendoring

# 3. Build Project
print_step "Building Project"
go build -v ./...

# 4. Run Unit Tests with Race Detector and Coverage
# Run tests for all packages except the 'test' package (E2E/Stress)
print_step "Running Unit Tests with Race Detector and Coverage"
# Exclude the test directory from unit test coverage calculation if desired
# Use go list to get all packages except the test package
UNIT_TEST_PACKAGES=$(go list ./... | grep -v '/test')
if go test -v -race -covermode=atomic -coverprofile="$COVERAGE_FILE" $UNIT_TEST_PACKAGES; then
  print_success "Unit tests passed."
else
  print_error "Unit tests failed."
  exit 1
fi

# 5. Run E2E and Stress Tests (separately to capture stress logs)
print_step "Running E2E & Stress Tests"

# Run standard E2E tests first
if go test ./test -v -race -run "^TestE2E_SinglePlayer"; then
   print_success "Standard E2E tests passed."
else
   print_error "Standard E2E tests failed."
   # Decide if this should be a hard fail or just a warning
   # exit 1
fi

# Run Stress Test and capture output
print_step "Running Stress Test (Output to $STRESS_LOG_FILE)"
# Use -run flag to specifically target the stress test
# Redirect both stdout and stderr to the log file
if go test ./test -v -race -run "^TestE2E_StressTestMultipleRooms$" > "$STRESS_LOG_FILE" 2>&1; then
   print_success "Stress test completed."
else
   print_error "Stress test failed. Check $STRESS_LOG_FILE for details."
   # Optionally exit here if stress test failure is critical
   # exit 1
fi

# 6. Analyze Stress Test Metrics (Basic Example)
print_step "Analyzing Stress Test Metrics from $STRESS_LOG_FILE"
# Use grep to find metric lines, awk to extract the duration (assuming format "Avg Tick Duration: Xs"),
# sed to remove 's' and convert to milliseconds, then calculate average.
# This is a basic example, might need refinement based on actual log format and desired metric (avg, max, p95 etc.)
AVG_TICK_MS=$(grep 'PERF_METRIC.*Avg Tick Duration:' "$STRESS_LOG_FILE" | \
              awk '{print $7}' | \
              sed 's/ms/ \* 1/g; s/µs/ \* 0.001/g; s/s$/ \* 1000/g' | \
              bc -l | \
              awk '{ total += $1; count++ } END { if (count > 0) printf "%.2f", total / count; else print "0" }')

echo "Overall Average Tick Duration from Stress Test: ${AVG_TICK_MS} ms"

# Check against threshold
# Use bc for floating point comparison
if (( $(echo "$AVG_TICK_MS > $MAX_AVG_TICK_MS" | bc -l) )); then
    print_error "Performance regression detected! Average tick time (${AVG_TICK_MS}ms) exceeded threshold (${MAX_AVG_TICK_MS}ms)."
    # exit 1 # Make it a hard failure in CI
else
    print_success "Performance metrics within threshold (${MAX_AVG_TICK_MS}ms)."
fi
echo "Raw stress test metrics saved to: $STRESS_LOG_FILE"


# 7. Generate Coverage Report (HTML) - Based on Unit Tests
print_step "Generating Coverage Report (HTML)"
if [ -f "$COVERAGE_FILE" ]; then
  go tool cover -html="$COVERAGE_FILE" -o "$COVERAGE_HTML"
  print_success "Coverage report generated: $COVERAGE_HTML"
else
  print_error "Coverage file ($COVERAGE_FILE) not found. Skipping HTML report generation."
fi


# --- Final Success Message ---
print_step "Build and Test Complete"
print_success "All steps finished."

exit 0