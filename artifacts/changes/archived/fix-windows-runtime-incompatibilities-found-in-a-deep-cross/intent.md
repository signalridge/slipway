# Intent

## Summary
Fix Windows runtime incompatibilities found in a deep cross-platform audit. (1) HIGH: internal/model/evidence.go ComputeFileContentHash hashes raw file bytes with no CRLF normalization, while its sibling normalizeCanonical normalizes \r\n->\n; on Windows (git autocrlf, editors) this makes artifact-freshness reconciliation (internal/engine/artifact/manager.go) and goal-verification evidence digests (internal/engine/progression/evidence_digests.go) report false staleness / failed fresh-evidence for semantically-unchanged files. Normalize CRLF before sha256, and add a repo .gitattributes enforcing eol=lf on hashed artifacts/templates. (2) MEDIUM: cmd/process_other.go isPIDAlive hardcoded false on Windows degrades stale-lock cleanup in internal/fsutil/lock.go; add a real Windows liveness check. (3) MEDIUM: internal/fsutil/atomic.go WriteFileAtomic os.Rename has no Windows sharing-violation retry. (4) MEDIUM: internal/toolgen/worktree_provision.go os.Symlink creation fails on Windows without privilege; add dereference fallback. (5) MEDIUM: internal/toolgen/toolgen.go nativeHookPath pins the committed settings.json hook command to the init-host OS. (6) MEDIUM: internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl prescribes Unix-only grep/perl stub scan.
## Complexity Assessment
complex
<!-- Rationale: 7 distinct edit sites across model/engine/fsutil/toolgen/tmpl plus
     a new build-tagged file and .gitattributes; one change alters the governance
     evidence/digest integrity core (fail-closed, sensitive); two changes alter
     generated external-contract surfaces (settings.json, goal-verification SKILL).
     Verified only via CI's windows-latest job (no local Windows). -->

## Guardrail Domains
<!-- No named sensitive domain (not auth/PII/financial/schema/irreversible). Two
     integrity-adjacent surfaces require fail-closed care:
     - evidence/digest integrity core (internal/model/evidence.go + freshness/goal-verify callers)
     - generated external contracts (settings.json hook command, goal-verification SKILL template) -->

## In Scope
- **(1, HIGH) `internal/model/evidence.go` `ComputeFileContentHash`** — normalize CRLF→LF before `sha256`, **binary-safe** (skip normalization when content is detected binary, e.g. contains NUL), aligning with the existing `normalizeCanonical` CRLF contract. Fix propagates to callers `internal/engine/artifact/manager.go:762,874,936,992` and `internal/engine/progression/evidence_digests.go:941,958`.
- **(1, HIGH) new repo-root `.gitattributes`** — enforce `text=auto eol=lf` for hashed/text artifacts & templates (`artifacts/**`, `*.md`, `*.yaml`/`*.yml`, `*.tmpl`, `*.go`), with binary assets marked `binary`.
- **(2, MED) `cmd/process_other.go` + new `cmd/process_windows.go`** — real `isPIDAlive` on Windows via `golang.org/x/sys/windows` (`OpenProcess`/`GetExitCodeProcess`), replacing hardcoded `false`; restores stale-lock cleanup correctness in `internal/fsutil/lock.go`.
- **(3, MED) `internal/fsutil/atomic.go` `WriteFileAtomic`** — bounded retry around `os.Rename` for Windows sharing-violation (`ERROR_SHARING_VIOLATION`/`ERROR_ACCESS_DENIED`), GOOS=windows-guarded.
- **(4, MED) `internal/toolgen/worktree_provision.go` (+ `internal/state/lifecycle.go` `copyDirRecursive`)** — on `os.Symlink` failure, dereference/copy target content as fallback.
- **(5, MED) `internal/toolgen/toolgen.go` hook command generation (`nativeHookPath`)** — make the committed `settings.json` hook `command` platform-portable instead of pinned to the init-host OS.
- **(6, MED) `internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl`** — replace Unix-only `grep`/`perl` stub scan with a portable/tool-agnostic instruction.
- **Tests** — OS-independent unit tests + dedicated Windows regression/integration tests (per chosen acceptance bar).

