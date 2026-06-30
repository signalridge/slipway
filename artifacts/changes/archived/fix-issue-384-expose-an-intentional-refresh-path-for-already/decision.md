# Decision

## Alternatives Considered

### Always allow selected reviewer overwrite
This would remove the current duplicate-evidence barrier for selected S3 review skills. It is simple, but it weakens the default fail-closed behavior and makes accidental restamps indistinguishable from intentional operator reruns.

### Add explicit refresh flag
This keeps the current default rejection and adds an opt-in path for the exact #384 workflow. It reuses the existing verification write and digest stamp path after the actionable guard accepts the explicit refresh.

### Add supplemental review-note storage
This would preserve both the original and rerun notes without replacing the current verification record. It has better audit-history ergonomics, but it requires new storage and read surfaces that are larger than the issue requires.

Selected direction: add an explicit refresh flag.

## Selected Approach

Add a `--refresh-current` flag to `slipway evidence skill`. The flag only affects S3 selected review skills that already have passing evidence for the current review set. Without the flag, existing duplicate-evidence behavior remains unchanged. With the flag, the command may pass the current-evidence guard and then reuse the existing record creation, `state.SaveVerification`, digest stamping, change evidence-ref update, and lifecycle event path.

The option is intentionally narrow: it is not a generic bypass for wrong lifecycle state, unselected review skills, missing execution summaries, failing blockers, or stale digest input failures.

## Interfaces and Data Flow

- CLI interface: `slipway evidence skill --refresh-current --skill <review-skill> --verdict pass ...`.
- Command flow: Cobra parses `--refresh-current`, `makeEvidenceSkillCmd` passes it into the actionable validation, and validation only permits it when the requested skill is in the selected S3 review set and already has passing evidence for the current run.
- Persistence flow: unchanged after validation. The replacement writes `verification/<skill>.yaml`, stamps the evidence digest for passing records, updates `change.yaml` evidence refs, and appends the lifecycle event.
- Generated/agent surface: command metadata and the evidence command partial document the flag.

## Rollout and Rollback

Rollout is additive and local to the evidence command. Rollback is to remove the flag, restore the previous actionable validation signature, remove the tests, and rerun `go test ./cmd -run 'TestEvidenceSkill' -count=1` plus the full suite used for ship verification.

## Risk

- Accidental restamp risk is controlled by requiring `--refresh-current`.
- Scope-creep risk is controlled by limiting the flag to selected S3 review skills and leaving task evidence, ship verification, and non-review skill ordering untouched.
- Documentation drift risk is controlled by updating generated command metadata and template coverage so agents can discover the flag.
