#!/usr/bin/env python3
"""
Parses Blackflow benchmark output and regenerates the README Performance
section.

Reads:
  docs/bench/results_baseline.txt
  docs/bench/results_round_robin.txt
  docs/bench/results_least_connection.txt
  docs/bench/results_ip_hash.txt
  docs/bench/balancer_bench.txt        (go test -bench output)

Writes the Performance section into README.md between these markers
(add them once to README.md, wrapping the existing section):

  <!-- PERF:START -->
  ...generated content...
  <!-- PERF:END -->

Usage:
  python3 docs/bench/scripts/gen_report.py            # parse + update README.md
  python3 docs/bench/scripts/gen_report.py --dry-run  # just print the table
  python3 docs/bench/scripts/gen_report.py --strict   # fail if any file has != 5 valid runs
"""

import argparse
import re
import statistics as st
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[3]
BENCH_DIR = REPO_ROOT / "docs" / "bench"
README = REPO_ROOT / "README.md"

RUN_BLOCK_RE = re.compile(r"Running \d+s test @.*?(?=Running \d+s test @|\Z)", re.S)
RPS_RE = re.compile(r"Requests/sec:\s+([\d.]+)")
P99_RE = re.compile(r"99\.000%\s+([\d.]+)ms")

CONFIGS = [
    ("baseline", "results_baseline.txt", "Baseline (direct, no proxy)"),
    ("round_robin", "results_round_robin.txt", "Round Robin"),
    ("least_connection", "results_least_connection.txt", "Least Connection"),
    ("ip_hash", "results_ip_hash.txt", "IP Hash"),
]


def parse_wrk2_file(path: Path, strict: bool):
    """Extract (rps, p99) pairs, one per valid run block. Skips incomplete
    blocks (e.g. a Ctrl-C'd run) rather than silently mixing bad data in."""
    text = path.read_text()
    rps_vals, p99_vals = [], []
    skipped = 0
    for block in RUN_BLOCK_RE.findall(text):
        rps_m = RPS_RE.search(block)
        p99_m = P99_RE.search(block)
        if rps_m and p99_m:
            rps_vals.append(float(rps_m.group(1)))
            p99_vals.append(float(p99_m.group(1)))
        else:
            skipped += 1

    if skipped:
        msg = f"WARNING: {path.name}: skipped {skipped} incomplete run block(s)"
        print(msg, file=sys.stderr)

    if len(rps_vals) != 5:
        msg = f"WARNING: {path.name}: found {len(rps_vals)} valid runs, expected 5"
        print(msg, file=sys.stderr)
        if strict:
            sys.exit(f"--strict: aborting due to {path.name}")

    return rps_vals, p99_vals


def stats_line(vals):
    if len(vals) < 2:
        return (vals[0], 0.0) if vals else (float("nan"), float("nan"))
    return st.mean(vals), st.stdev(vals)


def build_e2e_table(strict: bool):
    rows = []
    parsed = {}
    for key, fname, label in CONFIGS:
        path = BENCH_DIR / fname
        if not path.exists():
            sys.exit(f"ERROR: missing {path}")
        rps_vals, p99_vals = parse_wrk2_file(path, strict)
        rps_mean, rps_sd = stats_line(rps_vals)
        p99_mean, p99_sd = stats_line(p99_vals)
        parsed[key] = (rps_mean, rps_sd, p99_mean, p99_sd)

    base_p99 = parsed["baseline"][2]

    lines = [
        "| Config | Requests/sec (mean ± stdev) | p99 latency (mean ± stdev) | Overhead vs. baseline |",
        "|---|---|---|---|",
    ]
    for key, fname, label in CONFIGS:
        rps_mean, rps_sd, p99_mean, p99_sd = parsed[key]
        overhead = "—" if key == "baseline" else f"+{p99_mean - base_p99:.3f}ms"
        lines.append(
            f"| {label} | {rps_mean:.2f} ± {rps_sd:.3f} | "
            f"{p99_mean:.3f}ms ± {p99_sd:.3f}ms | {overhead} |"
        )
    return "\n".join(lines)


