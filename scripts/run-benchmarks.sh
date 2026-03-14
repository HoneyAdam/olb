#!/usr/bin/env bash
# run-benchmarks.sh - Run OpenLoadBalancer benchmark suite
#
# Usage:
#   ./scripts/run-benchmarks.sh              # run all benchmarks
#   ./scripts/run-benchmarks.sh -filter Proxy # run only benchmarks matching "Proxy"
#   ./scripts/run-benchmarks.sh -compare      # compare with previous results
#   ./scripts/run-benchmarks.sh -short        # quick run (count=1)

set -euo pipefail

# ---- Configuration --------------------------------------------------------

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BENCH_PKG="./test/benchmark/"
RESULTS_DIR="${PROJECT_ROOT}/bench-results"
CURRENT_FILE="${RESULTS_DIR}/current.txt"
PREVIOUS_FILE="${RESULTS_DIR}/previous.txt"

COUNT=3
BENCHTIME="1s"
FILTER="."
COMPARE=false
SHORT=false
CPU_PROFILE=""
MEM_PROFILE=""

# ---- Argument parsing ------------------------------------------------------

while [[ $# -gt 0 ]]; do
    case "$1" in
        -filter)
            FILTER="$2"
            shift 2
            ;;
        -compare)
            COMPARE=true
            shift
            ;;
        -short)
            SHORT=true
            COUNT=1
            BENCHTIME="100ms"
            shift
            ;;
        -count)
            COUNT="$2"
            shift 2
            ;;
        -benchtime)
            BENCHTIME="$2"
            shift 2
            ;;
        -cpuprofile)
            CPU_PROFILE="$2"
            shift 2
            ;;
        -memprofile)
            MEM_PROFILE="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  -filter PATTERN   Only run benchmarks matching PATTERN (default: .)"
            echo "  -compare          Compare results with previous run"
            echo "  -short            Quick run (count=1, benchtime=100ms)"
            echo "  -count N          Number of benchmark iterations (default: 3)"
            echo "  -benchtime T      Benchmark duration (default: 1s)"
            echo "  -cpuprofile FILE  Write CPU profile to FILE"
            echo "  -memprofile FILE  Write memory profile to FILE"
            echo "  -h, --help        Show this help"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# ---- Setup -----------------------------------------------------------------

mkdir -p "${RESULTS_DIR}"
cd "${PROJECT_ROOT}"

echo "============================================================"
echo "  OpenLoadBalancer Benchmark Suite"
echo "============================================================"
echo ""
echo "  Filter:     ${FILTER}"
echo "  Count:      ${COUNT}"
echo "  Bench time: ${BENCHTIME}"
echo "  Results:    ${RESULTS_DIR}"
echo ""

# ---- Build check -----------------------------------------------------------

echo ">> Verifying build..."
if ! go build ./... 2>/dev/null; then
    echo "ERROR: Build failed. Fix compilation errors first."
    exit 1
fi
echo "   Build OK"
echo ""

# ---- Run benchmarks --------------------------------------------------------

# Build benchmark flags
BENCH_FLAGS=(
    -bench="${FILTER}"
    -benchmem
    -count="${COUNT}"
    -benchtime="${BENCHTIME}"
    -run="^$"
    -timeout=600s
)

if [[ -n "${CPU_PROFILE}" ]]; then
    BENCH_FLAGS+=(-cpuprofile="${CPU_PROFILE}")
fi

if [[ -n "${MEM_PROFILE}" ]]; then
    BENCH_FLAGS+=(-memprofile="${MEM_PROFILE}")
fi

echo ">> Running benchmarks..."
echo ""

# Rotate previous results if current exists
if [[ -f "${CURRENT_FILE}" ]]; then
    cp "${CURRENT_FILE}" "${PREVIOUS_FILE}"
fi

# Run benchmarks and tee to both stdout and file
go test "${BENCH_FLAGS[@]}" "${BENCH_PKG}" 2>&1 | tee "${CURRENT_FILE}"

BENCH_EXIT=${PIPESTATUS[0]}
if [[ ${BENCH_EXIT} -ne 0 ]]; then
    echo ""
    echo "ERROR: Benchmarks failed with exit code ${BENCH_EXIT}"
    exit ${BENCH_EXIT}
fi

echo ""
echo ">> Results saved to: ${CURRENT_FILE}"

# ---- Comparison (benchstat) ------------------------------------------------

if [[ "${COMPARE}" == "true" ]]; then
    echo ""
    if [[ ! -f "${PREVIOUS_FILE}" ]]; then
        echo ">> No previous results found for comparison."
        echo "   Run benchmarks again to generate a comparison baseline."
    elif command -v benchstat >/dev/null 2>&1; then
        echo ">> Comparison with previous run (benchstat):"
        echo ""
        benchstat "${PREVIOUS_FILE}" "${CURRENT_FILE}"
    else
        echo ">> benchstat not found. Install it for comparison:"
        echo "   go install golang.org/x/perf/cmd/benchstat@latest"
        echo ""
        echo ">> Manual comparison (previous vs current):"
        echo ""
        echo "--- Previous ---"
        grep "^Benchmark" "${PREVIOUS_FILE}" | head -30
        echo ""
        echo "--- Current ---"
        grep "^Benchmark" "${CURRENT_FILE}" | head -30
    fi
fi

# ---- Summary ---------------------------------------------------------------

echo ""
echo "============================================================"
echo "  Summary"
echo "============================================================"
echo ""

# Extract and display a summary table
if grep -q "^Benchmark" "${CURRENT_FILE}"; then
    printf "%-55s %15s %15s %10s\n" "BENCHMARK" "ns/op" "B/op" "allocs/op"
    printf "%-55s %15s %15s %10s\n" "$(printf '%0.s-' {1..55})" "$(printf '%0.s-' {1..15})" "$(printf '%0.s-' {1..15})" "$(printf '%0.s-' {1..10})"

    grep "^Benchmark" "${CURRENT_FILE}" | while IFS= read -r line; do
        # Parse benchmark output: BenchmarkName-N  iterations  ns/op  B/op  allocs/op
        name=$(echo "$line" | awk '{print $1}')
        nsop=$(echo "$line" | grep -oP '\d+\.?\d*\s+ns/op' | awk '{print $1}' || echo "N/A")
        bop=$(echo "$line" | grep -oP '\d+\s+B/op' | awk '{print $1}' || echo "N/A")
        aop=$(echo "$line" | grep -oP '\d+\s+allocs/op' | awk '{print $1}' || echo "N/A")

        # Truncate long names
        if [[ ${#name} -gt 55 ]]; then
            name="${name:0:52}..."
        fi

        printf "%-55s %15s %15s %10s\n" "$name" "$nsop" "$bop" "$aop"
    done
else
    echo "  No benchmark results found."
fi

echo ""
echo ">> Done."
