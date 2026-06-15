# Requirements

## Requirements

### Requirement: CRLF-invariant, binary-safe content digest
REQ-001: `internal/model/evidence.go` `ComputeFileContentHash` MUST produce an
identical SHA-256 digest for two files whose contents differ only in line-ending
style (LF vs CRLF), by normalizing `\r\n` to `\n` before hashing text content. It
MUST NOT normalize content detected as binary (e.g. content containing a NUL byte),
which SHALL be hashed byte-for-byte. The digest for pure-LF text content MUST remain
unchanged from the pre-change value so existing recorded digests stay valid.

#### Scenario: Text file digest is line-ending invariant
GIVEN a UTF-8 text file written with LF line endings and a second file with the same
logical content written with CRLF line endings
WHEN `ComputeFileContentHash` is called on each
THEN both calls return the same hex digest.

#### Scenario: Existing LF digests are preserved
GIVEN a text file containing no `\r\n` sequences
WHEN `ComputeFileContentHash` is called
THEN the returned digest equals the raw `sha256` of the file bytes (no behavior change
for LF-only content).

#### Scenario: Binary content is hashed byte-exact
GIVEN two binary files that differ only by a `0x0D 0x0A` byte pair somewhere in their
content, each containing at least one NUL byte
WHEN `ComputeFileContentHash` is called on each
THEN the two digests differ (binary content is not CRLF-normalized).

### Requirement: Freshness and goal-verification tolerate CRLF checkout
REQ-002: Artifact-freshness reconciliation (`internal/engine/artifact/manager.go`) and
goal-verification evidence digests (`internal/engine/progression/evidence_digests.go`)
MUST NOT report a governed text artifact as stale solely because it was materialized as
LF and later re-read as CRLF (the Windows `git core.autocrlf=true` case).

#### Scenario: CRLF re-materialization is not stale
GIVEN a governed text artifact recorded with its content hash while stored as LF
WHEN the same artifact is re-read from disk with CRLF line endings and reconciliation runs
THEN reconciliation reports the artifact as unchanged (not stale) and does not auto-amend it.

#### Scenario: Evidence digest stable across line-ending round-trip
GIVEN a workspace file digested for goal-verification evidence as LF
WHEN the same file is digested again after conversion to CRLF
THEN the recomputed evidence digest equals the recorded digest.

### Requirement: Repository enforces LF for hashed artifacts
REQ-003: The repository MUST contain a root `.gitattributes` that enforces `eol=lf`
for the text and artifact paths whose content is hashed by the governance kernel
(at minimum `artifacts/**`, `*.md`, `*.yaml`/`*.yml`, `*.tmpl`, `*.go`), and MUST mark
known binary asset types as `binary` so they are never line-ending converted.

#### Scenario: Hashed text paths are pinned to LF
GIVEN the repository root `.gitattributes`
WHEN it is inspected for the governed text/artifact globs
THEN each such glob carries an `eol=lf` (or `text=auto eol=lf`) attribute.

### Requirement: Windows process liveness detection
REQ-004: `isPIDAlive` MUST return an accurate liveness result on Windows (true for a
running process, false for a process that has exited), replacing the hardcoded `false`,
so that stale-lock cleanup in `internal/fsutil/lock.go` does not treat a live lock
holder as dead. The Unix behavior MUST be unchanged.

#### Scenario: Running process is reported alive on Windows
GIVEN the current process PID on Windows
WHEN `isPIDAlive` is called with that PID
THEN it returns true.

#### Scenario: Non-existent process is reported dead on Windows
GIVEN a PID that does not correspond to any running process on Windows
WHEN `isPIDAlive` is called with that PID
THEN it returns false.

### Requirement: Atomic write survives Windows sharing violations
REQ-005: `internal/fsutil/atomic.go` `WriteFileAtomic` MUST, on Windows, retry the
final `os.Rename` for a bounded period when it fails with a sharing-violation /
access-denied error caused by a concurrent reader holding the destination open, and
MUST surface the error only after the retry budget is exhausted. Non-Windows behavior
MUST be unchanged.

