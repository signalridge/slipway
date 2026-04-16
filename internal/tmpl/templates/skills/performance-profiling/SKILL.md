---
skill_id: performance-profiling
domain: verification
function: profile and attribute performance against a baseline before optimizing
tier: T1
primary_attachment: procedure
summary: "Use when a change is suspected to affect performance. Triggers on validate command, goal-verification host, or perf-related user text."
trigger_signals:
  - command: validate
    reason: "validate command invoked; profiling may apply"
  - host: goal-verification
    reason: "Verification host active; perf regression may be in scope"
  - user_text_matches: ["perf", "profiling", "slow", "latency", "regression"]
    reason: "User text signals performance work"
evidence_contract: artifact
bindings:
  - type: command-auto
    target: validate
    attachment: procedure
  - type: host-embedded
    target: goal-verification
    attachment: checklist
---

# Performance Profiling

```
IRON LAW: PROFILE BEFORE OPTIMIZING; ATTRIBUTE BEFORE FIXING
```

## Purpose
Optimizations made without profile evidence usually move cost rather than
remove it. Attribute the cost to a call site, compare against a baseline,
and cite both before proposing a change.

## Procedure
1. Define the workload: a reproducible scenario (benchmark, load profile, or
   production trace) with inputs and expected steady state.
2. Capture a baseline: the version before the change (or before the
   suspected regression) running the same workload. Record environment,
   tool, and command.
3. Capture the candidate profile under the same workload.
4. Attribute cost to call sites using the profile; cite inclusive and
   exclusive time (or analogous metrics for the tool in use). Do not rely
   solely on top-level wall time.
5. Identify the seam to change. Re-profile after the change and report the
   delta against baseline. A change without a measured delta does not
   ship as a perf fix.

## Checklist
- [ ] Workload is reproducible and recorded.
- [ ] Baseline profile captured in the same environment.
- [ ] Candidate profile captured with the same tool and flags.
- [ ] Attribution cites inclusive + exclusive time (or analogous).
- [ ] Post-change delta measured and reported against baseline.

## Anti-patterns
- Optimizing from wall-clock feelings without a profile.
- Comparing profiles captured in different environments.
- Claiming a speedup without re-measuring after the change.

## Helpers
- `scripts/repo-performance-scan.py <path>` — static repository scan for
  performance risk indicators (large files, dependency counts, bundle
  weight). Not a process or binary profiler; attach a real profiler
  from `references/profiling-recipes.md` when you need runtime
  attribution. Accepts `--large-file-threshold-kb=<n>` and `--json`;
  exits 2 when the path is not a directory.
