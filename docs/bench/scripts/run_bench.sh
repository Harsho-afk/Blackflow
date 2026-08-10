#!/usr/bin/env bash
#
# Drives the full Blackflow benchmark: starts backend(s), optionally runs
# a direct-to-backend baseline, cycles the balancer algorithm in
# bench.toml, restarts Blackflow, and runs wrk2 (warm-up + 5 measured
# runs) for each proxied algorithm.
#
# Must be run from the repo root, on the target benchmark machine
# (this pins cores with taskset — don't run it in a container/CI runner
# where those cores don't map to anything meaningful).
#
# Usage:
#   docs/bench/scripts/run_bench.sh                        # round_robin, least_connection, ip_hash
#   docs/bench/scripts/run_bench.sh ip_hash                 # just one algorithm
#   docs/bench/scripts/run_bench.sh baseline                # just the baseline
#   docs/bench/scripts/run_bench.sh baseline round_robin least_connection ip_hash  # everything
#   RUNS=3 docs/bench/scripts/run_bench.sh                  # override run count
#
# "baseline" is NOT included in the default target list — it measures the
# backend directly (no Blackflow, no algorithm) and doesn't change when the
# proxy or algorithms change, so it's opt-in rather than rerun every time.

set -euo pipefail

# ---- config -----------------------------------------------------------
CONFIG="docs/bench/bench.toml"
PROXY_HOST="http://localhost:8080/api"
BASELINE_HOST="http://localhost:8081/"
RESULTS_DIR="docs/bench"

CLIENT_CORES="0,1"
PROXY_CORES="4,5"
BACKEND1_CORE="2"
BACKEND2_CORE="3"

THREADS=2
CONNS=50
RATE=5000
WARMUP_DUR="10s"
RUN_DUR="20s"
RUNS="${RUNS:-5}"

