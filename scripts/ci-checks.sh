#!/usr/bin/env bash
# ci-checks.sh - Pre-release CI checks for OpenLoadBalancer
#
# Runs the full test suite, benchmarks, binary size check, startup time
# measurement, and coverage report. Prints a pass/fail summary table.
#
# Usage:
#   ./scripts/ci-checks.sh
#
# Environment variables:
#   OLB_SKIP_RACE    Set to 1 to skip race detector tests (e.g. no GCC on Windows)
#   OLB_SKIP_BENCH   Set to 1 to skip benchmarks
#   OLB_MAX_SIZE_MB  Max binary size in MB (default: 20)

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

BINARY_NAME="olb"
MAX_SIZE_MB="${OLB_MAX_SIZE_MB:-20}"
RESULTS_DIR="$PROJECT_DIR/build-results"

# Track pass/fail results
declare -a CHECK_NAMES=()
declare -a CHECK_RESULTS=()
declare -a CHECK_DETAILS=()
EXIT_CODE=0

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

log_step() {
    echo ""
    echo "========================================================================"
    echo "  $1"
    echo "========================================================================"
    echo ""
}

record_result() {
    local name="$1"
    local result="$2"
    local detail="${3:-}"
    CHECK_NAMES+=("$name")
    CHECK_RESULTS+=("$result")
    CHECK_DETAILS+=("$detail")
    if [ "$result" = "FAIL" ]; then
        EXIT_CODE=1
    fi
}

# ---------------------------------------------------------------------------
# Setup
# ---------------------------------------------------------------------------

cd "$PROJECT_DIR"
mkdir -p "$RESULTS_DIR"

echo "OpenLoadBalancer CI Checks"
echo "Project directory: $PROJECT_DIR"
echo "Results directory: $RESULTS_DIR"
echo "Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo ""

# ---------------------------------------------------------------------------
# Check 1: Full Test Suite
# ---------------------------------------------------------------------------

log_step "Running full test suite..."

if go test -count=1 -timeout=300s -coverprofile="$RESULTS_DIR/coverage.out" ./... 2>&1 | tee "$RESULTS_DIR/test-output.txt"; then
    record_result "Test Suite" "PASS"
else
    record_result "Test Suite" "FAIL" "See $RESULTS_DIR/test-output.txt"
fi

# ---------------------------------------------------------------------------
# Check 2: Race Detector Tests (optional)
# ---------------------------------------------------------------------------

if [ "${OLB_SKIP_RACE:-0}" = "1" ]; then
    log_step "Skipping race detector tests (OLB_SKIP_RACE=1)"
    record_result "Race Detector" "SKIP" "OLB_SKIP_RACE=1"
else
    log_step "Running race detector tests..."

    if go test -race -count=1 -timeout=600s ./... 2>&1 | tee "$RESULTS_DIR/race-output.txt"; then
        record_result "Race Detector" "PASS"
    else
        record_result "Race Detector" "FAIL" "See $RESULTS_DIR/race-output.txt"
    fi
fi

# ---------------------------------------------------------------------------
# Check 3: Benchmarks
# ---------------------------------------------------------------------------

if [ "${OLB_SKIP_BENCH:-0}" = "1" ]; then
    log_step "Skipping benchmarks (OLB_SKIP_BENCH=1)"
    record_result "Benchmarks" "SKIP" "OLB_SKIP_BENCH=1"
else
    log_step "Running benchmarks..."

    if go test -bench=. -benchmem -count=1 -run='^$' -timeout=600s ./... 2>&1 | tee "$RESULTS_DIR/benchmark.txt"; then
        record_result "Benchmarks" "PASS" "Results in $RESULTS_DIR/benchmark.txt"
    else
        record_result "Benchmarks" "FAIL" "See $RESULTS_DIR/benchmark.txt"
    fi
fi

# ---------------------------------------------------------------------------
# Check 4: Build Binary
# ---------------------------------------------------------------------------

log_step "Building binary..."

if CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$RESULTS_DIR/$BINARY_NAME" ./cmd/olb 2>&1; then
    record_result "Build" "PASS"
else
    record_result "Build" "FAIL" "go build failed"
fi

# ---------------------------------------------------------------------------
# Check 5: Binary Size
# ---------------------------------------------------------------------------

log_step "Checking binary size..."

