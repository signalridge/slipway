# Slipway 日本語ドキュメント

Slipway は AIコーディングツールに、小さくユーザー制御されたワークフローを追加します。まず達成したいタスクから始め、統合やメンテナンスの場合だけプロトコルとアーキテクチャのページを参照してください。

[English](../en/index.md) · [简体中文](../zh/index.md)

## はじめる

- [はじめに](start-here.md) — Slipway をビルドまたはインストールし、1つのタスクを実行します。
- [インストール](installation.md) — リリース互換性、パッケージ、ソースビルド、アップグレード、削除。
- [コア概念](explanation/concepts.md) — Run、Action、source、Objective、Change、終了の意味。

## ガイド

- [GitHub Issues](guides/github-issues.md) — Objective と Change の使い分け、Issue ベースの Run。
- [Run、復旧、プライバシー](guides/runs-and-recovery.md) — Run の確認、停止、再開、保持、削除。
- [マシンプロトコル v2 チュートリアル](guides/machine-protocol-v2.md) — Strict Outcome を使ってホスト統合のライフサイクル を一通り実行します。

## リファレンス

- [コマンド](reference/commands.md) — 7つの ユーザーコマンド、生成されたアダプターが呼び出す `protocol` 操作、およびその flag。
- [ホストアダプター](reference/adapters.md) — 生成ターゲット、呼び出し方法、所有権の安全性。
- [マシンプロトコル](reference/machine-protocol.md) — ホスト統合用のバージョン付き JSON。

## メンテナー

- [アーキテクチャ](explanation/architecture.md) — プロセス境界、パッケージ、ストレージ、信頼境界。
- [開発リファレンス](contributing.md) — リポジトリ構成と検証。
- [コントリビューション](../../CONTRIBUTING.md) — プルリクエストのワークフロー。
- [Acceptance suite](../../tests/acceptance/README.md) — 実行可能/手動の動作確認。
- [Architecture Decision Records](../../adr/README.md) — ユーザードキュメントから分離した過去の判断理由。

英語、中国語、日本語の3つのドキュメント群は同じプロダクトを説明します。厳密なマシンフィールドの形状は言語中立の JSON Schema にあり、特定の翻訳が独立したプロダクト契約になることはありません。
