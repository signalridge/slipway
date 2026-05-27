# Conventions

- Naming: CLI commands live in cmd/ with make<Command>Cmd constructors; workflow states and durable schemas live in internal/model.
- File organization: Runtime state helpers belong under internal/state; progression decisions belong under internal/engine/progression.
- Error handling: CLI-facing failures use structured reason codes and typed CLI errors where user remediation matters.
- Configuration: .slipway.yaml is the project-local governance configuration authority.
- State management: change.yaml is current-state authority; lifecycle.jsonl is append-only audit evidence.
- Notes: Generated host-skill templates should stay synchronized with runtime contracts.
