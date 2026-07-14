# Acceptance and evidence (non-normative)

> This guide explains evidence collection; it is not a release verdict or CLI routing authority. The [Chinese product contract](../zh/reference/product-contract.md) defines all 35 scenarios. The executable [acceptance matrix](../../tests/acceptance/README.md) records current artifacts and honest gaps.

Evidence labels are complementary:

- C — deterministic Go contract/property/race or static generated-template test;
- S — Shell acceptance that invokes a built Slipway binary;
- G — isolated live GitHub.com user-owned fixture;
- H — sanitized Claude/Codex/Pi transcript plus evaluator notes;
- W — native Windows cmd.exe and PowerShell execution;
- R — documentation, website, package, or release validation.

C cannot establish autonomous host behavior H. A local fake endpoint or deterministic publication fault harness is reproducible H/G-adjacent evidence, not live G. A Windows cross-build is not native W. Missing evidence is recorded as `not collected` or `external`; it never controls Run routing, Issue status, Review, delivery, or a CLI exit decision.

The local publication harness deliberately covers timeout-after-success, partial relation failure, duplicate markers, indexing delay, and zero/one/multiple reconciliation without credentials. Live G requires a protected test account and repository; never expose credentials to fork pull requests. Transcript evidence follows the sanitized format under `tests/acceptance/transcripts/` and must not include secrets, raw conversations, or fabricated model runs.

The CI matrix builds `slipway.exe` on `windows-latest` and invokes both native PowerShell and `cmd.exe` assets. Workflow wiring remains only a collector. [Run 29197908671, Windows job 86664073429](https://github.com/signalridge/slipway/actions/runs/29197908671/job/86664073429) completed both assets against source `4c1741ae35b42d903fa1ccc4ec5ae32469aaca47`, so the matrix records W for that source, binary, and asset set. Later relevant changes require a new completed collection. R checkers use the stdlib link and release-artifact validators under `tests/acceptance/`, including built-site routes, archive LICENSE bytes, Scoop, AUR, and package paths.

Run locally available evidence from the repository root with the commands listed in the matrix. Executable acceptance assets live only under `tests/acceptance/`; documentation may link them but must not duplicate them under `scripts/`.