if [ -f "$RESULTS_DIR/$BINARY_NAME" ]; then
    # Cross-platform size detection
    if command -v stat > /dev/null 2>&1; then
        # Try GNU stat first (Linux)
        SIZE=$(stat --format=%s "$RESULTS_DIR/$BINARY_NAME" 2>/dev/null || stat -f%z "$RESULTS_DIR/$BINARY_NAME" 2>/dev/null || wc -c < "$RESULTS_DIR/$BINARY_NAME")
    else
        SIZE=$(wc -c < "$RESULTS_DIR/$BINARY_NAME")
    fi
    SIZE_MB=$((SIZE / 1024 / 1024))
    MAX_SIZE=$((MAX_SIZE_MB * 1024 * 1024))

    echo "Binary size: $SIZE bytes (${SIZE_MB} MB)"
    echo "Max allowed: ${MAX_SIZE_MB} MB"

    if [ "$SIZE" -gt "$MAX_SIZE" ]; then
        record_result "Binary Size" "FAIL" "${SIZE_MB}MB exceeds ${MAX_SIZE_MB}MB limit"
    else
        record_result "Binary Size" "PASS" "${SIZE_MB}MB (limit: ${MAX_SIZE_MB}MB)"
    fi
else
    record_result "Binary Size" "FAIL" "Binary not found"
fi

# ---------------------------------------------------------------------------
# Check 6: Startup Time
# ---------------------------------------------------------------------------

log_step "Measuring startup time..."

if [ -f "$RESULTS_DIR/$BINARY_NAME" ]; then
    BINARY="$RESULTS_DIR/$BINARY_NAME"

    # Warm up filesystem cache
    "$BINARY" version > /dev/null 2>&1 || true

    TOTAL=0
    RUNS=5
    echo "Running $RUNS iterations..."

    for i in $(seq 1 $RUNS); do
        START_NS=$(date +%s%N 2>/dev/null || echo "0")
        "$BINARY" version > /dev/null 2>&1 || true
        END_NS=$(date +%s%N 2>/dev/null || echo "0")

        if [ "$START_NS" = "0" ] || [ "$END_NS" = "0" ]; then
            echo "  Note: nanosecond timing not supported on this platform"
            record_result "Startup Time" "SKIP" "Nanosecond timing not available"
            break
        fi

        ELAPSED_MS=$(( (END_NS - START_NS) / 1000000 ))
        echo "  Run $i: ${ELAPSED_MS}ms"
        TOTAL=$((TOTAL + ELAPSED_MS))
    done

    if [ "$TOTAL" -gt 0 ]; then
        AVG=$((TOTAL / RUNS))
        echo "Average startup time: ${AVG}ms"

        if [ "$AVG" -gt 5000 ]; then
            record_result "Startup Time" "FAIL" "Average ${AVG}ms exceeds 5000ms"
        else
            record_result "Startup Time" "PASS" "Average ${AVG}ms"
        fi
    fi
else
    record_result "Startup Time" "FAIL" "Binary not found"
fi

# ---------------------------------------------------------------------------
# Check 7: Test Coverage Report
# ---------------------------------------------------------------------------

log_step "Generating coverage report..."

if [ -f "$RESULTS_DIR/coverage.out" ]; then
    go tool cover -func="$RESULTS_DIR/coverage.out" > "$RESULTS_DIR/coverage-summary.txt" 2>&1

    # Extract total coverage percentage
    TOTAL_LINE=$(tail -1 "$RESULTS_DIR/coverage-summary.txt")
    COVERAGE=$(echo "$TOTAL_LINE" | awk '{print $NF}')
    echo "Total coverage: $COVERAGE"

    record_result "Coverage Report" "PASS" "Total: $COVERAGE"

    # Also generate HTML report
    go tool cover -html="$RESULTS_DIR/coverage.out" -o "$RESULTS_DIR/coverage.html" 2>/dev/null || true
else
    record_result "Coverage Report" "SKIP" "No coverage.out found"
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------

log_step "SUMMARY"

# Print table header
printf "%-25s %-8s %s\n" "Check" "Result" "Details"
printf "%-25s %-8s %s\n" "-------------------------" "--------" "----------------------------"

for i in "${!CHECK_NAMES[@]}"; do
    NAME="${CHECK_NAMES[$i]}"
    RESULT="${CHECK_RESULTS[$i]}"
    DETAIL="${CHECK_DETAILS[$i]}"

    # Color output for terminals
    if [ -t 1 ]; then
        case "$RESULT" in
            PASS) COLOR="\033[32m" ;;  # green
            FAIL) COLOR="\033[31m" ;;  # red
            SKIP) COLOR="\033[33m" ;;  # yellow
            *)    COLOR="" ;;
        esac
        RESET="\033[0m"
        printf "%-25s ${COLOR}%-8s${RESET} %s\n" "$NAME" "$RESULT" "$DETAIL"
    else
        printf "%-25s %-8s %s\n" "$NAME" "$RESULT" "$DETAIL"
    fi
done

echo ""
echo "Results saved to: $RESULTS_DIR/"
echo ""

if [ "$EXIT_CODE" -ne 0 ]; then
    echo "OVERALL: FAIL"
else
    echo "OVERALL: PASS"
fi

exit "$EXIT_CODE"
