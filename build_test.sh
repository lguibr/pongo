#!/bin/bash

# PonGo Backend Build & Test Script
#
# Features:
# - Cleans caches
# - Tidies modules
# - Verifies build including test compilation
# - Runs Unit Tests with Race Detector & Coverage
# - Runs E2E Tests with Race Detector
# - Runs Stress Tests with Race Detector & Captures Metrics
# - Analyzes basic performance metrics from stress test logs
# - Generates HTML coverage report
#
# Options:
#   -s, --skip-stress      Skip running the stress tests (saves time)
#   -c, --skip-coverage    Skip generating the HTML coverage report
#   -f, --fail-fast        Exit immediately if any test stage fails
#   -h, --help             Show this help message

# --- Configuration ---
COVERAGE_FILE="coverage.out"
COVERAGE_HTML="coverage.html"
STRESS_LOG_FILE="stress_metrics.log"
# Define Performance Threshold (in milliseconds)
# Adjust this value based on expected performance on your CI/target machine
MAX_AVG_TICK_MS=${MAX_AVG_TICK_MS:-15} # Default to 15ms, allow override via env var

# --- Flags ---
SKIP_STRESS=false
SKIP_COVERAGE=false
FAIL_FAST=false

# --- Argument Parsing ---
while [[ "$#" -gt 0 ]]; do
    case $1 in
        -s|--skip-stress) SKIP_STRESS=true ;;
        -c|--skip-coverage) SKIP_COVERAGE=true ;;
        -f|--fail-fast) FAIL_FAST=true ;;
        -h|--help)
            echo "Usage: $0 [-s] [-c] [-f]"
            echo "  -s, --skip-stress      Skip running the stress tests"
            echo "  -c, --skip-coverage    Skip generating the HTML coverage report"
            echo "  -f, --fail-fast        Exit immediately if any test stage fails"
            exit 0
            ;;
        *) echo "Unknown parameter passed: $1"; exit 1 ;;
    esac
    shift
done

# --- Script Setup ---
# Exit immediately if a command exits with a non-zero status.
set -e
# Keep track of failures
FAILURE_DETECTED=false

# Define colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# --- Helper Functions ---
print_step() {
  echo -e "\n${YELLOW}>>> $1 <<<${NC}"
}

print_success() {
  echo -e "${GREEN}✔ Success: $1${NC}"
}

print_warning() {
  echo -e "${YELLOW}⚠ Warning: $1${NC}"
}

print_error() {
  echo -e "${RED}✘ Error: $1${NC}" >&2 # Print errors to stderr
  FAILURE_DETECTED=true
  if [ "$FAIL_FAST" = true ]; then
    echo -e "${RED}Exiting due to --fail-fast flag.${NC}"
    exit 1
  fi
}

# Function to run a command and handle errors based on FAIL_FAST
run_command() {
  local description="$1"
  shift # Remove description from arguments
  local command=("$@")

  echo "⏳ Running: ${command[*]}"
  if "${command[@]}"; then
    print_success "$description"
  else
    print_error "$description failed."
    # Error handling is done by print_error based on FAIL_FAST
  fi
}

# --- Script Steps ---

# 1. Clean Caches
print_step "Cleaning Caches"
run_command "Go cache cleaning" go clean -cache -testcache -modcache

# 2. Tidy & Vendor Modules
print_step "Tidying and Vendoring Modules"
run_command "Go module tidy" go mod tidy
# run_command "Go module vendor" go mod vendor # Optional: uncomment if you use vendoring

# 3. Verify Build (Compiles Packages and Tests)
# Uses `go test -run=^$` which compiles tests but runs none.
print_step "Verifying Build (Compiling Packages and Tests)"
run_command "Project and test compilation" go test -v -run=^$ ./...

# 4. Run Unit Tests with Race Detector and Coverage
print_step "Running Unit Tests"
# Exclude the test directory from unit test coverage calculation
# Use go list to get all packages except the test package
UNIT_TEST_PACKAGES=$(go list ./... | grep -v '/test$' || true) # Use '|| true' to avoid error if grep finds nothing
if [ -z "$UNIT_TEST_PACKAGES" ]; then
    print_warning "No packages found for unit testing (excluding /test)."
