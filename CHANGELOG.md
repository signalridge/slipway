# Changelog

All notable changes to Slipway will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.7.0](https://github.com/signalridge/slipway/compare/v0.6.0...v0.7.0) (2026-06-05)


### Features

* **recovery:** add recovery guidance surface ([#93](https://github.com/signalridge/slipway/issues/93)) ([926020d](https://github.com/signalridge/slipway/commit/926020ddcdd425beef30b2c3391ecdd18928f97a))

## [0.6.0](https://github.com/signalridge/slipway/compare/v0.5.1...v0.6.0) (2026-06-05)


### Features

* **skills:** generalized test-design technique + language-aware capability routing ([#82](https://github.com/signalridge/slipway/issues/82)) ([71f3521](https://github.com/signalridge/slipway/commit/71f3521f71a91e64361d3ae03bd098a3dec7a3af))


### Bug Fixes

* **governance:** add recovery UX tier-0 restamp ([#87](https://github.com/signalridge/slipway/issues/87)) ([6d65fcc](https://github.com/signalridge/slipway/commit/6d65fccdb49c132f0c4e6754b4ae7822f0122b2f))
* **governance:** align S2 task evidence diagnostics ([#77](https://github.com/signalridge/slipway/issues/77)) ([7ec3002](https://github.com/signalridge/slipway/commit/7ec3002d8cfba2fa969cdfc52bf9db6e5e1ceda3))
* **governance:** harden Open Questions detection against continuations and resolution markers ([#79](https://github.com/signalridge/slipway/issues/79)) ([31e9bda](https://github.com/signalridge/slipway/commit/31e9bda64b179b99bc6d0ae577b21059aeee27a3))

## [0.5.1](https://github.com/signalridge/slipway/compare/v0.5.0...v0.5.1) (2026-06-04)


### Bug Fixes

* **governance:** align health, confirmation, and portable scan ([#59](https://github.com/signalridge/slipway/issues/59), [#61](https://github.com/signalridge/slipway/issues/61), [#62](https://github.com/signalridge/slipway/issues/62)) ([#68](https://github.com/signalridge/slipway/issues/68)) ([5ee33e5](https://github.com/signalridge/slipway/commit/5ee33e5c4683fb6cac848b82f4553e5b9069ba2d))
* **governance:** stamp evidence freshness digests ([#74](https://github.com/signalridge/slipway/issues/74)) ([4483e74](https://github.com/signalridge/slipway/commit/4483e749e9038e0346b52e246a4b635ad3578226))

## [0.5.0](https://github.com/signalridge/slipway/compare/v0.4.1...v0.5.0) (2026-06-03)


### Features

* **evidence:** add task ledger and closeout reuse ([#63](https://github.com/signalridge/slipway/issues/63)) ([d1a94aa](https://github.com/signalridge/slipway/commit/d1a94aa9c78c121804de47d501591148d1d993af))


### Bug Fixes

* **governance:** add stale planning recovery ([#64](https://github.com/signalridge/slipway/issues/64)) ([33642f9](https://github.com/signalridge/slipway/commit/33642f90a33511f12eb1a53c39389edeb5443a98))
* **workflow:** issue [#53](https://github.com/signalridge/slipway/issues/53) tier 1 next/done diagnostics ([#60](https://github.com/signalridge/slipway/issues/60)) ([ce3fb2f](https://github.com/signalridge/slipway/commit/ce3fb2f211cd24a67e6ac6c9bebc8945e7a59507))


### Dependencies

* **actions:** bump the actions group with 2 updates ([#56](https://github.com/signalridge/slipway/issues/56)) ([f72fa41](https://github.com/signalridge/slipway/commit/f72fa411d587a493e33e0b9cad82ca51f83ddb75))

## [0.4.1](https://github.com/signalridge/slipway/compare/v0.4.0...v0.4.1) (2026-06-01)


### Bug Fixes

* **#44:** record-timestamp stale_planning_evidence display + spec-compliance review fidelity ([#49](https://github.com/signalridge/slipway/issues/49)) ([0829d1e](https://github.com/signalridge/slipway/commit/0829d1e909f9bcbd91c9976ed1ed2effebad7b8c))
* **governance:** enforce authored closeout assurance ([#54](https://github.com/signalridge/slipway/issues/54)) ([c79af6d](https://github.com/signalridge/slipway/commit/c79af6d08399c311c7cb34aa87b226feaba41579))
* **new:** scope active change create guard ([#55](https://github.com/signalridge/slipway/issues/55)) ([72604ee](https://github.com/signalridge/slipway/commit/72604ee73a4bad4bc931f76cc7caf55e05d4b04a))
* **state:** stop persisting absolute worktree_path in tracked change.yaml ([#46](https://github.com/signalridge/slipway/issues/46)) ([#51](https://github.com/signalridge/slipway/issues/51)) ([2def1b4](https://github.com/signalridge/slipway/commit/2def1b420828fad0c109fa61d2e511cb5902ad50))

## [0.4.0](https://github.com/signalridge/slipway/compare/v0.3.4...v0.4.0) (2026-06-01)


### Features

* **codebase-map:** nudge discovery-scoped changes off an empty map ([#41](https://github.com/signalridge/slipway/issues/41)) ([c52b647](https://github.com/signalridge/slipway/commit/c52b6474288b3aa2cce9a015b114f80487f52c8d))


### Bug Fixes

* **intake:** reject invalid stdin classification and surface valid tokens ([#43](https://github.com/signalridge/slipway/issues/43)) ([d8d56ad](https://github.com/signalridge/slipway/commit/d8d56adaef9c569ec2d560ea45c17e8b73a3446f))


### Refactoring

* **governance:** remove obsolete agent template surface ([#45](https://github.com/signalridge/slipway/issues/45)) ([66d3ecd](https://github.com/signalridge/slipway/commit/66d3ecda30b102702d708345af02f7d197cc06b2))

## [0.3.4](https://github.com/signalridge/slipway/compare/v0.3.3...v0.3.4) (2026-05-31)


### Bug Fixes

* **governance:** align review blockers with execution handoff ([#37](https://github.com/signalridge/slipway/issues/37)) ([5fffffd](https://github.com/signalridge/slipway/commit/5fffffd11b3861d46333fb2badc17e36652e7c00))

## [0.3.3](https://github.com/signalridge/slipway/compare/v0.3.2...v0.3.3) (2026-05-31)


### Bug Fixes

* **codebase-map:** strengthen freshness handoff ([#31](https://github.com/signalridge/slipway/issues/31)) ([6959d49](https://github.com/signalridge/slipway/commit/6959d49db17114fc1eb9c9b98879a5743178c20c))
* **state:** ignore summary timestamp for task freshness ([#35](https://github.com/signalridge/slipway/issues/35)) ([f134f3c](https://github.com/signalridge/slipway/commit/f134f3c1f1ec34a4bbb103de675ec6618b857588))

## [0.3.2](https://github.com/signalridge/slipway/compare/v0.3.1...v0.3.2) (2026-05-30)


### Bug Fixes

* address issue 24 workflow feedback ([#25](https://github.com/signalridge/slipway/issues/25)) ([6339e82](https://github.com/signalridge/slipway/commit/6339e8277ecdb417594a4bcfa2692be85982014d))

## [0.3.1](https://github.com/signalridge/slipway/compare/v0.3.0...v0.3.1) (2026-05-30)


### Bug Fixes

* **codebase-map:** stop fabricating repo context ([#19](https://github.com/signalridge/slipway/issues/19)) ([0f92e5d](https://github.com/signalridge/slipway/commit/0f92e5dde3fcf0925fd6b141a907662ee9ea5c19))
* **toolgen:** harden find-polluter go list handling ([#22](https://github.com/signalridge/slipway/issues/22)) ([20ed30e](https://github.com/signalridge/slipway/commit/20ed30e86d9b7ed7ded44523f859634e970f9127))

## [0.3.0](https://github.com/signalridge/slipway/compare/v0.2.0...v0.3.0) (2026-05-30)


### Features

* improve governed workflow diagnostics ([#15](https://github.com/signalridge/slipway/issues/15)) ([16c94af](https://github.com/signalridge/slipway/commit/16c94af7e2edee5cf9b01c876314709f99a12936))

## [0.2.0](https://github.com/signalridge/slipway/compare/v0.1.0...v0.2.0) (2026-05-28)


### Features

* AI-agent install prompt + shorten slug length cap ([#12](https://github.com/signalridge/slipway/issues/12)) ([a2fd6f9](https://github.com/signalridge/slipway/commit/a2fd6f965509760ae1c874a9ee573931ce011e53))

## [Unreleased]