## Out of Scope
- The LOW findings: `cmd/path_helpers.go` slash normalization, `internal/engine/wave/parse.go` trailing `\r`, `syscall.EXDEV` non-match on Windows, unused `.ps1` launcher artifact.
- Unix-only shell snippets in **SAST/GHA reference skills** (they analyze *other* repos, not Slipway operation).
- Broad Windows feature parity beyond the six findings — e.g. full mid-run task preemption/kill (`TerminateProcess`); `process_other.go` keeps its clear unsupported-error behavior, only liveness is added.
- Running on real Windows hardware locally (rely on CI `windows-latest`).
- Migrating/rewriting existing recorded digests (LF content hashes are unchanged by normalization, so no migration is needed).

## Constraints
- **Backward compatibility:** for pure-LF content, raw hash == normalized hash, so existing recorded digests stay valid; binary evidence files must remain byte-exact (do not normalize binaries).
- **Generated-surface contracts:** keep `settings.json` and goal-verification SKILL output coherent across OSes; do not regress the e225088 native-hook (`.sh`/`.cmd`/`.ps1`) migration.
- **Fail-closed:** evidence/digest changes must not introduce any bypass/force path; sensitive-domain posture preserved.
- **Dependencies:** prefer `golang.org/x/sys/windows` (already present transitively via `gofrs/flock`); no new top-level deps otherwise.
- **Gates:** must pass `golangci-lint` and the 3-OS test matrix.

## Acceptance Signals
- `go test ./... -count=1` green on **ubuntu-latest, macos-latest, AND windows-latest** (CI matrix).
- `GOOS=windows GOARCH=amd64 go build ./...` and `go vet ./...` green.
- New OS-independent unit tests: `ComputeFileContentHash` is CRLF-invariant for text and byte-exact for binary; Windows `isPIDAlive` returns true for the running process and false for a non-existent PID.
- Dedicated Windows regression/integration tests: a CRLF artifact round-tripped through freshness reconciliation is **not** reported stale; evidence digest stable across LF↔CRLF round-trip.
- `.gitattributes` present and enforcing `eol=lf` on hashed artifact/text paths.
- `golangci-lint` clean.

## Open Questions
None — all six fixes are well-understood; no research spike required.

## Deferred Ideas
- Full Windows process preemption (`TerminateProcess`) for mid-run task kill, beyond liveness.
- PowerShell equivalents for the SAST/GHA reference-skill shell snippets.
- Handle `ERROR_NOT_SAME_DEVICE` (Windows `EXDEV` analog) for cross-volume archive moves.

## Approved Summary
Fix six Windows runtime incompatibilities found in the cross-platform audit, in one
governed change. The HIGH fix normalizes CRLF→LF (binary-safe) before SHA-256 in
`internal/model/evidence.go` `ComputeFileContentHash` so artifact-freshness
reconciliation and goal-verification evidence digests stop reporting false staleness
on Windows (git autocrlf); adds a repo `.gitattributes` enforcing `eol=lf`. Five
MEDIUM platform gaps: real Windows `isPIDAlive` (restores stale-lock cleanup),
`WriteFileAtomic` rename retry on Windows sharing violations, `os.Symlink` dereference
fallback in worktree provisioning, platform-portable generated `settings.json` hook
command, and a tool-agnostic goal-verification stub scan replacing Unix-only
`grep`/`perl`.

Scope boundaries: LOW findings, SAST/GHA reference-skill shell snippets, full mid-run
task-kill parity, and local real-Windows runs are out of scope. Existing LF digests are
unchanged by normalization, so no digest migration.

Primary acceptance signal: `go test ./...` green on the 3-OS CI matrix (incl.
`windows-latest`) + new CRLF-invariance/binary-exact unit tests + dedicated Windows
regression tests (CRLF artifact → not-stale; LF↔CRLF digest stable) + Windows
cross-compile green.

Confirmed by user: 2026-06-15T07:50:11Z
