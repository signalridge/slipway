# Distributed-Tracing Playbook

Use when the slow path spans processes — an HTTP request, a queued job,
a cross-service call. Single-process profilers cannot attribute cost
across network or queue boundaries; tracing can.

## When a trace is the right tool

Reach for tracing when:

- The symptom is **high latency at low CPU** on each individual service.
- You can reproduce the slow request but cannot tell **which hop** is
  slow.
- **Tail latency** (p95 / p99) is the symptom, not median latency.
- You suspect a **fan-out amplification** — N+1 calls downstream.
- A request crosses **async boundaries** (queues, workers) and the
  wall-clock view is not contiguous.

Stick with a single-process profiler (see `profiling-recipes.md`) when
the slowness is isolated to one service's CPU, allocations, or locks.

## Before you trace

Tracing amplifies whatever is already instrumented. Before cutting a
trace, confirm:

1. **All services on the critical path propagate a trace context.** A
   dropped `traceparent` header turns a distributed trace into two
   disconnected single-service traces.
2. **Sampling is high enough to catch the slow request.** If production
   samples at 1%, reproduce with head-based sampling forced on for the
   target request (e.g. a debug header).
3. **Clocks are reasonably synchronized.** Skew larger than the spans
   you are measuring makes the trace unreadable.

## Reading a trace

Work top-down:

1. **Root span wall-clock.** Is the slowness in the root span's own
   work, or in a child? If the root contributes negligible self-time,
   move down.
2. **Longest child span.** Walk the longest branch until you reach a
   leaf whose self-time dominates. That is the hot path.
3. **Fan-out check.** Count sibling spans at each level. A service that
   used to make 1 downstream call and now makes 20 is an N+1
   regression.
4. **Gap analysis.** Time between a span's end and its parent's end is
   un-instrumented work — GC, queue dequeue, lock wait, missing
   instrumentation. Close the gap before blaming the adjacent span.
5. **Error correlation.** Filter by `status=error`. Errors concentrated
   at a single span almost always mark the regression site.

## Service-level recipes

| Goal | Query shape |
|------|-------------|
| Find slowest endpoint | Group by `span.name`, sort by p99 latency |
| Find slowest downstream call | Group by `peer.service`, sort by duration sum |
| Detect N+1 | Group by trace_id, count child spans per trace, sort desc |
| Detect regressions | Compare p99 per `span.name` across two releases |
| Correlate with logs | Filter traces with `status=error`, pivot to logs by `trace_id` |

## Instrumentation hygiene

The playbook only works if instrumentation is honest:

- **Span names are cardinality-bounded.** `GET /users/:id` is good;
  `GET /users/42` is a cardinality bomb.
- **Critical attributes are set on the span, not on logs.** Things like
  `http.status_code`, `db.statement`, `peer.service`, `user_id` belong
  on the span so trace queries can pivot on them.
- **Errors are recorded with `record_exception` / equivalent.** A 500
  that does not set `span.status = error` is invisible to the
  error-filter path above.
- **Async boundaries carry context.** Job enqueues should attach the
  current trace context; the consumer should start a child span from
  that context so the trace stays connected across the queue.

## Sampling strategy

| Mode | When | Caveat |
|------|------|--------|
| Head-based, fixed rate | Steady-state observability | Tail latency may be under-sampled |
| Head-based, forced per request | Reproducing a known slow request | Requires a debug toggle |
| Tail-based | Production p99 hunting | Requires a sampling collector (e.g. OTEL Collector) |
| Error-biased | Error-rate investigations | Combine with baseline sampling |

If you only have head-based fixed-rate sampling, hunt by reproducing
with the forced-sample header in a staging environment.

## Handing a trace to a code fix

Before opening a patch:

1. Identify the **exact span** whose self-time dominates.
2. Identify the **closest source site** that implements the spanned
   operation.
3. Confirm the site is hot in a single-process profile run against the
   same service — trace-level attribution at the service boundary
   should reconcile with profiler-level attribution inside the service.
4. Change one thing. Re-run the traced scenario. Compare p99 span
   duration, not just median — tail is why you traced in the first
   place.

## Anti-patterns

- Treating tracing as a replacement for single-process profiling. Trace
  attribution stops at the process boundary.
- Adding a span around every function. Noise drowns the signal; span at
  boundaries (RPC, DB, queue, external API) and at deliberate internal
  phases, not per-call.
- Reading a single trace as ground truth. Tail latency is inherently
  noisy; confirm with an aggregated p99 query across many traces.
- Optimizing a span whose neighbor's gap is larger. The gap is work you
  have not instrumented, and the win lives there.
- Leaving high-cardinality labels on spans (`user_id`, full URL paths).
  The backend will sample them away under cost pressure and your
  post-incident queries will return nothing.

## Cross-reference

Pair with `profiling-recipes.md` in this shelf: once tracing localizes
the slow hop to one service, the single-process recipe for that
service's runtime is the next step. Tracing tells you *where*;
profiling tells you *why*.