BENCH_LINE_RE = re.compile(
    r"^Benchmark(?P<algo>RoundRobin|LeastConnection|IPHash)_(?P<variant>\d+Backends|Parallel)-\d+\s+"
    r"\d+\s+(?P<ns>[\d.]+)\s+ns/op(?:\s+\d+\s+B/op\s+(?P<allocs>\d+)\s+allocs/op)?",
    re.M,
)

ALGO_LABELS = {
    "RoundRobin": "Round Robin",
    "LeastConnection": "Least Connection",
    "IPHash": "IP Hash",
}
BACKEND_COUNTS = ["2Backends", "10Backends", "100Backends"]


def build_micro_table():
    path = BENCH_DIR / "balancer_bench.txt"
    if not path.exists():
        print(
            f"WARNING: {path} not found, skipping micro-benchmark table",
            file=sys.stderr,
        )
        return None

    text = path.read_text()
    data = {}  # (algo, variant) -> (ns, allocs)
    for m in BENCH_LINE_RE.finditer(text):
        data[(m.group("algo"), m.group("variant"))] = (m.group("ns"), m.group("allocs"))

    lines = [
        "| Algorithm | 2 backends | 10 backends | 100 backends |",
        "|---|---|---|---|",
    ]
    for algo, label in ALGO_LABELS.items():
        cells = []
        for variant in BACKEND_COUNTS:
            entry = data.get((algo, variant))
            if entry:
                ns, allocs = entry
                cells.append(
                    f"{float(ns):.1f} ns/op, {allocs} allocs"
                    if allocs
                    else f"{float(ns):.1f} ns/op"
                )
            else:
                cells.append("—")
        lines.append(f"| {label} | {cells[0]} | {cells[1]} | {cells[2]} |")
    return "\n".join(lines)


SECTION_TEMPLATE = """## Performance

Benchmarked on a 12-core machine with client, backends, and Blackflow pinned to separate cores (`taskset`), using [wrk2](https://github.com/giltene/wrk2) for coordinated-omission-corrected load generation. Each configuration ran 5×20s measured runs at a fixed rate (`-t2 -c50 -R 5000 --latency`) after a discarded 10s warm-up, against two backend instances simulating 5ms of upstream latency. Full methodology and raw output in [docs/bench](docs/bench). Regenerate this section with `docs/bench/scripts/run_bench.sh` + `docs/bench/scripts/gen_report.py`.

### End-to-end throughput and latency

{e2e_table}

Throughput is unaffected by proxying — the run rate is capped by `-R 5000`, so this measures overhead at a fixed load rather than a max-throughput ceiling. Proxying adds a small, consistent amount of p99 latency regardless of balancing algorithm; at this network and backend-latency scale, algorithm choice has no measurable effect on end-to-end performance.

### Isolated algorithm-selection cost

Measured with `go test -bench=. -benchmem -run=^$ ./internal/balancer/...` — pure CPU cost of backend selection, no network or HTTP involved.

{micro_table}
"""


def render_section(strict: bool) -> str:
    e2e = build_e2e_table(strict)
    micro = (
        build_micro_table()
        or "_Micro-benchmark data not found — run `go test -bench=. -benchmem -run=^$ ./internal/balancer/...` and save output to `docs/bench/balancer_bench.txt`._"
    )
    return SECTION_TEMPLATE.format(e2e_table=e2e, micro_table=micro)


def update_readme(section: str):
    if not README.exists():
        sys.exit(f"ERROR: {README} not found")
    text = README.read_text()
    start, end = "<!-- PERF:START -->", "<!-- PERF:END -->"
    if start not in text or end not in text:
        sys.exit(
            f"ERROR: README.md is missing {start} / {end} markers.\n"
            f"Wrap the Performance section once with these markers, then rerun this script."
        )
    pattern = re.compile(re.escape(start) + r".*?" + re.escape(end), re.S)
    new_block = f"{start}\n{section}\n{end}"
    updated = pattern.sub(new_block, text)
    README.write_text(updated)
    print(f"Updated {README}")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument(
        "--dry-run",
        action="store_true",
        help="print the section instead of writing README.md",
    )
    ap.add_argument(
        "--strict",
        action="store_true",
        help="fail if any results file doesn't have exactly 5 valid runs",
    )
    args = ap.parse_args()

    section = render_section(args.strict)

    if args.dry_run:
        print(section)
    else:
        update_readme(section)


if __name__ == "__main__":
    main()
