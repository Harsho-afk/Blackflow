# Blackflow benchmarks

This directory holds everything needed to reproduce the numbers in the
main [README's Performance section](../../README.md#performance): the
benchmark backend, the Blackflow config used for benchmarking, the raw
result files, and the scripts that generate and parse them.

---

## What gets measured

Two independent things, reported in two separate tables — don't mix them:

1. **End-to-end (wrk2)** — real HTTP requests through the full stack
   (client → Blackflow → backend), measuring `Requests/sec` and p99
   latency under a fixed load. This is what "proxy overhead" means: how
   much slower is going through Blackflow than hitting the backend
   directly.
2. **Isolated algorithm-selection cost (`go test -bench`)** — the pure
   CPU cost of the balancer picking a backend, with no network or HTTP
   involved. This is where the three algorithms (round_robin,
   least_connection, ip_hash) actually differ; at the network/latency
   scale of (1), that difference is invisible.

Four configurations are benchmarked end-to-end:

| Config | What it hits | Needs Blackflow? |
|---|---|---|
| `baseline` | `bench-backend` directly, port `8081` | No |
| `round_robin` | Blackflow → 2 backends, round robin | Yes |
| `least_connection` | Blackflow → 2 backends, least connections | Yes |
| `ip_hash` | Blackflow → 2 backends, IP hash | Yes |

---

## Directory layout

```
docs/bench/
├── README.md                    ← this file
├── bench.toml                   ← config used for benchmarking (edited by run_bench.sh)
├── backend/main.go              ← bench-backend source (real concurrent HTTP server)
├── results_baseline.txt         ← wrk2 output, baseline
├── results_round_robin.txt      ← wrk2 output, round_robin
├── results_least_connection.txt ← wrk2 output, least_connection
├── results_ip_hash.txt          ← wrk2 output, ip_hash
├── balancer_bench.txt           ← go test -bench output
└── scripts/
    ├── run_bench.sh             ← runs the actual benchmarks
    └── gen_report.py            ← parses results, updates README.md
```

`bench.toml` is separate from
[`docs/example-configs/example.toml`](../example-configs/example.toml)
because the example config uses docker-compose hostnames
(`auth1:8081`, etc.) that don't resolve on a local machine.

---

## Prerequisites

- **Go 1.22+**, `bin/blackflow` and `bin/bench-backend` built (`make build`)
- **[wrk2](https://github.com/giltene/wrk2)** on `PATH` as `wrk2` — not
  `wrk` or `hey`. wrk2 is coordinated-omission corrected, which regular
  wrk is not; that matters for accurate tail-latency numbers under a
  fixed request rate. Build from source and install as `wrk2` so it
  doesn't collide with a stock `wrk` you might also have:
  ```bash
  git clone https://github.com/giltene/wrk2 && cd wrk2
  LDFLAGS="-fuse-ld=mold" make   # stock ld can choke on the bundled LuaJIT bytecode
  sudo cp wrk /usr/local/bin/wrk2
  ```
- **`taskset`** (Linux `util-linux`) for CPU pinning. This benchmark
  suite assumes Linux; there's no macOS/Windows equivalent wired in.
- **A 12-core-or-better machine.** Client, backends, and Blackflow are
  pinned to distinct core ranges so they don't contend with each other
  or the OS scheduler. Fewer cores will still run, but numbers won't be
  comparable to the ones already in the README.
- **`curl`**, used to poll `/health` before each run starts.

Do **not** run these scripts in a container/CI runner — `taskset`
pinning to specific core numbers is meaningless if you don't control
the underlying core layout, and results won't be trustworthy.

---

## Running it

### 1. Generate results — `run_bench.sh`

```bash
docs/bench/scripts/run_bench.sh                        # round_robin, least_connection, ip_hash
docs/bench/scripts/run_bench.sh ip_hash                 # just one algorithm
docs/bench/scripts/run_bench.sh baseline                # just the baseline
docs/bench/scripts/run_bench.sh baseline round_robin least_connection ip_hash  # everything
RUNS=3 docs/bench/scripts/run_bench.sh                  # override run count (default 5)
```

What it does, per target:

- **Backends**: starts `bench-backend` on `:8081` (and `:8082` too, if
  any proxied algorithm is requested), pinned to dedicated cores, and
  waits for `/health` to respond before moving on.
- **Proxied algorithms** (`round_robin` / `least_connection` /
  `ip_hash`): stops any running Blackflow, `sed`-edits `algorithm =
  "..."` in `bench.toml`, restarts Blackflow pinned to its own cores,
  waits for it to come up, then runs wrk2: a discarded 10s warm-up
  followed by 5×20s measured runs at `-t2 -c50 -R 5000 --latency`
  against `http://localhost:8080/api`. Output goes to
  `docs/bench/results_<algorithm>.txt`.
- **`baseline`**: same wrk2 warm-up/run pattern, but hits
  `http://localhost:8081/` directly — no Blackflow, no algorithm, only
  backend `:8081`. Output goes to `docs/bench/results_baseline.txt`.
  Not included in the default target list, since it doesn't change
  when the proxy or algorithms do — pass it explicitly when you want it
  regenerated (e.g. after changing `bench-backend` or the benchmark
  machine).
- **Micro-benchmark**: after any proxied-algorithm target runs, also
  runs `go test -bench=. -benchmem -run=^$ ./internal/balancer/...` and
  saves it to `docs/bench/balancer_bench.txt`. Skipped on a
  baseline-only invocation.

Each result file is freshly `rm -f`'d before its 5 runs start, so
re-running a target can't silently concatenate an old attempt's data
onto a new one (see "Gotchas" below for why that matters).

Core assignment, all `taskset`-pinned:

| Role | Cores |
|---|---|
| wrk2 client | 0, 1 |
| Backend `:8081` | 2 |
| Backend `:8082` | 3 |
| Blackflow | 4, 5 |

### 2. Compute stats and update the README — `gen_report.py`

```bash
python3 docs/bench/scripts/gen_report.py --dry-run   # preview, don't write anything
python3 docs/bench/scripts/gen_report.py               # write the README.md Performance section
python3 docs/bench/scripts/gen_report.py --strict     # abort instead of warn if any file has != 5 valid runs
```

This script does no benchmarking itself — it only reads the four
`results_*.txt` files and `balancer_bench.txt` from this directory,
computes mean ± stdev of `Requests/sec` and the `99.000%` latency line
per config, computes each proxied config's p99 overhead vs. baseline,
and parses the Go benchmark output into a second table.

It writes into the main [`README.md`](../../README.md) between two
marker comments:

```markdown
<!-- PERF:START -->
...generated content...
<!-- PERF:END -->
```

Those markers must already exist in `README.md` — the script errors out rather than guessing where to insert if it can't find them.

---

## Reading wrk2 output / how parsing works

Each 20s run produces a block starting with `Running 20s test @ ...`.
`gen_report.py` splits each results file on those headers and, per
block, pulls out:

- `Requests/sec:` — throughput for that run
- ` 99.000%   X.XXms` — the p99 latency line from wrk2's percentile summary

A block missing either line (e.g. a run that got Ctrl-C'd mid-flight,
which still leaves a `--- run N done ---` marker but no data) is
skipped and logged as a warning, not silently averaged in. If a file
ends up with anything other than exactly 5 valid blocks, `gen_report.py`
prints a warning to stderr (or hard-fails with `--strict`).

The `--- run N done ---` lines are written by `run_bench.sh`'s loop for
human-readability when tailing a file live; the parser doesn't rely on
them, only on the `Running ... test @` headers.

---

## Gotchas

- **Don't append multiple attempts into one results file.** If a run
  gets interrupted, restart it — `run_bench.sh` already handles this by
  `rm -f`ing the file before each 5-run loop. If you're ever running
  wrk2 manually instead of through the script, do the same, or use a
  differently-named file per attempt. A results file that's secretly
  two concatenated attempts (e.g. 4 runs from one, 5 from another) will
  either get caught by the "exactly 5 blocks" check or, worse, silently
  average over the wrong set if it happens to total 5.
- **`context canceled` log lines in Blackflow's output at the boundary
  between wrk2 runs are expected**, not a real failure — that's wrk2
  closing its connection pool between loop iterations.
- **`-R 5000` measures overhead at a fixed rate, not a throughput
  ceiling.** Requests/sec being flat across baseline and all three
  algorithms is expected — wrk2 is capped at the target rate, so it
  isn't finding the max the system can sustain. If you want a
  throughput-ceiling number, that's a separate, uncapped (or very
  high `-R`) run — not currently part of `run_bench.sh`.
- **Numbers aren't portable across machines.** Everything here is
  pinned to specific core numbers and calibrated against a 12-core
  machine. Re-running on different hardware will produce internally
  consistent but not directly comparable numbers to what's already in
  the README.
