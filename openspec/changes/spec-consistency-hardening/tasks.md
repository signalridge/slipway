## 1. Evidence Ownership Hardening

- [x] 1.1 Add `request_id` as required governance evidence field in `skill-contracts`
- [x] 1.2 Add request-scoped governance evidence path contract in `skill-contracts`
- [x] 1.3 Align `state-persistence` evidence schema and persistence paths
- [x] 1.4 Add readiness rejection scenario for missing `request_id`

## 2. Governed Tasks Structure Contract

- [x] 2.1 Add canonical `tasks.md` structure requirement in `artifact-lifecycle`
- [x] 2.2 Bind `wave-execution` planning input to canonical task-node parsing
- [x] 2.3 Bind `S4_SPEC_BUNDLE` readiness to `tasks.md` structure validity
- [x] 2.4 Bind `G_plan` approval to parseable `tasks.md`

## 3. Contract Authority Cleanup

- [x] 3.1 Make CLI failure taxonomy self-canonical in `cli-commands`
- [x] 3.2 Add write-minimization guidance for optional `mitigation_target`

## 4. Validate

- [x] 4.1 Run `openspec status --change spec-consistency-hardening`
- [x] 4.2 Run `openspec validate --changes`
