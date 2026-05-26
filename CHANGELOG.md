# Changelog

All notable changes to Slipway will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0](https://github.com/signalridge/slipway/compare/v0.1.0...v0.2.0) (2026-05-26)


### ⚠ BREAKING CHANGES

* **validate:** slipway validate-requirements is removed. Use slipway validate and inspect requirements_contract instead.

### Features

* add release and maintenance automation ([4edf556](https://github.com/signalridge/slipway/commit/4edf556140fc98b00b0b2e174b1e13ccef0b3dbb))
* **ai-native:** Complete workflow realignment ([dfea485](https://github.com/signalridge/slipway/commit/dfea485cc1550bb2d614dd4c6063de2d08daaabb))
* **governance:** add trace and learning foundations ([0d8baf8](https://github.com/signalridge/slipway/commit/0d8baf8be6dd207406fd9f552b2836fc460ad362))
* **governance:** harden runtime boundaries ([b0729b2](https://github.com/signalridge/slipway/commit/b0729b2dc96149fa9242931630f0b0ff53d2d8eb))
* **init:** Initialize slipway repository ([e6e5397](https://github.com/signalridge/slipway/commit/e6e539720b8ee0ea19364a4cc5cf3f45530f5dd8))
* **kernel:** Land thin-kernel refactor waves 1-2, 4-5 ([88ba00e](https://github.com/signalridge/slipway/commit/88ba00e1c388571fefa1cb05a51219ef21192afe))
* **runtime:** slim default handoff surfaces ([dc3b55e](https://github.com/signalridge/slipway/commit/dc3b55e127df3aafc0f87fbb0bbb8f06a30a1487))
* **skills:** Absorb coding discipline and dedupe debugging ([96a08d7](https://github.com/signalridge/slipway/commit/96a08d78198c14e1e4263ca009d70c5963dca766))
* **skills:** Canonicalize exported Slipway skill names ([dfc2425](https://github.com/signalridge/slipway/commit/dfc2425b7952b68664f2870b75b53538fb723a5f))
* **skills:** Land 25-skill catalog with binding registry and routed commands ([ffe2271](https://github.com/signalridge/slipway/commit/ffe2271ca4c0fe299e276558df525e1144515f1e))
* **skills:** Land wave-2/3 plans, hydrate surface, and toolgen support files ([c8f22a3](https://github.com/signalridge/slipway/commit/c8f22a31ead9972955f04441e594a66849d53df1))
* **skills:** Land wave-3 skills and retire distillation provenance ([18bea05](https://github.com/signalridge/slipway/commit/18bea058e1c3bab9c0d2d4cae92c1cfb93ef073f))
* **skills:** tighten generated skill surface guardrails ([db28a2b](https://github.com/signalridge/slipway/commit/db28a2b5d1e139f2f3783f2073a1a58a6a809e32))
* **surface:** Ship route-surface refactor, wave-2/3 closeout, and knowledge-only cleanup ([7cd8b85](https://github.com/signalridge/slipway/commit/7cd8b854d311739f14d3c46a3902791ecaeb52ff))
* **toolgen:** Add standalone slipway workflow skill ([a04850f](https://github.com/signalridge/slipway/commit/a04850fc88bb8c9b2bb5c6ca37786c6a4cf0b9e7))
* **toolgen:** hard cut exported agent surfaces ([6f93cb8](https://github.com/signalridge/slipway/commit/6f93cb868bae9bd630d73b92be784a73b4507c9a))
* **validate:** Retire validate-requirements surface ([9aca090](https://github.com/signalridge/slipway/commit/9aca09046c96879ddb45f8e1f4ec6b5a1212494d))
* **workflow:** Split next from run and harden recovery ([894ca25](https://github.com/signalridge/slipway/commit/894ca25b6f1f9f2a97471e4e5e7067ae2573e317))


### Bug Fixes

* address governed workflow feedback ([0047206](https://github.com/signalridge/slipway/commit/004720673e9810f3c5b8100b96012108565dcbcb))
* **ci:** handle Windows filesystem semantics ([69dd026](https://github.com/signalridge/slipway/commit/69dd02650408a8c956881cb73180561a902ffe30))
* **ci:** handle Windows toolgen tests ([646483b](https://github.com/signalridge/slipway/commit/646483b571659398b9c68f3faa3453344d173a64))
* **ci:** harden remaining Windows tests ([b05cfa5](https://github.com/signalridge/slipway/commit/b05cfa59d0bcdd7a42a51cf4a3d416001a59dcc9))
* **ci:** harden Windows state tests ([4ae6c7d](https://github.com/signalridge/slipway/commit/4ae6c7d80f3e502985170ed45d56351672a9b04c))
* **ci:** normalize Windows template paths ([79e3f3f](https://github.com/signalridge/slipway/commit/79e3f3fc44ba79aba33ad4dc2c5905b37cc87e1c))
* **ci:** repair cross-platform workflow failures ([e5e2b72](https://github.com/signalridge/slipway/commit/e5e2b72bd6754241828ca1fae8882d3794af9571))
* **governance:** defer bundle scaffolding until plan ([e31e56d](https://github.com/signalridge/slipway/commit/e31e56d639ad348592731fd0bafefd0c9de57914))
* **governance:** remove generated catalog layer ([841001d](https://github.com/signalridge/slipway/commit/841001dd9cb8681854199d466f6297ee41c94937))
* **governance:** resolve archived workflow feedback ([ad4611e](https://github.com/signalridge/slipway/commit/ad4611e4624138214990b832d1a1fe902eceba84))
* **progression:** Harden next runtime contracts ([9aebf88](https://github.com/signalridge/slipway/commit/9aebf88e09270d58960c299b28e143a1ea8daa57))
* **toolgen:** Prune stale support files ([974ba10](https://github.com/signalridge/slipway/commit/974ba101d62748feb07a05561823e427eaee0e30))


### Performance

* **cmd:** Reduce governance test runtime ([ac9f464](https://github.com/signalridge/slipway/commit/ac9f46424c1046c4394452a844cf66bc50004f6b))


### Refactoring

* **governance:** remove legacy compatibility paths ([9a21c69](https://github.com/signalridge/slipway/commit/9a21c69058c825b1fd5dd326e07decd4e810674a))


### Dependencies

* **actions:** bump the actions group with 9 updates ([084a00f](https://github.com/signalridge/slipway/commit/084a00f944b9ca3af0d49acba7969b682787c3ca))
* **docker:** bump golang from 1.25-alpine to 1.26-alpine ([5f9282e](https://github.com/signalridge/slipway/commit/5f9282edb3e933a18cbfb5aeae795d7892ea7866))
* **go:** bump the go-minor group with 2 updates ([040c81c](https://github.com/signalridge/slipway/commit/040c81c393ed5bab1744f136b6b2c560fb350d67))

## [Unreleased]
