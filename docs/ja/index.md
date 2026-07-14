# Slipway 日本語ドキュメント

Slipway は、明示的に起動される、Issue 駆動でありながら Issue の有無に制約されない、AI コーディング向けのソフトオートパイロットです。日本語ページは非規範の要約であり、完全な[中国語製品契約](../zh/reference/product-contract.md)と versioned [machine protocol schema](../reference/machine-protocol.schema.json)が実装上の正本です。

```text
Objective Issue (optional planning parent; never executable)
  └─ self-contained Change Issue (the only issue-backed source)
       └─ Run (one revision-pinned, interruptible attempt)
            orient → clarify if needed → implement → review on observed diff → summarize
```

## はじめる

- [はじめに](start-here.md) — インストールから1つの Run までの最短経路。
- [インストール](installation.md) — プラットフォーム別の導入方法と adapter command。
- [製品概要](reference/product-overview.md) — 4軸モデル、6 capability、7 command。

## リファレンス

- [Issue workflow](reference/issue-workflow.md) — Objective/Change marker、label、自己完結性、GitHub の制約、公開手順。
- [コマンド](reference/commands.md) — public command と JSON surface。
- [マシンプロトコル](reference/machine-protocol.md) — versioned Action / Outcome contract と hidden operation。
- [ホストアダプター](reference/adapters.md) — 10種類の host、6 capability、ownership safety。
- [Windows rendering と durability](reference/windows-rendering-and-durability.md) — argv rendering と crash durability。
- [Acceptance と evidence](reference/acceptance-evidence.md) — evidence type と scenario matrix。

## 解説

- [アーキテクチャ](explanation/architecture.md) — package layout と依存関係の方向。
- [Run とプライバシー](explanation/runs-and-privacy.md) — journal の内容、retention、privacy promise。

## 意思決定とシナリオ

- [アーキテクチャ上の意思決定](../decisions/0001-source-bundle-v2.md) — manifest-addressed source bundle。
- [Prompt scenario](../../tests/acceptance/README.md) — host behavior の評価。
