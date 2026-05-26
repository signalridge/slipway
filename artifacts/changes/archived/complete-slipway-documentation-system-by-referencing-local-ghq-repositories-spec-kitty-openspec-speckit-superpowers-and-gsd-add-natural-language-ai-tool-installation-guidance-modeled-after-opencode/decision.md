# Decision

## Project Context
- Tech Stack: Go CLI, Markdown documentation, MkDocs Material
- Conventions: use tracked docs pages, repo-native commands, focused regression tests, and generated adapter contracts from `internal/toolgen`
- Test Command: `go test -timeout=20m ./... -count=1`
- Build Command: `go build ./...`
- Languages: Go, Markdown, YAML

## Alternatives Considered

- Full documentation and governance rewrite.
  - Tradeoffs: would allow a new lifecycle/docs model, but the evidence shows drift in contracts, not a failed architecture. It risks breaking stable `change.yaml` authority and generated adapter behavior.
  - Decision: rejected.
- README-only patch.
  - Tradeoffs: faster and smaller, but it would not complete the MkDocs system, would not give AI tools an installation path, and would leave `mkdocs.yml` pointing at non-existent pages.
  - Decision: rejected.
- Targeted contract review plus docs-system completion.
  - Tradeoffs: adds several tracked docs pages and one narrow runtime fix, but keeps the existing governance architecture intact and directly satisfies the requested documentation and Open Questions outcomes.
  - Decision: selected.

## Selected Approach

Implement the targeted contract review and docs-system completion:

- Replace the placeholder MkDocs nav with a tracked `docs/` tree covering overview, installation, workflow, command reference, AI-tool adapters, operator guide, and contributor guide.
- Update `README.md` to remain a concise project entrypoint that points readers to the fuller docs system.
- Add a docs GitHub Actions workflow because the current README describes `.github/workflows/docs.yml` but the workflow is absent.
- Add natural-language AI-tool installation guidance modeled after OpenCode/Superpowers style: copy-paste prompt first, concrete verification steps second.
- Keep the Open Questions runtime fix narrow by centralizing resolved/unresolved semantics in `internal/stringutil` and reusing it from intake progression and traceability.

## Interfaces and Data Flow

- Documentation flow:
  - `mkdocs.yml` nav -> tracked files under `docs/`.
  - `README.md` -> high-level entrypoint linking to docs pages.
  - `.github/workflows/docs.yml` -> strict MkDocs build and GitHub Pages deploy from `main`.
- AI adapter flow:
  - `slipway init --tools <ids>` -> `internal/toolgen` -> tool-specific skills/commands/hooks.
  - Docs must mirror the existing adapter contract: Claude/Cursor/Gemini/OpenCode get project-local surfaces; Codex prompts are generated under `$CODEX_HOME/prompts`.
- Governance flow:
  - `intent.md` -> `stringutil.HasBlockingOpenQuestions` -> intake progression and governance traceability.
  - No new lifecycle state, artifact root, or authority file is introduced.

## Rollout and Rollback

- Rollout:
  - Add tracked docs pages and docs workflow.
  - Update MkDocs nav and README entrypoint.
  - Keep the Open Questions helper/test change in the same governed change because it unblocks reliable governed documentation work.
- Verification:
  - `mkdocs build --strict` when MkDocs is available.
  - Targeted Open Questions tests.
  - `go test -timeout=20m ./... -count=1`.
  - `go build ./...`.
- Rollback:
  - Revert the docs/MkDocs/workflow files to return to README-only docs.
  - Revert the `HasBlockingOpenQuestions` helper and callers if intake behavior must return to historical non-empty-section blocking.
  - No data migration or persistent external service state is involved.

## Risk

- Risk: docs overstate release installation paths before a release exists.
  - Mitigation: document local build/source install separately from tagged release install.
- Risk: generated adapter docs drift from `internal/toolgen`.
  - Mitigation: base adapter tables on current toolgen contracts and keep toolgen tests as verification evidence.
- Risk: Open Questions blocker behavior regresses.
  - Mitigation: centralize semantics and add both unit and progression tests.
- Risk: docs workflow adds CI surface.
  - Mitigation: keep workflow limited to dependency install, `mkdocs build --strict`, and Pages deploy on `main`.
