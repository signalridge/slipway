# Profiling Recipes

Use when a slow path has been reproduced and you are ready to attach an
observer. The recipe you pick depends on the *class* of slowness (CPU,
allocation, I/O, lock contention), not on the language's default tool.

## Pick the observer to match the symptom

| Symptom | Observer class | Wrong tool trap |
|---------|----------------|-----------------|
| High CPU, flat wall-clock scaling | Sampling CPU profiler | A tracing profiler will distort the hot path |
| GC pauses, memory growth, OOM | Allocation / heap profiler | CPU profiler will blame the allocator, not the caller |
| High latency at low CPU | Off-CPU / wall-clock profiler | Sampling CPU profiler will see nothing |
| Slow I/O, lock waits | Blocking / contention profiler | Wall-clock profiler will attribute time to whichever function blocked |
| Tail-latency spikes | Continuous profiler + tracing | Single-run snapshot will miss the outlier |

If you cannot classify the symptom first, stop — attaching the wrong
profiler will produce confident but misleading flamegraphs.

## Python

| Recipe | Tool | Entry point |
|--------|------|-------------|
| CPU sampling, production-safe | `py-spy record -o out.svg --pid $PID` | Attach to running process |
| CPU sampling, local script | `python -X dev -m cProfile -o out.prof script.py` + `snakeviz out.prof` | Development only |
| Wall-clock / off-CPU | `pyinstrument -r html script.py` | Includes sleep/IO attribution |
| Allocations | `memray run -o out.bin script.py` then `memray flamegraph out.bin` | Heap and allocation tracking |
| Continuous | `pyroscope` Python SDK | Production, low overhead |

Interpretation:
- `cProfile` overstates CPython internals; prefer `py-spy` when you need
  honest attribution in production.
- `memray`'s "temporary allocations" view exposes churn the heap-size
  view hides.

## Node.js

| Recipe | Tool | Entry point |
|--------|------|-------------|
| CPU sampling, production | `clinic flame -- node app.js` | Produces SVG, safe to attach |
| V8 inspector (dev) | `node --inspect --cpu-prof app.js` then load `.cpuprofile` in Chrome DevTools | Dev only |
| Event-loop delay | `clinic doctor -- node app.js` | Classifies the bottleneck before you pick a deeper tool |
| Heap snapshots | `node --heap-prof app.js` | Compare two snapshots to find growth |
| Continuous | `0x` or `pyroscope` Node SDK | Production |

Interpretation:
- `clinic doctor` is the *first* stop — it tells you whether the loop is
  busy, starved, or blocked on I/O so you do not profile the wrong axis.

## JVM

| Recipe | Tool | Entry point |
|--------|------|-------------|
| CPU sampling, production | `async-profiler -e cpu -d 60 -f out.html $PID` | Low overhead, ships a flamegraph |
| Allocations | `async-profiler -e alloc -d 60 -f out.html $PID` | Pairs with CPU run for full picture |
| Locks / contention | `async-profiler -e lock -d 60 -f out.html $PID` | Surfaces monitor contention |
| Flight Recorder | `jcmd $PID JFR.start duration=60s filename=out.jfr` + JMC | Broad recording for post-hoc analysis |
| Continuous | JFR + Pyroscope / Grafana Phlare | Production |

Interpretation:
- Mixed-mode flamegraphs (CPU + alloc) expose cases where a hot method
  is hot *because* it allocates, not because of compute.

## Go

| Recipe | Tool | Entry point |
|--------|------|-------------|
| CPU | `go test -cpuprofile=cpu.out` or `import _ "net/http/pprof"` then `go tool pprof http://host/debug/pprof/profile` | Built-in |
| Heap | `go tool pprof http://host/debug/pprof/heap` | `--base` to diff two snapshots |
| Goroutine / block / mutex | `/debug/pprof/{goroutine,block,mutex}` | Requires `runtime.SetBlockProfileRate` / `SetMutexProfileFraction` |
| Execution trace | `go tool trace trace.out` | Goroutine scheduling, GC, syscalls |
| Continuous | Pyroscope Go SDK / Datadog continuous profiler | Production |

Interpretation:
- Always diff heaps (`-base`) rather than reading absolute snapshots.
- The execution trace is the only tool that reliably attributes
  goroutine starvation.

## Rust

| Recipe | Tool | Entry point |
|--------|------|-------------|
| CPU sampling | `cargo flamegraph` (Linux: perf; macOS: dtrace) | Build with `debug = true` in release profile |
| Allocations | `dhat-rs` crate | Requires a source-code hook |
| Benchmark regressions | `criterion` | Use before profiling — reproduce first |
| Continuous | `pyroscope-rs` | Production |

## Databases and external dependencies

When the application looks idle, the database or a downstream service is
often the hot path:

- PostgreSQL: `pg_stat_statements` for cumulative query cost;
  `EXPLAIN (ANALYZE, BUFFERS)` for a single query.
- MySQL: `performance_schema.events_statements_summary_by_digest`.
- Redis: `SLOWLOG GET 128`, `MONITOR` only for short bursts.
- HTTP dependencies: distributed tracing — see
  `distributed-tracing-playbook.md` in this references shelf.

## The loop

1. Reproduce the slowness deterministically. A profile of a flaky repro
   is a profile of the flake.
2. Classify the symptom against the table at the top.
3. Pick the matching observer, run it for long enough (≥30s for sampling
   profilers) to collect a representative sample.
4. Read the flamegraph or profile **before** you change any code.
5. Attribute the cost. If a single frame owns ≥20% of samples, that is
   the target. If no frame does, the slowness is distributed and you
   likely picked the wrong observer class.
6. Change one thing. Re-profile. Never change two things in the same
   run — you will never know which one moved the needle.

## Pitfalls

- Profiling in debug builds. CPU attribution is meaningless without
  compiler optimizations.
- Running the profiler on one request. Use steady-state load or record
  across a warm-up window.
- Treating a single flamegraph as ground truth. Sampling profilers have
  variance; confirm with a second run before shipping a fix.
- Attributing cost to `malloc` / `GC` and optimizing the allocator. The
  real bug is almost always the caller generating the churn.
