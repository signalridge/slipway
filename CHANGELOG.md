# Changelog

All notable changes to Slipway will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.37.4](https://github.com/signalridge/slipway/compare/v0.37.3...v0.37.4) (2026-06-29)


### Bug Fixes

* **release:** include release test cleanup ([00b1b6c](https://github.com/signalridge/slipway/commit/00b1b6c27febc7e572fd5a19c72b7025bc8c2253))

## [0.37.3](https://github.com/signalridge/slipway/compare/v0.37.2...v0.37.3) (2026-06-29)


### Bug Fixes

* **release:** repair post-publish checks ([5db9221](https://github.com/signalridge/slipway/commit/5db9221b5c28da1ef0e647350ac1518789d29eb4))

## [0.37.2](https://github.com/signalridge/slipway/compare/v0.37.1...v0.37.2) (2026-06-29)


### Bug Fixes

* **release:** support cosign v3 publishing ([946b593](https://github.com/signalridge/slipway/commit/946b59398528a5878f720e0ba28544c5cdba527c))

## [0.37.1](https://github.com/signalridge/slipway/compare/v0.37.0...v0.37.1) (2026-06-29)


### Bug Fixes

* **cli:** make headless handoff write and archived evidence honest ([#364](https://github.com/signalridge/slipway/issues/364), [#368](https://github.com/signalridge/slipway/issues/368)) ([#378](https://github.com/signalridge/slipway/issues/378)) ([c7f15e2](https://github.com/signalridge/slipway/commit/c7f15e28665e3877b3581e72c5a525f08827687e))
* **lifecycle:** honest S1_PLAN read-only surfaces — next/validate/recovery consistency ([#382](https://github.com/signalridge/slipway/issues/382), [#377](https://github.com/signalridge/slipway/issues/377), [#376](https://github.com/signalridge/slipway/issues/376)) ([#383](https://github.com/signalridge/slipway/issues/383)) ([45b6a46](https://github.com/signalridge/slipway/commit/45b6a462589161db35099016352f6f35874ef575))
* **lifecycle:** surface host subagent-delegation authorization as an actionable next action ([#339](https://github.com/signalridge/slipway/issues/339), [#369](https://github.com/signalridge/slipway/issues/369), [#357](https://github.com/signalridge/slipway/issues/357)) ([#375](https://github.com/signalridge/slipway/issues/375)) ([82d4864](https://github.com/signalridge/slipway/commit/82d4864d5124e507f088010bce958f56472bc9a6))
* **recovery:** make blocker recoveries name a state-valid next action ([#372](https://github.com/signalridge/slipway/issues/372)) ([fe17a6f](https://github.com/signalridge/slipway/commit/fe17a6f4181725785f5691c3fe9ca6f084005e7c)), closes [#341](https://github.com/signalridge/slipway/issues/341) [#343](https://github.com/signalridge/slipway/issues/343) [#346](https://github.com/signalridge/slipway/issues/346) [#347](https://github.com/signalridge/slipway/issues/347) [#356](https://github.com/signalridge/slipway/issues/356)
* **recovery:** route S3 task-plan drift to reexecution + honest ship evidence wording ([#344](https://github.com/signalridge/slipway/issues/344), [#352](https://github.com/signalridge/slipway/issues/352)) ([#379](https://github.com/signalridge/slipway/issues/379)) ([35baa3d](https://github.com/signalridge/slipway/commit/35baa3dd4ca5ee327b9c7b6055f97a4c5e46bc06))


### Dependencies

* **actions:** bump the actions group with 3 updates ([#381](https://github.com/signalridge/slipway/issues/381)) ([8059ff0](https://github.com/signalridge/slipway/commit/8059ff086ff94f84c2158cc3901924d74b679d5b))
* **go:** bump golang.org/x/tools from 0.46.0 to 0.47.0 in the go-minor group ([#380](https://github.com/signalridge/slipway/issues/380)) ([58f1014](https://github.com/signalridge/slipway/commit/58f10142c3c8e03f4016c73ea807651931e600c2))

## [0.37.0](https://github.com/signalridge/slipway/compare/v0.36.0...v0.37.0) (2026-06-28)


### Features

* **coverage:** add public surface gate ([45044f3](https://github.com/signalridge/slipway/commit/45044f374d86e9734798cd36d9e2f814191a4c8c))
* **perf:** add state-read baseline ([#355](https://github.com/signalridge/slipway/issues/355)) ([7130762](https://github.com/signalridge/slipway/commit/7130762123ebbdfc77eadf95120fe0a3a39fa9f3))


### Bug Fixes

* align public command surfaces ([#337](https://github.com/signalridge/slipway/issues/337)) ([a1e135e](https://github.com/signalridge/slipway/commit/a1e135e23850d04cc62a8cf38289baae5c31fcb0))
* **governance:** align lifecycle handoff surfaces ([#365](https://github.com/signalridge/slipway/issues/365)) ([a984a54](https://github.com/signalridge/slipway/commit/a984a54ceb04a91714b1233dd02c89a321ec2412))
* **lifecycle:** cache route read context ([#367](https://github.com/signalridge/slipway/issues/367)) ([2b9305a](https://github.com/signalridge/slipway/commit/2b9305a1390673b5373e2ef0e6c375990b1ecbc8))
* **lifecycle:** expose route on mutating surfaces ([c9e5ab9](https://github.com/signalridge/slipway/commit/c9e5ab953c47432c54160e28653c10127aa8411f))
* **lifecycle:** repair public command contracts ([#348](https://github.com/signalridge/slipway/issues/348)) ([f21cc36](https://github.com/signalridge/slipway/commit/f21cc368c253533f1a29ada92dcac8a6b326cc68))
* **lifecycle:** repair route freshness diagnostics ([#363](https://github.com/signalridge/slipway/issues/363)) ([086dc98](https://github.com/signalridge/slipway/commit/086dc98bc08ec9a2e94915ce8b4cb769e0dcb290))


### Performance

* **state:** complete state read fast paths ([#358](https://github.com/signalridge/slipway/issues/358)) ([bee6a5e](https://github.com/signalridge/slipway/commit/bee6a5e0e74409c1c297aa881542281356ed5e2f))
* **state:** optimize governed read contexts ([#354](https://github.com/signalridge/slipway/issues/354)) ([b52a207](https://github.com/signalridge/slipway/commit/b52a20732df75e378dc7526a9487b125bbf70453))


### Refactoring

* clean stale code surfaces ([#370](https://github.com/signalridge/slipway/issues/370)) ([6fd0d69](https://github.com/signalridge/slipway/commit/6fd0d69a110bbd33068e3c433d7a084bb2da0fb9))
* **state:** enforce engine boundary ([#351](https://github.com/signalridge/slipway/issues/351)) ([333d429](https://github.com/signalridge/slipway/commit/333d429e1ac8212404710c5bcfa6fbb07710a395))

## [0.36.0](https://github.com/signalridge/slipway/compare/v0.35.0...v0.36.0) (2026-06-25)


### Features

* **docs:** migrate docs site from MkDocs to Astro Starlight ([#333](https://github.com/signalridge/slipway/issues/333)) ([340baae](https://github.com/signalridge/slipway/commit/340baae48b76558a92a08904cd113ad617cb64de))
* **docs:** surface legacy Design Philosophy and Governed Workflow deep-dives in sidebar ([#335](https://github.com/signalridge/slipway/issues/335)) ([801440c](https://github.com/signalridge/slipway/commit/801440c6f5e5a6f03feeb01c7229a3bf90333284))

## [0.35.0](https://github.com/signalridge/slipway/compare/v0.34.0...v0.35.0) (2026-06-25)


### Features

* **brand:** replace icon mark with pixel "Slipway" wordmark ([d62226d](https://github.com/signalridge/slipway/commit/d62226dc7b5a8049c317e5b0f430a8cc10b2569a))
* **toolgen:** remove Gemini adapter support ([#331](https://github.com/signalridge/slipway/issues/331)) ([642a74a](https://github.com/signalridge/slipway/commit/642a74a9a86938bdc298557fa292c55e3cc6b75e))

## [0.34.0](https://github.com/signalridge/slipway/compare/v0.33.0...v0.34.0) (2026-06-25)


### ⚠ BREAKING CHANGES

* **governance:** governance evidence contract, CLI JSON, generated skills, and docs change; no compat shim — in-flight S3 changes re-run S3.

### Features

* **config:** public `slipway config` surface + env-var discoverability; fix S2 stale-wave recovery ([#315](https://github.com/signalridge/slipway/issues/315), [#324](https://github.com/signalridge/slipway/issues/324)) ([#329](https://github.com/signalridge/slipway/issues/329)) ([6b9607e](https://github.com/signalridge/slipway/commit/6b9607e55032dbbcea34ceb24e3b4d3e1dbe6d93))
* **governance:** converge S3 review in place on plan edits ([#316](https://github.com/signalridge/slipway/issues/316)) ([b2c398f](https://github.com/signalridge/slipway/commit/b2c398f201d9861b950d0dca544df64f1ffede64))
* **governance:** merge goal-verification + final-closeout into terminal ship-verification gate ([#322](https://github.com/signalridge/slipway/issues/322)) ([61d77ae](https://github.com/signalridge/slipway/commit/61d77ae12b47379d50fd3e66d28c99b68729ac45))
* **handoff:** close session-handoff continuity loop (W2 pressure escalation, R2 resume protocol, R3 SessionStart cleanup) ([#323](https://github.com/signalridge/slipway/issues/323)) ([60955b4](https://github.com/signalridge/slipway/commit/60955b4f9a5a5dd8bed62e98cf22e64ac6edb7ea))


### Bug Fixes

* **governance:** point cache-unreadable remediation at the engine-owned wave-plan cache, not tasks.md ([#325](https://github.com/signalridge/slipway/issues/325)) ([#328](https://github.com/signalridge/slipway/issues/328)) ([d6636aa](https://github.com/signalridge/slipway/commit/d6636aa23e858e77ff1e78f8513c151cc92cef41))
* **governance:** stop false reviewer context-origin diagnostic for multi-fix evidence ([#319](https://github.com/signalridge/slipway/issues/319)) ([#327](https://github.com/signalridge/slipway/issues/327)) ([e3faa45](https://github.com/signalridge/slipway/commit/e3faa455fad761e230cd593f1986a7b14dd82ee1))
* **hooks:** harden against version skew — fail-silent floor + in-repo go-run ([#317](https://github.com/signalridge/slipway/issues/317)) ([39f1f17](https://github.com/signalridge/slipway/commit/39f1f17ddaac2b87ed1cc81efd95a9dff78f682b))
* **hooks:** render in-repo go-run for codex & settings.json inline hooks ([#321](https://github.com/signalridge/slipway/issues/321)) ([d9acf7a](https://github.com/signalridge/slipway/commit/d9acf7a3b8c6d0406ef5c527f766e5b224fc6a53))
* **toolgen:** prune stale command skills ([#330](https://github.com/signalridge/slipway/issues/330)) ([9f22529](https://github.com/signalridge/slipway/commit/9f225292abea90dfd2d4ccb7f04aeeb1b7772460))

## [0.33.0](https://github.com/signalridge/slipway/compare/v0.32.1...v0.33.0) (2026-06-23)


### Features

* **handoff:** add engine-owned per-change handoff ([#312](https://github.com/signalridge/slipway/issues/312)) ([4f0e036](https://github.com/signalridge/slipway/commit/4f0e036b9aa8d7e544d18d816051e3cddfd7d9a6))


### Bug Fixes

* **governance:** repair evidence-recording and recovery UX defects ([#310](https://github.com/signalridge/slipway/issues/310), [#311](https://github.com/signalridge/slipway/issues/311)) ([#314](https://github.com/signalridge/slipway/issues/314)) ([967cc9c](https://github.com/signalridge/slipway/commit/967cc9ca6ded0a54cca6d83094ede46909f1776d))

## [0.32.1](https://github.com/signalridge/slipway/compare/v0.32.0...v0.32.1) (2026-06-23)


### Bug Fixes

* **evidence:** allow in-place stale re-cert of upstream governance skills ([#308](https://github.com/signalridge/slipway/issues/308)) ([61b3b85](https://github.com/signalridge/slipway/commit/61b3b856183ccef58bdcaed975c9067b21e7d227))

## [0.32.0](https://github.com/signalridge/slipway/compare/v0.31.5...v0.32.0) (2026-06-22)


### Features

* **commands:** simplify Workstream A command surface ([#300](https://github.com/signalridge/slipway/issues/300)) ([139cc82](https://github.com/signalridge/slipway/commit/139cc8290030b699e90e3e23050f39cfd3adb796))
* **evidence:** add task result import ([#305](https://github.com/signalridge/slipway/issues/305)) ([99d5be5](https://github.com/signalridge/slipway/commit/99d5be5739b2380818dd2df41cfa6f3966958407))
* **evidence:** demote manual task evidence surface ([#306](https://github.com/signalridge/slipway/issues/306)) ([dfe4d4f](https://github.com/signalridge/slipway/commit/dfe4d4f81e4bc1588f18999aafae80144d31effe))


### Dependencies

* **actions:** bump actions/checkout from 6 to 7 in the actions group ([#304](https://github.com/signalridge/slipway/issues/304)) ([2a9a517](https://github.com/signalridge/slipway/commit/2a9a51704c65de8277be21ddaee27e35149a4885))
* **docker:** bump golang from `7a3e500` to `3ad5730` ([#303](https://github.com/signalridge/slipway/issues/303)) ([a09663c](https://github.com/signalridge/slipway/commit/a09663cbfc16884d40c69f4487d95dcb71402897))

## [0.31.5](https://github.com/signalridge/slipway/compare/v0.31.4...v0.31.5) (2026-06-21)


### Bug Fixes

* add suite-result evidence command ([#295](https://github.com/signalridge/slipway/issues/295)) ([d4b50b3](https://github.com/signalridge/slipway/commit/d4b50b3abcae1571d60b6eab115de4473fdc3925))

## [0.31.4](https://github.com/signalridge/slipway/compare/v0.31.3...v0.31.4) (2026-06-21)


### Refactoring

* remove verified over-compat code + make change creation atomic ([#292](https://github.com/signalridge/slipway/issues/292)) ([03d1eba](https://github.com/signalridge/slipway/commit/03d1ebae279192cd760f243bbaf41d34d3a12515))

## [0.31.3](https://github.com/signalridge/slipway/compare/v0.31.2...v0.31.3) (2026-06-21)


### Refactoring

* remove verified dead code, retire unwired gate, drop legacy compat ([#290](https://github.com/signalridge/slipway/issues/290)) ([dfd47a3](https://github.com/signalridge/slipway/commit/dfd47a3bf59a6e9aee674c83dafaf0d860295294))

## [0.31.2](https://github.com/signalridge/slipway/compare/v0.31.1...v0.31.2) (2026-06-20)


### Bug Fixes

* **execution:** harden auto mode governance ([#288](https://github.com/signalridge/slipway/issues/288)) ([4ad740b](https://github.com/signalridge/slipway/commit/4ad740b3c36fe84f8719d60e280d307f1c8f7428))
* **runtime:** prefer local archived change for unscoped status/validate in its worktree ([#283](https://github.com/signalridge/slipway/issues/283)) ([#284](https://github.com/signalridge/slipway/issues/284)) ([7993c06](https://github.com/signalridge/slipway/commit/7993c067f5ea1eb090d2f33bcbdc0f1b96d1b982))

## [0.31.1](https://github.com/signalridge/slipway/compare/v0.31.0...v0.31.1) (2026-06-20)


### Bug Fixes

* **plan-audit:** flag shared-type changes whose target_files omit the blast radius ([#281](https://github.com/signalridge/slipway/issues/281)) ([88241d2](https://github.com/signalridge/slipway/commit/88241d2bcf341a9db155eedf9aac80b7bc33d270)), closes [#277](https://github.com/signalridge/slipway/issues/277)

## [0.31.0](https://github.com/signalridge/slipway/compare/v0.30.1...v0.31.0) (2026-06-20)


### Features

* **execution:** add opt-in auto mode that auto-advances pure-pacing pauses ([#280](https://github.com/signalridge/slipway/issues/280)) ([9910d9f](https://github.com/signalridge/slipway/commit/9910d9f6f2e0dd698884e1472666bac456711358))


### Bug Fixes

* **repair:** route tasks.md parse-failure drift to fixing tasks.md ([#275](https://github.com/signalridge/slipway/issues/275)) ([#278](https://github.com/signalridge/slipway/issues/278)) ([15af266](https://github.com/signalridge/slipway/commit/15af2662090412a95ad0d9130dd1a6a1780d86e4))
* **runtime:** isolate handoff state per change ([#276](https://github.com/signalridge/slipway/issues/276)) ([708aca4](https://github.com/signalridge/slipway/commit/708aca4694d3c65f38199e8fbee98d11f0187049))
* **toolgen:** bootstrap legacy adapter refreshes ([ea95b37](https://github.com/signalridge/slipway/commit/ea95b370d37492ca8a5b068a214744eadb0f94b1))

## [0.30.1](https://github.com/signalridge/slipway/compare/v0.30.0...v0.30.1) (2026-06-19)


### Bug Fixes

* **status:** prefer bound worktree active change ([#271](https://github.com/signalridge/slipway/issues/271)) ([13bd9dc](https://github.com/signalridge/slipway/commit/13bd9dc83df135e3d7d5406fe59107776106850d))

## [0.30.0](https://github.com/signalridge/slipway/compare/v0.29.0...v0.30.0) (2026-06-18)


### Features

* **governance:** add handoff authoring guidance ([#269](https://github.com/signalridge/slipway/issues/269)) ([2696ff7](https://github.com/signalridge/slipway/commit/2696ff736680591803f3cadfee8a067140244c65))
* resolve current open issues ([#267](https://github.com/signalridge/slipway/issues/267)) ([532ed32](https://github.com/signalridge/slipway/commit/532ed3250d5399a16ee01ca4a658ec5e989f7e21))

## [0.29.0](https://github.com/signalridge/slipway/compare/v0.28.0...v0.29.0) (2026-06-18)


### Features

* **governance:** generalize digest proof reuse ([#264](https://github.com/signalridge/slipway/issues/264)) ([d5cf5a6](https://github.com/signalridge/slipway/commit/d5cf5a62824078b0c42e1ad9682b6a42fb92e2ad))

## [0.28.0](https://github.com/signalridge/slipway/compare/v0.27.0...v0.28.0) (2026-06-18)


### Features

* **governance:** redesign forward lifecycle ([#259](https://github.com/signalridge/slipway/issues/259)) ([2c1a50e](https://github.com/signalridge/slipway/commit/2c1a50e49b67f74616744ece8d5fb5b2a7e4e19d))


### Bug Fixes

* **governance:** align docs skill and help surfaces ([#261](https://github.com/signalridge/slipway/issues/261)) ([2912e0e](https://github.com/signalridge/slipway/commit/2912e0ec00ef8f85c0b793e1ca6c3ab11f0fb1e4))

## [0.27.0](https://github.com/signalridge/slipway/compare/v0.26.0...v0.27.0) (2026-06-17)


### Features

* **governance:** enforce native subagent review set ([#257](https://github.com/signalridge/slipway/issues/257)) ([1e9d1e5](https://github.com/signalridge/slipway/commit/1e9d1e5585cb334d40e561dd85201127a275a82d))


### Bug Fixes

* **ci:** stabilize cwd-sensitive cmd tests ([30c8834](https://github.com/signalridge/slipway/commit/30c8834e59d46220a6f7b8c743bb899b5433c322))
* **governance:** ignore intake open question resolution in digest ([#242](https://github.com/signalridge/slipway/issues/242)) ([5256499](https://github.com/signalridge/slipway/commit/525649957f81bc5644b4c49c9e3229445555e5c7))

## [0.26.0](https://github.com/signalridge/slipway/compare/v0.25.2...v0.26.0) (2026-06-16)


### ⚠ BREAKING CHANGES

* **governance:** engine-consumed reviewer-independence + context-origin attestation ([#239](https://github.com/signalridge/slipway/issues/239))

### Features

* **governance:** engine-consumed reviewer-independence + context-origin attestation ([#239](https://github.com/signalridge/slipway/issues/239)) ([2d2adac](https://github.com/signalridge/slipway/commit/2d2adacd6d85bd10a2317652238f6333912308f4))

## [0.25.2](https://github.com/signalridge/slipway/compare/v0.25.1...v0.25.2) (2026-06-15)


### Bug Fixes

* **governance:** clarify lifecycle timeline transitions ([#236](https://github.com/signalridge/slipway/issues/236)) ([f741779](https://github.com/signalridge/slipway/commit/f7417799dae4f67278e20b4f6b512b3dc5bdbef8))

## [0.25.1](https://github.com/signalridge/slipway/compare/v0.25.0...v0.25.1) (2026-06-15)


### Bug Fixes

* **governance:** self-sufficient per-task wave-evidence flow + non-corrupting archived reads ([#227](https://github.com/signalridge/slipway/issues/227) [#228](https://github.com/signalridge/slipway/issues/228) [#229](https://github.com/signalridge/slipway/issues/229) [#232](https://github.com/signalridge/slipway/issues/232)) ([#235](https://github.com/signalridge/slipway/issues/235)) ([0e76c83](https://github.com/signalridge/slipway/commit/0e76c833926866cdfe80ade051ddb480581ffb41))
* **hooks:** eliminate Claude and Gemini launcher scripts ([#233](https://github.com/signalridge/slipway/issues/233)) ([fa5e6f0](https://github.com/signalridge/slipway/commit/fa5e6f01f3e63ee3a0362e9c571ee78e9a59463d))
* support Windows runtime portability ([#230](https://github.com/signalridge/slipway/issues/230)) ([603e693](https://github.com/signalridge/slipway/commit/603e6934db184367a012abf0f61d695613c37a10))

## [0.25.0](https://github.com/signalridge/slipway/compare/v0.24.1...v0.25.0) (2026-06-15)


### Features

* **ci:** add governance kernel coverage gate ([#226](https://github.com/signalridge/slipway/issues/226)) ([a1cd8d9](https://github.com/signalridge/slipway/commit/a1cd8d92d6cfe85020ca4bd1d3e97bffe13d8077))


### Dependencies

* **docker:** bump golang from `f23e8b2` to `7a3e500` ([#223](https://github.com/signalridge/slipway/issues/223)) ([aa04960](https://github.com/signalridge/slipway/commit/aa0496097202f0c4811ad0be8a0453b796c6af6d))
* **go:** bump golang.org/x/term from 0.43.0 to 0.44.0 in the go-minor group ([#224](https://github.com/signalridge/slipway/issues/224)) ([11f657a](https://github.com/signalridge/slipway/commit/11f657afb2f7e43f139e161ec413de5348d20984))

## [0.24.1](https://github.com/signalridge/slipway/compare/v0.24.0...v0.24.1) (2026-06-15)


### Bug Fixes

* **ci:** stabilize hook path assertion on Windows ([31ce168](https://github.com/signalridge/slipway/commit/31ce168597c28f93d98183e203936862cc943f86))

## [0.24.0](https://github.com/signalridge/slipway/compare/v0.23.0...v0.24.0) (2026-06-15)


### ⚠ BREAKING CHANGES

* **tool:** generated settings register native launchers and slipway tool/slipway hook commands instead of bash "<hook>.sh" and skills/*/scripts/* payloads. Run slipway init --refresh to migrate existing installs; legacy Slipway-owned bash "<hook>.sh" hook entries are pruned while user hooks are preserved.

### Features

* **tool:** replace shell/python hooks & skill scripts with native slipway hook/tool commands ([#218](https://github.com/signalridge/slipway/issues/218)) ([e225088](https://github.com/signalridge/slipway/commit/e2250886331432af423a55e5748f529948fed08b))

## [0.23.0](https://github.com/signalridge/slipway/compare/v0.22.2...v0.23.0) (2026-06-14)


### ⚠ BREAKING CHANGES

* **wave:** the WaveRun.dispatch_mode public JSON value changes from "parallel" to "parallel_subagents", and the validate/next/status --json blocker-code set gains four fail-closed wave blockers. A started parallel wave without an explicit dispatch_mode token is now blocked instead of silently inferred parallel; record dispatch_mode:wave=<n>:parallel_subagents (or degraded_sequential) plus per-task executor_agent handles, then re-run.

### Features

* **wave:** engine-enforced fail-closed safety nets for shared-worktree parallelism ([#214](https://github.com/signalridge/slipway/issues/214)) ([ee86d70](https://github.com/signalridge/slipway/commit/ee86d70261a0aa455bedea44a5de135243026144))
* **worktree:** provision host-adapter surfaces into git worktrees ([#208](https://github.com/signalridge/slipway/issues/208)) ([82e49dc](https://github.com/signalridge/slipway/commit/82e49dca2288bd79c16333000f11f01de5ab8721))


### Bug Fixes

* **codex:** replace deprecated prompt command surfaces with discoverable skills ([#213](https://github.com/signalridge/slipway/issues/213)) ([9407056](https://github.com/signalridge/slipway/commit/94070569f8d40eb7585d154781f9039774c8ec45))
* **governance:** surface scope-contract codebase-map exemption + drop rejected run_summary_version=0 ([#207](https://github.com/signalridge/slipway/issues/207), [#211](https://github.com/signalridge/slipway/issues/211)) ([#216](https://github.com/signalridge/slipway/issues/216)) ([c558c17](https://github.com/signalridge/slipway/commit/c558c1785a7e586975b73dad0bcbcab0a81586bc))
* **hook:** make cross-worktree session handoff informational ([#215](https://github.com/signalridge/slipway/issues/215)) ([62119f2](https://github.com/signalridge/slipway/commit/62119f247423005ee96fe7f60e5aed2580b07f2c))

## [0.22.2](https://github.com/signalridge/slipway/compare/v0.22.1...v0.22.2) (2026-06-14)


### Bug Fixes

* **lifecycle:** soften prose scaffold digest churn ([#202](https://github.com/signalridge/slipway/issues/202)) ([4c54eee](https://github.com/signalridge/slipway/commit/4c54eeeaa12b99e655a36ad8d93c6dec6252712d))

## [0.22.1](https://github.com/signalridge/slipway/compare/v0.22.0...v0.22.1) (2026-06-13)


### Bug Fixes

* **ci:** normalize archive path assertion ([#200](https://github.com/signalridge/slipway/issues/200)) ([005f76a](https://github.com/signalridge/slipway/commit/005f76a8ec3f8e19c50d2de1e59fc3158ed6cf37))

## [0.22.0](https://github.com/signalridge/slipway/compare/v0.21.0...v0.22.0) (2026-06-13)


### Features

* **wave:** compute wave plans from task graph ([#197](https://github.com/signalridge/slipway/issues/197)) ([8d38b82](https://github.com/signalridge/slipway/commit/8d38b82c22ffeb26f4c550a8a8911f4643925a48))


### Bug Fixes

* **status:** expose done-ready and archived status ([#199](https://github.com/signalridge/slipway/issues/199)) ([102cd74](https://github.com/signalridge/slipway/commit/102cd74a38d073f4401f463543b1c66c359b222a))

## [0.21.0](https://github.com/signalridge/slipway/compare/v0.20.0...v0.21.0) (2026-06-12)


### Features

* **wave:** add GSD-style subagent dispatch ([#190](https://github.com/signalridge/slipway/issues/190)) ([91e2be6](https://github.com/signalridge/slipway/commit/91e2be63f99243d17781ca2a9626143201fffd73))


### Bug Fixes

* **cli:** resolve Lattice workflow feedback ([#193](https://github.com/signalridge/slipway/issues/193)) ([f35e1d9](https://github.com/signalridge/slipway/commit/f35e1d9779ee57d992d4dedb5a42ff199d36c6c7))

## [0.20.0](https://github.com/signalridge/slipway/compare/v0.19.0...v0.20.0) (2026-06-12)


### Features

* **decisions:** add dead decision status gate ([#186](https://github.com/signalridge/slipway/issues/186)) ([bb9f0bf](https://github.com/signalridge/slipway/commit/bb9f0bf4ae2b43d3f296d8a68036267a8f25f33e))


### Bug Fixes

* **progression:** prevent S4 evidence self-stale ([#188](https://github.com/signalridge/slipway/issues/188)) ([56b05ee](https://github.com/signalridge/slipway/commit/56b05ee9bb3a67cd485634bbd22105c38b4b644f))

## [0.19.0](https://github.com/signalridge/slipway/compare/v0.18.0...v0.19.0) (2026-06-11)


### Features

* **robustness:** add transactional multi-file writes ([#181](https://github.com/signalridge/slipway/issues/181)) ([2e7cfec](https://github.com/signalridge/slipway/commit/2e7cfec06aaac8b084662ed7a451b36efd56e758))

## [0.18.0](https://github.com/signalridge/slipway/compare/v0.17.0...v0.18.0) (2026-06-11)


### Features

* **coherence:** add generated surface manifest ([#178](https://github.com/signalridge/slipway/issues/178)) ([5ec2904](https://github.com/signalridge/slipway/commit/5ec29047146e281b7e8f3f88890c1edc8f76a59b))
* **evidence:** add sensitive evidence gate ([#179](https://github.com/signalridge/slipway/issues/179)) ([cd7ecdb](https://github.com/signalridge/slipway/commit/cd7ecdbb87277d98430655a8fe1335e47d10c5c4))

## [0.17.0](https://github.com/signalridge/slipway/compare/v0.16.0...v0.17.0) (2026-06-10)


### Features

* **context:** add thin-host disk handoff evidence ([#176](https://github.com/signalridge/slipway/issues/176)) ([8850d12](https://github.com/signalridge/slipway/commit/8850d12c5160d5c9b53a91e9cbff0453d8b51793))
* **verify:** add uncheckable trace coverage status ([#174](https://github.com/signalridge/slipway/issues/174)) ([e20aac5](https://github.com/signalridge/slipway/commit/e20aac52b76fa33ad08ec2672fcaafd270f7eb3e))

## [0.16.0](https://github.com/signalridge/slipway/compare/v0.15.0...v0.16.0) (2026-06-10)


### Features

* **hooks:** add context pressure PostToolUse hook ([#171](https://github.com/signalridge/slipway/issues/171)) ([021f053](https://github.com/signalridge/slipway/commit/021f05352d768f5da13f302eab8735ba759b1e23))


### Bug Fixes

* **model:** freeze reason-code taxonomy ([#173](https://github.com/signalridge/slipway/issues/173)) ([f88b63d](https://github.com/signalridge/slipway/commit/f88b63d1edcc4057ab253a795e22b444acd0981a))

## [0.15.0](https://github.com/signalridge/slipway/compare/v0.14.0...v0.15.0) (2026-06-10)


### Features

* **security:** add Go SAST baseline gate ([#149](https://github.com/signalridge/slipway/issues/149)) ([f1357c1](https://github.com/signalridge/slipway/commit/f1357c1874e7bad42885638cc7b0bd54950c61e7))

## [0.14.0](https://github.com/signalridge/slipway/compare/v0.13.0...v0.14.0) (2026-06-09)


### Features

* **execution:** force within-wave parallel dispatch by default ([#147](https://github.com/signalridge/slipway/issues/147)) ([c7e301c](https://github.com/signalridge/slipway/commit/c7e301cc832dc5e7b86ca66f0d088dbcc263681a))

## [0.13.0](https://github.com/signalridge/slipway/compare/v0.12.0...v0.13.0) (2026-06-09)


### Features

* **governance:** defer assurance.md creation to S3_REVIEW ([#145](https://github.com/signalridge/slipway/issues/145)) ([182c55d](https://github.com/signalridge/slipway/commit/182c55dd70a6b810aac0027e5db087cdb858e0c8))


### Bug Fixes

* **governance:** make S2 scope drift non-destructive ([#142](https://github.com/signalridge/slipway/issues/142)) ([1d814a0](https://github.com/signalridge/slipway/commit/1d814a0a8877bae18f731866e6013f1ea556f1e7))
* **next:** split pending decisions from locked decisions ([#144](https://github.com/signalridge/slipway/issues/144)) ([0b6b840](https://github.com/signalridge/slipway/commit/0b6b8403ed416b47439930994738ef1807feed06))

## [0.12.0](https://github.com/signalridge/slipway/compare/v0.11.6...v0.12.0) (2026-06-08)


### Features

* **cleanup:** add `slipway delete` for abandoned governed changes and worktrees ([#138](https://github.com/signalridge/slipway/issues/138)) ([fc95992](https://github.com/signalridge/slipway/commit/fc9599250ae71a95ed9b0a1902468c9769862f0f))


### Dependencies

* **docker:** bump golang from `91eda97` to `f23e8b2` ([#133](https://github.com/signalridge/slipway/issues/133)) ([78ae1cf](https://github.com/signalridge/slipway/commit/78ae1cf148b8a635461f69cf88712b6eff33dd73))

## [0.11.6](https://github.com/signalridge/slipway/compare/v0.11.5...v0.11.6) (2026-06-08)


### Refactoring

* **artifacts:** defer planning artifact authoring ([#128](https://github.com/signalridge/slipway/issues/128)) ([0d3626e](https://github.com/signalridge/slipway/commit/0d3626e3179dea66786f28d230b9c7977c61f35f))

## [0.11.5](https://github.com/signalridge/slipway/compare/v0.11.4...v0.11.5) (2026-06-07)


### Performance

* **skills:** thin-host heavy governed stages to cut main-thread token span ([#114](https://github.com/signalridge/slipway/issues/114)) ([#122](https://github.com/signalridge/slipway/issues/122)) ([007b62b](https://github.com/signalridge/slipway/commit/007b62b583d58e325402681cda78f95de2f599e4))

## [0.11.4](https://github.com/signalridge/slipway/compare/v0.11.3...v0.11.4) (2026-06-07)


### Bug Fixes

* **intake:** gate Open Questions on checklist structure, not prose ([#104](https://github.com/signalridge/slipway/issues/104)) ([#120](https://github.com/signalridge/slipway/issues/120)) ([cbf716d](https://github.com/signalridge/slipway/commit/cbf716dedb6b2fb678cf04e6c97ab62f05acca4d))

## [0.11.3](https://github.com/signalridge/slipway/compare/v0.11.2...v0.11.3) (2026-06-07)


### Bug Fixes

* align AI command/skill/doc surfaces with real CLI behavior + complete the Claude Code command set ([#117](https://github.com/signalridge/slipway/issues/117)) ([5775142](https://github.com/signalridge/slipway/commit/57751427c10724f315072dc6ad0fff187448d44e))

## [0.11.2](https://github.com/signalridge/slipway/compare/v0.11.1...v0.11.2) (2026-06-07)


### Bug Fixes

* **recovery:** make live P3 lifecycle dead-ends name an executable next action ([#86](https://github.com/signalridge/slipway/issues/86)) ([#108](https://github.com/signalridge/slipway/issues/108)) ([aed8f81](https://github.com/signalridge/slipway/commit/aed8f8128cc55369de3f1c01b5449f961af473bf))

## [0.11.1](https://github.com/signalridge/slipway/compare/v0.11.0...v0.11.1) (2026-06-06)


### Bug Fixes

* **governance:** skill-authored requirements/tasks + substance gates ([#91](https://github.com/signalridge/slipway/issues/91)) ([#113](https://github.com/signalridge/slipway/issues/113)) ([ca8334f](https://github.com/signalridge/slipway/commit/ca8334f9f11ca63eb9d0c622735569c0a0c9fcc2))

## [0.11.0](https://github.com/signalridge/slipway/compare/v0.10.0...v0.11.0) (2026-06-06)


### Features

* **codebase-map:** host-AI semantic staleness self-check + inline refresh ([#80](https://github.com/signalridge/slipway/issues/80)) ([#112](https://github.com/signalridge/slipway/issues/112)) ([33f89ef](https://github.com/signalridge/slipway/commit/33f89ef6b3ebb6bcf6cc7aa16381e761face03b2))


### Bug Fixes

* **governance:** stage-aware assurance traceability; doctor raises no incident before review ([#92](https://github.com/signalridge/slipway/issues/92)) ([#110](https://github.com/signalridge/slipway/issues/110)) ([066850b](https://github.com/signalridge/slipway/commit/066850b735f296e07eea1ea387ce6c725437a33f))

## [0.10.0](https://github.com/signalridge/slipway/compare/v0.9.0...v0.10.0) (2026-06-06)


### ⚠ BREAKING CHANGES

* **governance:** execution-completeness gate, safety-baseline satisfy-path, per-change worktrees (#95, #88) ([#106](https://github.com/signalridge/slipway/issues/106))

### Features

* **governance:** execution-completeness gate, safety-baseline satisfy-path, per-change worktrees ([#95](https://github.com/signalridge/slipway/issues/95), [#88](https://github.com/signalridge/slipway/issues/88)) ([#106](https://github.com/signalridge/slipway/issues/106)) ([4e6963b](https://github.com/signalridge/slipway/commit/4e6963bcfcb6134704f4136e6d9cb1205353c190))

## [0.9.0](https://github.com/signalridge/slipway/compare/v0.8.0...v0.9.0) (2026-06-06)


### Features

* **toolgen:** align generated surfaces with real cobra flags; redesign entry skill ([#103](https://github.com/signalridge/slipway/issues/103)) ([d466a14](https://github.com/signalridge/slipway/commit/d466a14ec80e6dfa973af30572d71df20f48f50c))


### Bug Fixes

* **progression:** reopen to S2_EXECUTE when the Scope Contract fails ([#102](https://github.com/signalridge/slipway/issues/102)) ([7623d94](https://github.com/signalridge/slipway/commit/7623d949755595ede2c83598b9cfea29b89e3499))

## [0.8.0](https://github.com/signalridge/slipway/compare/v0.7.0...v0.8.0) (2026-06-06)


### Features

* **recovery:** generalize stale-evidence re-walk to earliest affected authority ([#98](https://github.com/signalridge/slipway/issues/98)) ([#99](https://github.com/signalridge/slipway/issues/99)) ([81e699b](https://github.com/signalridge/slipway/commit/81e699b77b7bf03175ca320d662a1fac1e860206))

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
