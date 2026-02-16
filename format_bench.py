#!/usr/bin/env python3
"""Parse CRI benchmark output and generate a markdown table comparing list vs stream performance.

Usage:
    # Run benchmarks and generate table:
    python3 format_bench.py

    # Or pipe existing benchmark output:
    go test -bench=BenchmarkCRIListVsStream -benchtime=1x -run=^$ | python3 format_bench.py
"""

import re
import subprocess
import sys


def parse_benchmark_output(text):
    results = {}
    pattern = re.compile(
        r"containers=(\d+)/annotations=(\d+).*?"
        r"([\d.]+)\s+list-ms/req.*?"
        r"([\d.]+)\s+stream-ms/req"
    )
    for line in text.splitlines():
        m = pattern.search(line)
        if m:
            containers = int(m.group(1))
            annotations = int(m.group(2))
            list_ms = float(m.group(3))
            stream_ms = float(m.group(4))
            results[(containers, annotations)] = (list_ms, stream_ms)
    return results


def format_val(val, is_faster):
    s = f"{val:.2f}"
    if is_faster:
        return f"**{s}**"
    return s


def generate_table(results):
    containers_set = sorted(set(c for c, _ in results))
    annotations_set = sorted(set(a for _, a in results))

    # Header
    header = "| Annotations \\ Containers |"
    for c in containers_set:
        header += f" {c} |"
    print(header)

    # Separator
    sep = "| --- |"
    for _ in containers_set:
        sep += " --- |"
    print(sep)

    # Rows
    for a in annotations_set:
        row = f"| {a} |"
        for c in containers_set:
            if (c, a) in results:
                list_ms, stream_ms = results[(c, a)]
                list_faster = list_ms < stream_ms
                stream_faster = stream_ms < list_ms
                cell = (
                    f"{format_val(list_ms, list_faster)} / "
                    f"{format_val(stream_ms, stream_faster)}"
                )
                row += f" {cell} |"
            else:
                row += " - |"
        print(row)

    print()
    print("Values: list ms/req / stream ms/req (lower is better, **bold** = faster)")


def main():
    if not sys.stdin.isatty():
        text = sys.stdin.read()
    else:
        print("Running benchmarks...", file=sys.stderr)
        result = subprocess.run(
            [
                "go", "test",
                "-bench=BenchmarkCRIListVsStream",
                "-benchtime=1x",
                "-run=^$",
                "-timeout=30m",
            ],
            capture_output=True,
            text=True,
        )
        text = result.stdout + "\n" + result.stderr
        if result.returncode != 0:
            print("Benchmark failed:", file=sys.stderr)
            print(result.stderr, file=sys.stderr)
            sys.exit(1)
        # Print benchmark progress to stderr
        if result.stderr:
            print(result.stderr, file=sys.stderr, end="")

    results = parse_benchmark_output(text)
    if not results:
        print("No benchmark results found in output.", file=sys.stderr)
        print("Expected lines matching: BenchmarkCRIListVsStream/containers=N/annotations=N", file=sys.stderr)
        sys.exit(1)

    generate_table(results)


if __name__ == "__main__":
    main()