PROXIED_ALGOS=("round_robin" "least_connection" "ip_hash")
TARGETS=("${@:-round_robin least_connection ip_hash}")
if [[ ${#TARGETS[@]} -eq 1 && "${TARGETS[0]}" == *" "* ]]; then
  # handle the default-string case above
  read -r -a TARGETS <<< "${TARGETS[0]}"
fi

needs_blackflow() {
  for t in "${TARGETS[@]}"; do
    [[ "$t" != "baseline" ]] && return 0
  done
  return 1
}

BACKEND1_PID=""
BACKEND2_PID=""
BLACKFLOW_PID=""

# ---- helpers ------------------------------------------------------------
log() { echo "[$(date +%H:%M:%S)] $*"; }

cleanup() {
  log "Cleaning up background processes..."
  [[ -n "$BLACKFLOW_PID" ]] && kill "$BLACKFLOW_PID" 2>/dev/null || true
  [[ -n "$BACKEND1_PID" ]] && kill "$BACKEND1_PID" 2>/dev/null || true
  [[ -n "$BACKEND2_PID" ]] && kill "$BACKEND2_PID" 2>/dev/null || true
  wait 2>/dev/null || true
}
trap cleanup EXIT INT TERM

require_bin() {
  command -v "$1" >/dev/null 2>&1 || { echo "ERROR: '$1' not found on PATH" >&2; exit 1; }
}

wait_for_health() {
  local url=$1 tries=30
  until curl -sf "$url" >/dev/null 2>&1; do
    tries=$((tries - 1))
    [[ $tries -le 0 ]] && { echo "ERROR: $url never became healthy" >&2; exit 1; }
    sleep 0.2
  done
}

start_backends() {
  log "Starting backend instance(s)..."
  taskset -c "$BACKEND1_CORE" ./bin/bench-backend -addr :8081 -latency 5ms &
  BACKEND1_PID=$!
  wait_for_health "http://localhost:8081/health"

  if needs_blackflow; then
    taskset -c "$BACKEND2_CORE" ./bin/bench-backend -addr :8082 -latency 5ms &
    BACKEND2_PID=$!
    wait_for_health "http://localhost:8082/health"
  fi
}

start_blackflow() {
  LOG_LEVEL=warn taskset -c "$PROXY_CORES" ./bin/blackflow "$CONFIG" &
  BLACKFLOW_PID=$!
  wait_for_health "http://localhost:8080/api/health" || true
  sleep 1  # give the health-check loop a moment even if the above probe is lenient
}

stop_blackflow() {
  [[ -n "$BLACKFLOW_PID" ]] && kill "$BLACKFLOW_PID" 2>/dev/null || true
  wait "$BLACKFLOW_PID" 2>/dev/null || true
  BLACKFLOW_PID=""
}

set_algorithm() {
  local algo=$1
  sed -i.bak -E "s/algorithm = \"[a-zA-Z_]+\"/algorithm = \"${algo}\"/" "$CONFIG"
  rm -f "${CONFIG}.bak"
  log "Set algorithm = \"${algo}\" in $CONFIG"
}

run_one_algorithm() {
  local algo=$1
  local outfile="/tmp/results_${algo}.txt"

  stop_blackflow
  set_algorithm "$algo"
  start_blackflow

  log "[$algo] warm-up (${WARMUP_DUR}, discarded)"
  taskset -c "$CLIENT_CORES" wrk2 -t"$THREADS" -c"$CONNS" -d"$WARMUP_DUR" -R "$RATE" --latency "$PROXY_HOST" \
    > "/tmp/warmup_${algo}.txt"

  rm -f "$outfile"
  log "[$algo] ${RUNS} measured runs (${RUN_DUR} each)"
  for i in $(seq 1 "$RUNS"); do
    taskset -c "$CLIENT_CORES" wrk2 -t"$THREADS" -c"$CONNS" -d"$RUN_DUR" -R "$RATE" --latency "$PROXY_HOST" \
      | tee -a "$outfile"
    echo "--- run $i done ---" >> "$outfile"
  done

  cp "$outfile" "$RESULTS_DIR/results_${algo}.txt"
  log "[$algo] wrote $RESULTS_DIR/results_${algo}.txt"
}

run_baseline() {
  # Direct-to-backend: no Blackflow, no algorithm, single backend (:8081).
  local outfile="/tmp/results_baseline.txt"

  log "[baseline] warm-up (${WARMUP_DUR}, discarded)"
  taskset -c "$CLIENT_CORES" wrk2 -t"$THREADS" -c"$CONNS" -d"$WARMUP_DUR" -R "$RATE" --latency "$BASELINE_HOST" \
    > "/tmp/warmup_baseline.txt"

  rm -f "$outfile"
  log "[baseline] ${RUNS} measured runs (${RUN_DUR} each)"
  for i in $(seq 1 "$RUNS"); do
    taskset -c "$CLIENT_CORES" wrk2 -t"$THREADS" -c"$CONNS" -d"$RUN_DUR" -R "$RATE" --latency "$BASELINE_HOST" \
      | tee -a "$outfile"
    echo "--- run $i done ---" >> "$outfile"
  done

  cp "$outfile" "$RESULTS_DIR/results_baseline.txt"
  log "[baseline] wrote $RESULTS_DIR/results_baseline.txt"
}

# ---- main ---------------------------------------------------------------
require_bin taskset
require_bin wrk2
require_bin curl
[[ -x ./bin/bench-backend ]] || { echo "ERROR: ./bin/bench-backend not built" >&2; exit 1; }
if needs_blackflow; then
  [[ -x ./bin/blackflow ]] || { echo "ERROR: ./bin/blackflow not built (run 'make build')" >&2; exit 1; }
fi

for t in "${TARGETS[@]}"; do
  if [[ "$t" != "baseline" && ! " ${PROXIED_ALGOS[*]} " =~ " $t " ]]; then
    echo "ERROR: unknown target '$t' (expected: baseline, round_robin, least_connection, ip_hash)" >&2
    exit 1
  fi
done

start_backends

for t in "${TARGETS[@]}"; do
  log "############ TARGET: $t ############"
  if [[ "$t" == "baseline" ]]; then
    run_baseline
  else
    run_one_algorithm "$t"
  fi
done

stop_blackflow

if needs_blackflow; then
  log "Running balancer micro-benchmarks..."
  go test -bench=. -benchmem -run=^$ ./internal/balancer/... | tee "$RESULTS_DIR/balancer_bench.txt"
else
  log "Skipping balancer micro-benchmarks (baseline-only run)"
fi

log "Done. Results in $RESULTS_DIR/:"
ls -1 "$RESULTS_DIR"/results_*.txt "$RESULTS_DIR"/balancer_bench.txt 2>/dev/null || true

log "Next: docs/bench/scripts/gen_report.py to compute stats and update the README."