else
    echo "Targeting packages for unit tests: $UNIT_TEST_PACKAGES"
    if [ "$SKIP_COVERAGE" = true ]; then
        run_command "Unit tests (no coverage)" go test -v -race $UNIT_TEST_PACKAGES
    else
        run_command "Unit tests (with coverage)" go test -v -race -covermode=atomic -coverprofile="$COVERAGE_FILE" $UNIT_TEST_PACKAGES
    fi
fi

# 5. Run E2E Tests
print_step "Running E2E Tests"
# Target only tests starting with TestE2E_SinglePlayer in the test package
run_command "Standard E2E tests" go test ./test -v -race -run "^TestE2E_SinglePlayer"

# 6. Run Stress Tests (Conditional)
if [ "$SKIP_STRESS" = true ]; then
  print_step "Skipping Stress Tests"
else
  print_step "Running Stress Tests (Output to $STRESS_LOG_FILE)"
  # Use -run flag to specifically target the stress tests
  # Redirect both stdout and stderr to the log file
  # Use a subshell to capture exit code without `set -e` stopping the script
  STRESS_TEST_PASSED=true
  ( set +e; go test ./test -v -race -run "^TestE2E_StressTest" > "$STRESS_LOG_FILE" 2>&1; exit $? )
  if [ $? -ne 0 ]; then
      STRESS_TEST_PASSED=false
      print_error "Stress test command failed. Check $STRESS_LOG_FILE for details."
  else
      print_success "Stress test command completed."
  fi

  # 7. Analyze Stress Test Metrics (Basic Example)
  print_step "Analyzing Stress Test Metrics from $STRESS_LOG_FILE"
  # Use grep to find metric lines, awk to extract the duration value and unit.
  # Handles ms, µs, s units. Calculates average.
  AVG_TICK_MS=$(grep 'PERF_METRIC.*AvgTick=' "$STRESS_LOG_FILE" | \
                awk -F'[ =]+' '{print $6}' | \
                awk '{ \
                    val = substr($1, 1, length($1)-2); \
                    unit = substr($1, length($1)-1); \
                    if (unit == "ms") multiplier = 1; \
                    else if (unit == "µs") multiplier = 0.001; \
                    else if (unit == "s") multiplier = 1000; \
                    else multiplier = 0; \
                    print val * multiplier \
                }' | \
                awk '{ total += $1; count++ } END { if (count > 0) printf "%.2f", total / count; else print "N/A" }')


  if [[ "$AVG_TICK_MS" == "N/A" ]]; then
      print_warning "Could not calculate average tick time (no metrics found in $STRESS_LOG_FILE)."
      # If the stress test itself failed, this might be expected.
      if [ "$STRESS_TEST_PASSED" = false ]; then
          print_error "Stress test failed, performance check skipped."
      fi
  else
      echo "Overall Average Tick Duration from Stress Test: ${AVG_TICK_MS} ms"
      # Check against threshold using bc for floating point comparison
      if (( $(echo "$AVG_TICK_MS > $MAX_AVG_TICK_MS" | bc -l) )); then
          print_error "Performance regression detected! Average tick time (${AVG_TICK_MS}ms) exceeded threshold (${MAX_AVG_TICK_MS}ms)."
      else
          print_success "Performance metrics within threshold (${MAX_AVG_TICK_MS}ms)."
      fi
  fi
  echo "Raw stress test metrics saved to: $STRESS_LOG_FILE"
fi


# 8. Generate Coverage Report (HTML) - Based on Unit Tests (Conditional)
if [ "$SKIP_COVERAGE" = true ]; then
  print_step "Skipping Coverage Report Generation"
elif [ -f "$COVERAGE_FILE" ]; then
  print_step "Generating Coverage Report (HTML)"
  run_command "HTML coverage report generation" go tool cover -html="$COVERAGE_FILE" -o "$COVERAGE_HTML"
  if [ -f "$COVERAGE_HTML" ]; then
      print_success "Coverage report generated: $COVERAGE_HTML"
  fi
else
  print_step "Skipping Coverage Report Generation"
  print_warning "Coverage file ($COVERAGE_FILE) not found."
fi


# --- Final Status Check ---
print_step "Build and Test Summary"
if [ "$FAILURE_DETECTED" = true ]; then
  print_error "One or more steps failed."
  exit 1
else
  print_success "All steps completed successfully."
  exit 0
fi