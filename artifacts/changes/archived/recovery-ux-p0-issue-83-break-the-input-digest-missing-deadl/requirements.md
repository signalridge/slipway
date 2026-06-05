# Requirements
## Project Context
- Tech Stack: Go
- Conventions: cmd/* CLI over internal/engine/* kernel; table-driven tests
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: Tier-0 backfill heals the input_digest_missing deadlock
REQ-001: When a governance skill has a recorded passing verdict but no
`evidence-digests.yaml` entry, mutating advancement MUST backfill the digest when
the certified inputs did not change after the verdict, and MUST report the
specific `required_skill_stale:<skill>:<input>` only when an input genuinely
changed after the verdict — never the generic `input_digest_missing` deadlock
token. This MUST hold whether or not a digest file already exists for other
skills.

#### Scenario: Orphan with unchanged inputs is backfilled
GIVEN a passing skill verdict, a `skill.evidence_recorded` event, no digest entry
for that skill, and inputs whose mtime is at or before the verdict timestamp
WHEN `slipway run` performs mutating advancement
THEN the skill's digest is stamped (backfilled) and no `required_skill_stale`
blocker is produced for it.

#### Scenario: Orphan with drifted inputs reports the specific stale input
GIVEN the same orphan but an input whose mtime is after the verdict timestamp
WHEN required-skill evidence is evaluated
THEN a `required_skill_stale:<skill>:<changed-input>` blocker is produced and the
generic `input_digest_missing` token is not.

### Requirement: assurance.md is decoupled from the plan-audit digest
REQ-002: The plan-audit input digest MUST NOT include `assurance.md`, so a late
edit to `assurance.md` does not retroactively stale plan-audit or trigger a
stale-planning reopen. `assurance.md` MUST remain an input to the final-closeout
digest.

#### Scenario: Late assurance.md edit does not stale plan-audit
GIVEN a change with a passing plan-audit verdict at S1
WHEN `assurance.md` is created or edited
THEN the plan-audit input digest is unchanged and no
`required_skill_stale:plan-audit:assurance.md` blocker is produced.

### Requirement: Stale-planning recovery prunes digest entries
REQ-003: When stale-planning recovery deletes a skill's `verification/<skill>.yaml`
record, it MUST also remove that skill's `evidence-digests.yaml` entry, so a
digest entry never outlives its verification record. Records that recovery
preserves (e.g. wave-orchestration) MUST keep their digest entries.

#### Scenario: Cleared skills leave no zombie digests
GIVEN a change at S3/S4 with digests stamped for plan-audit, the reviews,
goal-verification, final-closeout, and wave-orchestration
WHEN stale-planning recovery reopens S1_PLAN/audit
THEN the digest entries for the five cleared skills are removed while the
wave-orchestration digest entry remains.

### Requirement: required_skill_stale is an actionable blocker
REQ-004: A `required_skill_stale` blocker MUST be treated as a required-skill
blocker so the stale skill is routed/surfaced as the actionable next skill.

#### Scenario: next routes a stale skill
GIVEN a `required_skill_stale:<skill>:<input>` blocker for a non-display skill
WHEN the next-skill view resolves the actionable skill
THEN it routes to `<skill>`.

### Requirement: Evidence view reports a stale status for digest drift
REQ-005: The required-skill evidence view MUST report `stale` (not `missing` or
`passing`) for a skill with a `required_skill_stale` digest blocker, on both the
precomputed and non-precomputed evidence paths.

#### Scenario: Digest drift shows stale on both paths
GIVEN a required skill with a `required_skill_stale` blocker in `view.Blockers`
WHEN the skill-evidence entries are built (precomputed or non-precomputed)
THEN that skill's status is `stale`.

### Requirement: Minimal Tier-0 evidence restamp command
REQ-006: `slipway evidence restamp --skill X [--dry-run]` MUST stamp the skill's
engine-owned digest only when the verdict is passing and inputs did not change
after the verdict; otherwise it MUST refuse and state why and which skill to
re-run. `--dry-run` MUST report the eligibility decision without mutating state.

#### Scenario: Eligible restamp
GIVEN a passing verdict with inputs unchanged after the verdict and a missing
digest entry
WHEN `slipway evidence restamp --skill X` runs
THEN the digest is stamped; with `--dry-run` it reports it would stamp without
writing.

#### Scenario: Refused restamp names the skill to re-run
GIVEN inputs changed after the verdict (or a non-passing/absent verdict)
WHEN `slipway evidence restamp --skill X` runs
THEN it refuses, states the reason, and names the host skill to re-run.

#### Scenario: Dry-run refuses unavailable digest inputs
GIVEN a passing verdict whose certified input set cannot be computed
WHEN `slipway evidence restamp --skill X --dry-run` runs
THEN it reports `eligible=false`, reason `input_digest_unavailable`, and names
the host skill to re-run without mutating state.

### Requirement: repair routes planning/digest drift at recovery
REQ-007: When `repair` encounters planning/digest drift it MUST point the
operator at `slipway evidence restamp --dry-run` and state that repair does not
mutate engine-owned evidence digests, instead of generic next-action text.

#### Scenario: repair points digest drift at evidence restamp
GIVEN a planning/digest drift reason
WHEN repair derives the next action
THEN the next action references `slipway evidence restamp` and explains repair
does not mutate engine-owned digests.
