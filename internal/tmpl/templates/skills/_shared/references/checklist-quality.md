# Requirements Quality Checklist

- Requirement-to-intent traceability is explicit and specific.
- Requirements are measurable and written as testable behavioral contracts.
- Edge cases and failure modes are called out where they materially affect correctness or safety.
- `decision.md` records alternatives, trade-offs, and concrete remaining risks.

## Generated Skill Template Quality

Use this section only when editing generated Slipway skill templates under
`internal/tmpl/templates/skills/`; it is not a general prompt-writing manual.

- Start steps with familiar action words such as read, run, write, record, or
  stop unless a Slipway term is itself the contract.
- Keep context pointers reliable: say when the agent should read referenced
  material, and keep must-have contract details inline when a pointer would be
  easy to miss.
- Make completion criteria checkable: name the command, path, evidence record,
  output, or state that distinguishes done from not done.
- Prune no-op prose that does not change behavior, while preserving contract
  tokens such as `next_skill.name`, `verification_dir`, reason codes, command
  names, and evidence paths.
