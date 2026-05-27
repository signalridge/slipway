# Integrations

- External APIs: Git CLI is used for repository and worktree inspection.
- Infrastructure bindings: Local filesystem state under artifacts/, .slipway.yaml, and git-local runtime directories.
- Datastores and queues: No service datastore detected by baseline scan; Slipway stores YAML, JSON, JSONL, and Markdown artifacts on disk.
- File formats and protocols: YAML change authority, JSON CLI output, JSONL lifecycle events, Markdown governed artifacts.
- Notes: Integration inventory is deterministic baseline context; refine with project-specific external services when present.
