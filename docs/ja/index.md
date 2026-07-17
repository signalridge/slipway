# Slipway 日本語ドキュメント

Slipway は AI coding host に、小さくユーザー制御された workflow を追加します。まず達成したい task から始め、integration や maintenance の場合だけ protocol と architecture を参照してください。

[English](../en/index.md) · [简体中文](../zh/index.md)

## はじめる

- [はじめに](start-here.md) — Slipway を build または install し、1つの host adapter で1つの task を実行します。
- [インストール](installation.md) — Release compatibility、package、source build、upgrade、removal。
- [コア概念](explanation/concepts.md) — Run、Action、source、Objective、Change、終了の意味。

## ガイド

- [GitHub Issues](guides/github-issues.md) — Objective と Change の使い分け、issue-backed Run。
- [Run、復旧、プライバシー](guides/runs-and-recovery.md) — Run の inspect、stop、resume、retention、removal。
- [マシンプロトコル v2 チュートリアル](guides/machine-protocol-v2.md) — Strict Outcome を使って host integration lifecycle を一通り実行します。

## リファレンス

- [コマンド](reference/commands.md) — 7つの user command、generated adapter が呼び出す `protocol` 操作、およびその flag。
- [ホストアダプター](reference/adapters.md) — Generated target、invocation、ownership safety。
- [マシンプロトコル](reference/machine-protocol.md) — Host integration 用の versioned JSON。

## メンテナー

- [アーキテクチャ](explanation/architecture.md) — Process boundary、package、storage、trust boundary。
- [開発リファレンス](contributing.md) — Repository layout と verification。
- [コントリビューション](../../CONTRIBUTING.md) — Pull Request workflow。
- [Acceptance suite](../../tests/acceptance/README.md) — Executable/manual behavior check。
- [Architecture Decision Records](../../adr/README.md) — User docs から分離した historical rationale。

English、中文、日本語の3つの tree は同じ product を説明します。Exact machine field shape は language-neutral JSON Schema にあり、特定の翻訳が独立した product contract になることはありません。
