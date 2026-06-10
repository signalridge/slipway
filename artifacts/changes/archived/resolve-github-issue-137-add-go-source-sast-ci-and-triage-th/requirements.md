# Requirements

## Requirements

### Requirement: Security Workflow Adds Both Go SAST Engines
REQ-001: The Security workflow MUST add Go-source SAST coverage using both gosec
and CodeQL, while preserving existing `govulncheck`, Trivy, SBOM, and license
jobs.

#### Scenario: pull request receives both SAST checks
GIVEN a pull request targets `main`
WHEN the Security workflow runs
THEN the workflow includes a gosec job and a CodeQL Go analysis job in addition
to the existing security jobs

#### Scenario: main branch receives both SAST checks
GIVEN a push targets `main`
WHEN the Security workflow runs
THEN both gosec and CodeQL Go analysis run for the repository source

### Requirement: Gosec Produces SARIF And Fails On Future Unsuppressed Findings
REQ-002: The gosec CI job MUST scan the full repository with `./...`, emit SARIF,
upload it as an artifact, upload it to Code Scanning, and fail when gosec reports
any unsuppressed finding or processing error.

#### Scenario: gosec finds no unsuppressed findings
GIVEN the current full repository baseline has been fixed or locally suppressed
with rationale
WHEN the gosec job runs
THEN it emits `gosec.sarif`, uploads the SARIF artifact, uploads Code Scanning
results, and exits successfully

#### Scenario: a future unsuppressed finding is introduced
GIVEN a later source change introduces a gosec finding without a code fix or
local suppression
WHEN the gosec job runs
THEN the job fails and the SARIF upload remains available for triage

### Requirement: Full Gosec Baseline Is Resolved
REQ-003: Every current unsuppressed `gosec ./...` finding MUST be resolved by a
code fix or by a local `#nosec` suppression that states the bounded authority or
security rationale at the finding site.

#### Scenario: current full baseline is re-scanned
GIVEN the current full baseline reports 136 findings across `G101`, `G122`,
`G703`, `G304`, `G301`, `G204`, and `G306`
WHEN `go run github.com/securego/gosec/v2/cmd/gosec@v2.27.1 -fmt=json -out=<path> ./...` runs after the change
THEN the report has no unsuppressed findings

#### Scenario: a suppression is used
GIVEN a gosec finding is a controlled Slipway path, permission, credential-like,
or git subprocess pattern
WHEN the code uses `#nosec` to suppress it
THEN the suppression includes the rule ID and a local rationale explaining why
the call site is safe

### Requirement: High Findings Receive Explicit Triage
REQ-004: The HIGH findings from the current baseline (`G101`, `G122`, and
`G703`) MUST be fixed or locally suppressed with a rationale that identifies the
trusted root, bounded path, or false-positive secret pattern.

#### Scenario: high findings are audited
GIVEN the current full gosec baseline includes HIGH findings
WHEN the implementation triages the baseline
THEN each HIGH finding has either changed code that removes the finding or a
local `#nosec` rationale at the finding site

### Requirement: Medium Families Receive Explicit Triage
REQ-005: The MEDIUM finding families (`G304`, `G301`, `G204`, and `G306`) MUST be
reviewed across the full repository and fixed or locally suppressed with
rationale.

#### Scenario: permission findings are audited
GIVEN gosec reports `G301` or `G306`
WHEN the finding touches local runtime evidence, config, or generated artifacts
THEN the implementation either tightens permissions where safe or records why
the broader permission is intentional

#### Scenario: path and subprocess findings are audited
GIVEN gosec reports `G304` or `G204`
WHEN the finding touches Slipway artifact paths or git subprocess invocations
THEN the implementation records the authority boundary or controlled executable
argument pattern at the call site

### Requirement: Guardrail Evidence Fails Closed
REQ-006: Goal verification for this `irreversible_operations` change MUST include
fresh gosec evidence, fresh Go test evidence, and domain/security review
evidence before `done-ready`; missing, stale, or inconclusive evidence MUST block
closeout.

#### Scenario: closeout lacks SAST evidence
GIVEN the change has code and workflow edits
WHEN goal verification runs without fresh gosec output
THEN `G_ship` remains blocked and the change does not reach `done-ready`

#### Scenario: closeout has all required evidence
GIVEN full-repository gosec, Go tests, and review evidence are fresh and passing
WHEN final closeout runs
THEN `G_ship` can approve and the lifecycle can advance to `done-ready`
