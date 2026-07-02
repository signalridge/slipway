# Intent

## Summary
Repair public installation guidance so README and release-note entry points do
not imply that Homebrew or Go install are the only supported paths.

## Complexity Assessment
simple
The change is documentation and release-note wording only. It touches multiple
localized entry surfaces, but it does not change installer behavior, release
generation, package publishing, or runtime code.

## Guardrail Domains
external_api_contracts

## In Scope
- Replace the brew/go-only Quick Start in `README.md`, `README.zh.md`, and
  `README.ja.md` with a concise platform routing table plus adapter
  initialization.
- Align `docs/start-here.md`, `docs/zh/start-here.md`, and
  `docs/ja/start-here.md` with the same short platform routing model.
- Align `docs/how-to/install-and-refresh-adapters.md` and localized variants so
  adapter setup points to platform-specific install routes without duplicating
  the full installation manual.
- Replace the GoReleaser release footer's brew/go-only installation block with a
  short link to the authoritative installation page plus common platform cues.

## Out of Scope
- Changing GoReleaser build, signing, package publishing, or verification logic.
- Changing `docs/installation.md`, which remains the authoritative full install
  matrix.
- Adding a one-line shell installer or `curl | sh` style bootstrap path.
- Editing package-manager repositories, release assets, or generated manifest
  output.

## Constraints
- README must stay short and avoid embedding full macOS/Linux/Windows install
  scripts.
- Linux installation should link to the full installation page because distro
  package choices are too broad for README.
- AI-assisted installation should be exposed as a link to the existing prompt,
  not copied into every entry surface.

## Acceptance Signals
- README and Start Here no longer present brew/go as the only visible install
  choices.
- Windows and Linux have first-class visible routes in the short installation
  entry points.
- Release notes point to the authoritative installation guide rather than
  showing only Homebrew and Go install.
- Markdown/YAML whitespace checks pass.

## Open Questions
None

## Deferred Ideas
- A future docs pass can make localized README navigation point directly to
  localized installation pages instead of the English canonical page.

## Approved Summary
User approved the concise-routing approach in conversation on 2026-07-01: keep
README short, show macOS/Windows/Linux/Go fallback at a glance, keep detailed
commands in `docs/installation.md`, and expose the AI installation prompt as a
link rather than duplicating it.
