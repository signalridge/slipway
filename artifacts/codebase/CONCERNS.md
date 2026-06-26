# Concerns

- Live protection drift: GitHub rulesets and environments can change outside
  git. This change records request bodies in verification artifacts and must
  re-query live settings before final closeout.
- Required check drift: branch rulesets use exact check context names. A future
  workflow rename can block merges until ruleset `18174607` is updated.
- Path-filtered check risk: ruleset required checks should avoid jobs that do
  not always run. This change intentionally requires CI/security/title checks,
  not docs or Nix path-filtered jobs.
- Tag lockout risk: a tag ruleset with no bypass can prevent legitimate release
  tags on a user-owned repo. Ruleset `18174614` keeps an explicit owner-user
  bypass while restricting arbitrary `v*` tag mutation.
- Secret exposure risk: release workflow inputs are untrusted. The only
  acceptable flow is no-secret validation before any job can read `GH_PAT`,
  `AUR_SSH_PRIVATE_KEY`, package/write/attestation permissions, or the
  protected release environment.
- Floating dependency risk: action tags and `@latest` tool installs are mutable
  supply-chain inputs. Full SHA pins and fixed Go module versions are required
  in workflows touched by this change.
- Override token leakage risk: `SLIPWAY_GITHUB_API_URL` can point at a
  non-public GitHub host. Ambient `GH_TOKEN` and `GITHUB_TOKEN` must not be sent
  to override hosts; override hosts need exact allowlist plus
  `SLIPWAY_GITHUB_API_TOKEN`.
- Git ref confusion risk: `BaseRef` is not shell-interpolated, but option-like
  values can still be parsed by Git as options. Validate before
  `git worktree add`.
- Release smoke drift: hard-coded artifact names rot as GoReleaser config
  changes. Smoke jobs should consume asset names generated from actual `dist/`
  outputs.
- Local evidence gap: this workstation lacks a usable Docker daemon for full
  container snapshot validation, and local syft runs through aqua. CI must cover
  Docker/SBOM with explicit setup steps.