#### Scenario: Transient sharing violation is retried
GIVEN a Windows rename that initially fails with a sharing-violation error and then
succeeds within the retry budget
WHEN `WriteFileAtomic` performs the destination rename
THEN the write completes successfully without returning an error.

### Requirement: Symlink provisioning degrades gracefully on Windows
REQ-006: When copying a tree that contains a symlink (worktree provisioning in
`internal/toolgen/worktree_provision.go` and the archive copy in
`internal/state/lifecycle.go`), an `os.Symlink` failure caused by missing privilege
MUST fall back to materializing the link target's content at the destination rather
than aborting the copy.

#### Scenario: Symlink creation failure falls back to dereference
GIVEN a source tree containing a symlink and a platform where `os.Symlink` returns a
privilege error
WHEN the tree is copied during provisioning
THEN the destination contains a regular file/dir with the link target's content and the
copy completes without error.

### Requirement: Generated hook command is platform-portable
REQ-007: The hook `command` that Slipway writes into a tool's generated
`settings.json` (`internal/toolgen/toolgen.go`) MUST resolve and run correctly on a
consumer host regardless of which OS ran `slipway init`; it MUST NOT be pinned to the
init-host OS such that a settings file generated on one OS fails on another. The change
MUST NOT regress the native `slipway hook` launcher migration (no reintroduction of
`sh`/`bash`/`python` interpreter prefixes). The registered command MUST avoid
shell-specific chaining operators (`||`, `&&`, `;`, `&`, `|`) so it parses under POSIX
`sh`, `cmd.exe`, Windows PowerShell 5.1, and PowerShell 7+.

#### Scenario: Settings generated on one OS work on another
GIVEN a `settings.json` whose hook command is generated on a non-Windows host
WHEN that repository is used by a tool on a Windows host
THEN the hook command resolves and dispatches to the native `slipway hook` invocation.

#### Scenario: Settings command is shell-neutral
GIVEN a generated `settings.json` hook command
WHEN it is inspected as a command string
THEN it contains only the direct `slipway hook ...` invocation and no shell chaining
operator that would be rejected by Windows PowerShell 5.1.

### Requirement: Goal-verification stub scan is tool-agnostic
REQ-008: The goal-verification skill template
(`internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl`) MUST express its
placeholder/stub scan step using portable, tool-agnostic instructions rather than
Unix-only `grep`/`perl` one-liners that cannot run on a stock Windows host.

#### Scenario: Stub scan has no Unix-only hard dependency
GIVEN the rendered goal-verification SKILL content
WHEN its prescribed stub-scan step is inspected
THEN it does not require `perl` or GNU/BSD-specific `grep` alternation as the only way
to perform the scan.

### Requirement: Adapter hook documentation matches generated contracts
REQ-010: Public documentation for AI-tool adapters MUST describe the generated
`settings.json` hook command and native launcher files accurately. It MUST NOT say
settings-capable adapters register a platform-specific launcher path when the generator
registers a direct `slipway hook ...` command.

#### Scenario: Public docs do not describe retired launcher registration
GIVEN the public adapter and installation documentation
WHEN the hook registration contract is inspected
THEN docs describe direct shell-neutral `slipway hook ...` settings entries and the
separate generated native launcher files without claiming `.cmd` or POSIX launchers are
the registered settings command.

### Requirement: Cross-platform build and test integrity preserved
REQ-009: After the change, the project MUST build and pass tests on all three CI
operating systems (`ubuntu-latest`, `macos-latest`, `windows-latest`), MUST cross-compile
with `GOOS=windows`, and MUST remain `golangci-lint` clean. New platform-specific code
MUST be isolated behind Go build constraints so each target compiles cleanly.

#### Scenario: Windows cross-compile stays green
GIVEN the modified source tree
WHEN `GOOS=windows GOARCH=amd64 go build ./...` and `go vet ./...` run
THEN both succeed with no errors.

#### Scenario: Test matrix stays green
GIVEN the modified source tree with new tests
WHEN `go test ./... -count=1` runs on ubuntu-latest, macos-latest, and windows-latest
THEN all three jobs pass.
