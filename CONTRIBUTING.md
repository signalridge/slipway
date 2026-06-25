# Contributing to Slipway

Thanks for your interest in improving Slipway. This project uses a
**fork-and-pull-request** workflow: you push changes to your own fork and open a
pull request against [`signalridge/slipway`](https://github.com/signalridge/slipway).
Nobody pushes directly to `main`.

> This page describes **how to submit a contribution** (the collaboration flow).
> For **development details** — repository layout, development commands, the test
> quality policy, adapter/governance contracts, and the coverage gate — see
> [`docs/contributing.md`](docs/contributing.md).

## License of contributions

By contributing, you agree that your contributions are licensed under the
[BSD 3-Clause License](LICENSE), the same license that covers this project. You
also confirm you have the right to submit the work under that license.

## Before you start

- For anything more than a small fix, **open an issue first** to discuss the
  problem and the intended approach. It avoids wasted work on a change that may
  not be merged.
- Keep each pull request focused on one logical change. Unrelated fixes belong in
  separate pull requests.
- Slipway governs its own development. For non-trivial changes you are encouraged
  to drive the work through Slipway's governed lifecycle rather than editing ad
  hoc — see [`AGENTS.md`](AGENTS.md) and the
  [workflow guide](docs/workflow.md).

## Fork-and-pull-request workflow

1. **Fork** the repository on GitHub (use the *Fork* button on
   `signalridge/slipway`).

2. **Clone your fork** and add the canonical repository as the `upstream` remote:

   ```bash
   git clone git@github.com:<your-username>/slipway.git
   cd slipway
   git remote add upstream git@github.com:signalridge/slipway.git
   ```

3. **Create a branch** off `main`. Name it after the change, e.g. with a
   Conventional Commit type prefix:

   ```bash
   git switch -c feat/short-description
   ```

4. **Make your changes**, then verify locally before pushing (see
   [`docs/contributing.md`](docs/contributing.md) for the full command list):

   ```bash
   go build ./...
   go vet ./...
   go test ./... -count=1
   go run ./internal/testlint/cmd/testlint ./...
   ```

5. **Keep your branch current** with upstream `main` and prefer rebasing for a
   linear history:

   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

6. **Commit** using [Conventional Commits](https://www.conventionalcommits.org/)
   (see the next section), **push to your fork**, and **open a pull request**
   against `signalridge/slipway`'s `main` branch:

   ```bash
   git push origin feat/short-description
   ```

## Commit and pull request requirements

This repository derives its changelog and releases automatically from commit
history via [release-please](https://github.com/googleapis/release-please), so
commit and pull-request titles must follow
[Conventional Commits](https://www.conventionalcommits.org/).

- **Pull request titles are validated in CI** and must use the form
  `type(optional-scope): description`. The scope is optional. Allowed types are:

  `feat`, `fix`, `perf`, `refactor`, `deps`, `security`, `revert`, `docs`,
  `style`, `chore`, `test`, `ci`, `build`.

  Examples:

  ```text
  feat: add new feature
  fix(api): resolve timeout issue
  feat!: breaking change
  ```

  Append `!` (e.g. `feat!:`) or a `BREAKING CHANGE:` footer for breaking changes.

- **All CI checks must pass.** Pushing your branch and opening the pull request
  runs the build, `go vet`, the test suite, the `testlint` analyzer, linting, the
  Nix flake build, the security scan, and the governance-kernel coverage gate.
  Run the relevant checks locally first.

- **Update docs and contracts in the same pull request** as the code they
  describe. When command metadata, generated tool surfaces, hooks, or
  lifecycle/gate semantics change, update the code, tests, generated surfaces,
  and docs together — they are reviewed as one product surface.

## Reporting bugs and requesting features

Use [GitHub Issues](https://github.com/signalridge/slipway/issues). For bugs,
include the Slipway version (`slipway --version`), your platform, the command you
ran, and the actual vs. expected behavior. For features, describe the problem you
want solved before proposing a specific solution.

## Development reference

Deeper development guidance lives in
[`docs/contributing.md`](docs/contributing.md): repository layout, the full
development command set, the test quality policy, adapter and governance
contracts, and the governance-kernel coverage gate.
