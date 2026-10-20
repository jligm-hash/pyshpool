# Slurm arrays with `pysh submit`

`pysh submit` turns a task file into a native Slurm job array. It deliberately
does **not** split the file into per-node chunks and it does not generate a
second scheduler. One array element reads one line.

## Why arrays instead of splitting

The v2 multi-node scripts split the input into `inpDL1.out`, `inpDL2.out`, ...
and launched a local pool inside each allocation. That approach had several
problems:

- when nodes outnumbered tasks, the last chunk was duplicated (`tail -n -1`);
- fixed `tmp.git` and `logNode*.out` names collided between runs;
- an empty account produced `#SBATCH -A` with no value;
- a failed split or submission was easy to miss.

A job array delegates queueing, accounting, concurrency limits, and per-element
logs to Slurm. The script only needs to select the line and run it.

## Usage

```bash
pysh submit -i TASKS [flags]
```

By default the generated script is printed to stdout. Add `--submit` to write
it under `.pysh/submits/<timestamp>/submit.sbatch` and run `sbatch`.

```bash
pysh submit \
  -i tasks.dat \
  -J demo \
  -p general \
  -A my-account \
  -N 1 \
  -n 8 \
  --array-limit 50 \
  --time 04:00:00 \
  --mem 16G \
  --output 'logs/%A_%a.out' \
  --submit
```

Omit `-A` if your site does not use accounts; the directive is omitted rather
than emitted empty.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `-i`, `--input` | — | Task file. Required unless positional. |
| `-J`, `--job-name` | `pysh` | Slurm job name. |
| `-p`, `--partition` | — | Partition. |
| `-A`, `--account` | — | Account; omitted when empty. |
| `-N`, `--nodes` | `1` | Nodes per array element. |
| `-n`, `--cpus-per-task` | `1` | CPUs per array element. |
| `--array` | `1-<lines>` | Custom array expression. |
| `--array-limit` | `0` | Append `%K` to throttle concurrent elements. |
| `--time` | — | Time limit, e.g. `04:00:00`. |
| `--mem` | — | Memory, e.g. `16G`. |
| `--gres` | — | Generic resources, e.g. `gpu:1`. |
| `-w`, `--nodelist` | — | Node list. |
| `--exclude` | — | Nodes to exclude. |
| `--mail-type` | — | Slurm mail type. |
| `--mail-user` | — | Mail address. |
| `--workdir` | current dir | `#SBATCH --chdir`. |
| `--output` | `pysh-%A_%a.out` | Output pattern. |
| `--error` | same as output | Error pattern. |
| `--script-out` | — | Write the script to a path. |
| `--submit` | `false` | Run `sbatch` on the generated script. |
| `--dry-run` | `false` | Print the script. |
| `--quiet` | `false` | Suppress status messages. |
| `--sbatch-opt` | — | Extra `#SBATCH` option. Repeatable. |

## How the array maps to lines

`--array=1-<line count>` includes every line, including blank lines and
comments. Each array element runs:

```bash
cmd="$(sed -n "${SLURM_ARRAY_TASK_ID}p" "$TASK_FILE")"
bash -c "$cmd"
```

A blank line exits 0 with a short message. A comment line would be passed to
`bash` and is treated as a no-op. If you want array indices to match only real
tasks, keep the file free of blank lines and comments, or use a custom
`--array` with a wrapper.

The script captures the command's exit status and exits with it, so Slurm
records failures correctly:

```bash
set +e
bash -c "$cmd"
status=$?
set -e
exit "$status"
```

## CPU and thread settings

`-n` is `--cpus-per-task` for each array element. If a command is
single-threaded, `-n 1` is appropriate. If it uses OpenMP, BLAS, or another
thread pool, set `-n` to the thread count and export the matching environment
variables in the command:

```bash
export OMP_NUM_THREADS=8
python train.py
```

`pysh submit` does not set `OMP_NUM_THREADS` automatically because the correct
value depends on the program and the allocation.

## Many small tasks

Slurm arrays have per-element scheduling overhead. For thousands of very short
commands, consider:

- increasing `--array-limit` so fewer elements wait;
- grouping commands into batches inside one array element;
- using `pysh run` inside a single allocation with `-c` set to the allocated
  CPU count.

## Logs and accounting

Use `--output` and `--error` with `%A` (job ID) and `%a` (array index):

```bash
pysh submit -i tasks.dat --output 'logs/pysh-%A_%a.out' --error 'logs/pysh-%A_%a.err' --submit
```

Then inspect with `squeue`, `sacct`, and the files under `logs/`.

## Differences from v2

| v2 | v3 |
|---|---|
| Split input into per-node files | One array element per line |
| Duplicated last task on extra nodes | No splitter, no duplication |
| Empty `#SBATCH -A` when unset | Directive omitted |
| Fixed `tmp.git`, `logNode*.out` | No generated fixed names; `%A_%a` patterns |
| `sbatch` result often unchecked | `--submit` fails loudly if `sbatch` fails |
| Custom local pool per node | Slurm-native array; optional `pysh run` inside a job |
