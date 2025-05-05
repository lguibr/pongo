#!/bin/bash

# PonGo Backend Build & Test Script
# This script cleans, builds, tests (unit & E2E), runs stress tests,
# analyzes metrics, and generates coverage reports.

# Exit immediately if a command exits with a non-zero status.
set -e

# --- Configuration ---
COVERAGE_FILE="coverage.out"
COVERAGE_HTML="coverage.html"
STRESS_METRICS_FILE="stress_metrics.log"
CPU_PROFILE_FILE="cpu.prof"
MEM_PROFILE_FILE="mem.prof"

# --- Helper Functions ---
print_header() {
    echo -e "\n\033[1;33m>>> $1 <<<\033[0m"
}

print_success() {
    echo -e "\033[0;32m✔ Success: $1\033[0m"
}

print_error() {
    echo -e "\033[0;31m✘ Error: $1\033[0m"
}

print_warning() {
    echo -e "\033[1;33m⚠ Warning: $1\033[0m"
}

run_command() {
    local description=$1
    local command=$2
    echo -e "⏳ Running: $command"
    if eval "$command"; then
        print_success "$description"
        return 0
    else
        print_error "$description failed."
        return 1 # Propagate failure
    fi
}

# --- Main Script ---
# Trap errors to print a summary message
trap 'print_error "Build and Test Summary"; print_error "One or more steps failed."; exit 1' ERR

# 1. Clean Caches
print_header "Cleaning Caches"
run_command "Go cache cleaning" "go clean -cache -testcache -modcache"

# 2. Tidy Modules
print_header "Tidying and Vendoring Modules"
run_command "Go module tidy" "go mod tidy"
# Optional: Vendor dependencies (uncomment if needed)
# run_command "Go module vendoring" "go mod vendor"

# 3. Verify Build (Compile packages and tests without running tests)
print_header "Verifying Build (Compiling Packages and Tests)"
run_command "Project and test compilation" "go test -v -run=^$ ./..."

# 4. Run Unit Tests with Coverage
print_header "Running Unit Tests"
# Target specific packages containing unit tests
# Exclude 'test' package which contains E2E tests
UNIT_TEST_PACKAGES=$(go list ./... | grep -v '/test$' | tr '\n' ' ')
echo "Targeting packages for unit tests: $UNIT_TEST_PACKAGES"
run_command "Unit tests (with coverage)" "go test -v -race -covermode=atomic -coverprofile=$COVERAGE_FILE $UNIT_TEST_PACKAGES"

# 5. Run Standard E2E Tests (Non-Stress)
print_header "Running Standard E2E Tests"
# Target only the 'test' package and run tests matching ^TestE2E_ but NOT StressTest*
# Use -run 'TestE2E_(SinglePlayer|BallWall)' or similar specific patterns if needed
# Or use grep -v to exclude stress tests
E2E_TEST_PATTERN='^TestE2E_(SinglePlayer|BallWall)' # Adjust this pattern as needed
run_command "Standard E2E tests" "go test ./test -v -race -run \"$E2E_TEST_PATTERN\""


# 6. Run Stress Tests with Profiling (Output to $STRESS_METRICS_FILE)
print_header "Running Stress Tests with Profiling (Output to $STRESS_METRICS_FILE, Profiles to $CPU_PROFILE_FILE, $MEM_PROFILE_FILE)"
# Run stress tests (e.g., TestE2E_StressTest*) without -short flag
# Add profiling flags. Output files will be created in the current directory.
# Redirect stdout/stderr to capture metrics.
# Choose ONE stress test to profile, e.g., TestE2E_StressTestMultipleRooms
STRESS_TEST_TO_PROFILE='^TestE2E_StressTestMultipleRooms$' # Use $ to match end of string for specificity
STRESS_COMMAND="go test ./test -v -race -run \"$STRESS_TEST_TO_PROFILE\" -timeout 90s -cpuprofile \"$CPU_PROFILE_FILE\" -memprofile \"$MEM_PROFILE_FILE\" > \"$STRESS_METRICS_FILE\" 2>&1"
echo -e "⏳ Running: $STRESS_COMMAND"
if eval "$STRESS_COMMAND"; then
    print_success "Stress test with profiling command completed."
    echo "CPU profile saved to: $CPU_PROFILE_FILE"
    echo "Memory profile saved to: $MEM_PROFILE_FILE"
    echo "Analyze with: go tool pprof $CPU_PROFILE_FILE"
    echo "Analyze with: go tool pprof $MEM_PROFILE_FILE"
else
    # Don't exit script on stress test failure, just warn
    print_warning "Stress test with profiling command failed or timed out. Check $STRESS_METRICS_FILE for details."
    # Profiles might still be partially written or empty on failure/timeout
    if [ -f "$CPU_PROFILE_FILE" ]; then echo "CPU profile might exist but could be incomplete: $CPU_PROFILE_FILE"; fi
    if [ -f "$MEM_PROFILE_FILE" ]; then echo "Memory profile might exist but could be incomplete: $MEM_PROFILE_FILE"; fi
fi


# 7. Analyze Stress Test Metrics
print_header "Analyzing Stress Test Metrics from $STRESS_METRICS_FILE"
# Example: grep for average tick time metric
AVG_TICK_TIME=$(grep 'PERF_METRIC.*AvgPhysicsTick' "$STRESS_METRICS_FILE" | tail -n 1 | awk -F '=' '{print $2}' | awk '{print $1}')

if [ -n "$AVG_TICK_TIME" ]; then
    echo "Average Physics Tick Time (from last relevant log): $AVG_TICK_TIME"
    print_success "Stress test metric analysis (basic)"
else
    print_warning "Could not calculate average tick time (no metrics found in $STRESS_METRICS_FILE)."
fi
echo "Raw stress test metrics saved to: $STRESS_METRICS_FILE"


# 8. Generate HTML Coverage Report
print_header "Generating Coverage Report (HTML)"
run_command "HTML coverage report generation" "go tool cover -html=$COVERAGE_FILE -o $COVERAGE_HTML"
print_success "Coverage report generated: $COVERAGE_HTML"

# --- Final Summary ---
print_header "Build and Test Summary"
print_success "All steps completed successfully (Stress test failures are warnings)."

exit 0