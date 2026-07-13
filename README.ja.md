<div align="center">

<img alt="Slipway" src="docs/assets/brand/slipway-wordmark.svg" width="480">

# Slipway

**ユーザーが明示的に起動する、issue-first かつ issue-gated ではない AI coding 用 soft autopilot。**

[English](README.md) · [简体中文](README.zh.md) · [日本語ドキュメント](docs/ja/start-here.md)

</div>

> **日本語は non-normative summary です。** 完全な[中国語製品契約](docs/zh/reference/product-contract.md)と versioned [machine protocol schema](docs/reference/machine-protocol.schema.json) が実装 authority です。

Slipway は host の repository 調査、human decision の clarification、bounded implementation、任意の read-only Review、中断 Run の recovery、事実報告を支援します。User が明示的に開始し、skip/stop/resume/manual takeover が可能です。

```text
Objective Issue（任意 planning parent、実行不可）
  └─ self-contained Change Issue（唯一の issue-backed source）
       └─ Run（revision を固定した1回の中断可能な試行）
            orient → 必要なら clarify → implement → observed diff なら review → summarize
```

Requirements は一時的な delivery contract で永久 Spec ではありません。Objective は複数の独立 delivery にだけ使い、Change は parent/comments を runtime inheritance しません。最初の exact body marker が Level authority で、label/title は warning projection だけです。`ready-for-agent`、Issue/Project status、test、finding は marker-valid Run を gate しません。

## Quick start

```bash
go install github.com/signalridge/slipway@latest
cd your-repository
slipway install --tool claude

# Ad-hoc escape hatch: tiny/sensitive/urgent/offline/no-Issue choice.
slipway run "レポートに CSV export を追加" --json

# Issue-bound: trusted host が strict manifest-addressed Source Bundle v2 を一度だけ取得。
slipway run "bounded Change を実装" \
  --source-file C:\safe\temp\change-envelope.json --json
```

CLI は model provider を呼ばず GitHub token を持ちません。Host は fetch の trusted attester ですが Issue content は untrusted data です。CLI は manifest が明示的に参照する chapter comments だけを検証し、exact payload を local material として固定し、一度に1 bounded Action catalog を返します。Host は structured `_machine material` operation で chapter を読みます。Amendment は current candidate の明示選択、destructive work は exact-scope one-shot structured grant が必要で、natural-language yes は authority ではありません。

[Issue workflow](docs/ja/reference/issue-workflow.md)は marker、exact Level/Kind labels、self-containment、`gh >= 2.94`/official REST fallback、same-host transfer、100/50 limits、approved markers、partial/ambiguous reconciliation を説明します。

## 6つの explicit host capabilities

```text
slipway-run       slipway-clarify     slipway-propose
slipway-decompose slipway-implement   slipway-review
```

Claude Code、Codex、GitHub Copilot、Cursor、Kilo Code、Kiro、OpenCode、Pi、Qwen Code、Windsurf に対応し、すべて explicit invocation が必要です。Clarify は Matt Pocock MIT `grill-me`/`grilling` の fact-first、dependency order、one question+recommendation、changed shared-understanding confirmation、stateless、immediate wrap-up を保ちます。暗黙の clarification-document capability はありません。Review は read-only で repair/re-review loop を作りません。

## 7つの public commands

```text
install      6 capability を安全に導入
uninstall    pristine managed file だけを削除
list         adapter state を表示
doctor       adapter/Git/GitHub/recovery capability を診断
run          ad-hoc または issue-bound Run を開始
status       recoverable Run を表示
stop         journal を残して停止
```

Hidden versioned `_machine submit/answer/skip/resume/material` は[マシンプロトコル](docs/ja/reference/machine-protocol.md)を参照してください。`ended` は automatic Action queue が空であることだけを示し、correct/delivered/deployed/release-ready/no-findings を認定しません。

## Journal と privacy

Recovery authority は `.git/slipway/runs/<run-id>/journal.jsonl` です。Journal は accepted Requirements、goal、answer、truthful command summary を含む可能性があります。Secret-free は約束せず、raw body/comments、token、env dump、full transcript、hidden reasoning を避け、認識した credential value を command identity を保って redact します。Unix mode/Windows current-user ACL には root/admin、backup、malware、inherited ACL、same-account process の制限があります。

Run directory の削除は recovery capability だけを除き、secure erase、backup purge、key destruction ではありません。[Privacy](docs/ja/explanation/runs-and-privacy.md)、[Windows](docs/ja/reference/windows-rendering-and-durability.md)、[evidence matrix](tests/acceptance/README.md)を参照してください。

Slipway は [BSD 3-Clause License](LICENSE) で配布されます。
