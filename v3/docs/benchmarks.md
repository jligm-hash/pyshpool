# Benchmarks

These are **local microbenchmarks**, not universal performance claims. Process
startup, filesystem speed, and the shape of each command dominate the result.

## Method

[`scripts/bench.py`](https://github.com/jligm-hash/pyshpool/blob/main/v3/scripts/bench.py) generates `N` lines containing `true`
and measures wall time for:

- `pysh run -i tasks.dat -c J --quiet`
- `xargs -P J -I{} sh -c '{}'`
- `parallel -j J` (GNU parallel, when installed)

All tools read the same file. `pysh` additionally writes one log per task and a
`summary.json`; that overhead is part of the measurement. Each command is run
once after the binary is built; no warm-up or repeated median is performed.

## Example result

Machine: macOS, Apple-silicon era laptop, Go 1.27.3, 500 trivial tasks, 8
workers.

| Tool | Wall time | Tasks/sec |
|---|---:|---:|
| `pysh` | 522 ms | 957 |
| `xargs -P` | 502 ms | 996 |
| GNU parallel | 5635 ms | 89 |

`pysh` is within a few percent of `xargs` for this workload while also
producing per-task logs and a structured summary. GNU parallel's result here is
dominated by its startup and per-job machinery on this machine; it remains the
better tool for large batches, SSH fan-out, and job-log resume.

## Reproduce

```bash
cd v3
make build
python3 scripts/bench.py --tasks 500 --jobs 8
```

Try other shapes:

```bash
python3 scripts/bench.py --tasks 2000 --jobs 32
```

For I/O-bound or CPU-bound commands, replace `true` in the script with the real
command. The relative ordering may change substantially.

## What is not measured

- Slurm queue latency and array scheduling overhead.
- Network-bound API calls in `pysh-agent`.
- Peak memory or file-descriptor usage.
- Long-tail tasks where one slow job dominates wall time.

For Slurm, compare a native array against a split-plus-pool approach on your
own cluster; queue policy and element overhead are site-specific.
