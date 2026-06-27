# Requirements

## Requirements

### Requirement: Public surface coverage target
REQ-001: The system MUST expose a high-risk public-surface coverage target that
includes the `cmd` public lifecycle surfaces `status`, `next`, `validate`,
`done`, and `evidence`, plus `internal/state` verification, worktree, and
runtime-state read/write paths.

#### Scenario: Public surface target is declared
GIVEN the coverage gate is invoked for the public-surface target
WHEN the gate validates or writes a baseline
THEN it uses a declared package set covering `cmd` and `internal/state`
AND it retains surface metadata for the lifecycle and state paths named by this
requirement.

### Requirement: Fail-closed actionable diagnostics
REQ-002: The system MUST fail closed when a declared high-risk package is
missing from the coverage profile, missing a committed floor, excluded from the
target, or below its committed floor, and the diagnostic SHALL identify the
package and the related surface/file metadata.

#### Scenario: Public surface coverage regresses
GIVEN a committed public-surface coverage baseline
WHEN current coverage for a baselined package drops below its floor
THEN `covergate -check` fails
AND stdout names the package, related surface labels, and related source files.

### Requirement: Preserve kernel gate while adding public gate
REQ-003: The system MUST keep the existing governance-kernel coverage baseline
and enforce the new public-surface baseline alongside it in local recipes and CI.

#### Scenario: CI enforces both coverage targets
GIVEN a pull request runs the CI coverage job
WHEN coverage measurement completes
THEN the job checks the kernel target against `coverage-baseline.json`
AND checks the public-surface target against
`coverage-public-surface-baseline.json`.

### Requirement: Reviewable baseline and documentation
REQ-004: The system MUST provide reviewable baseline and operator documentation
for the public-surface target without adding compatibility shims or soft-pass
paths.

#### Scenario: Maintainer runs the gate locally
GIVEN a maintainer runs the local coverage gate recipe
WHEN a public-surface regression is present
THEN the command fails without a skip path
AND the docs describe adding tests or reviewing a baseline diff as the only
remediation paths.
