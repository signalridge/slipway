<div align="center">

<img alt="Slipway" src="docs/assets/brand/slipway-wordmark.svg" width="480">

<br/>
<br/>

<p>
  <a href="https://github.com/signalridge/slipway/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/signalridge/slipway/ci.yml?branch=main&style=for-the-badge&logo=github&label=CI"></a>&nbsp;
  <a href="https://github.com/signalridge/slipway/actions/workflows/docs.yml"><img alt="Docs" src="https://img.shields.io/github/actions/workflow/status/signalridge/slipway/docs.yml?branch=main&style=for-the-badge&logo=astro&label=Docs"></a>&nbsp;
  <a href="https://github.com/signalridge/slipway/releases"><img alt="Release" src="https://img.shields.io/github/v/release/signalridge/slipway?style=for-the-badge&logo=github"></a>
</p>

[ドキュメント](https://signalridge.github.io/slipway/ja/) ·
[はじめに](docs/ja/start-here.md) ·
[インストール](docs/ja/installation.md) ·
[リリースノート](CHANGELOG.md)

[English](README.md) · [简体中文](README.zh.md) · **日本語**

</div>

# Slipway

AIコーディングツールは処理が速い一方で、長時間自律動作させると、依頼内容から逸脱したり、検証していないのに実行済みだと報告したり、変更がリポジトリに反映される前に完了したと判断したりすることがあります。

Slipway は、ユーザーが明示的に起動する AI コーディング向けの補助自動化ツールです。中断・再開しやすい手順を提供しつつ、最終判断と主導権はユーザーが持ち続けます。

1回の Run は、境界の明確な Action を1つずつ進みます。`orient`、`clarify`、`implement`、`review`、`summarize` のいずれかです。この順序は固定された pipeline ではなく、CLI が直前の Outcome と自身による独立した Git 観測から各 Action を導きます。

ホストが実際の作業を行い、Slipway CLI は Run の記録、リポジトリの変更 の独立観測、構造化された復旧を担当します。モデルを呼び出さず、GitHub token を保持せず、ソフトウェア が merge や release 可能かをユーザーの代わりに判断しません。

![Slipway Run の lifecycle: 明示的な start から、1回1 Action の loop に入ります。CLI が Action を1つ発行し、ホストが実行して structured Outcome を返し、CLI が検証・記録して Git を独立に観測してから次を決めます。ユーザーは理由なく skip でき、stop や resume も可能です。Ended は automatic Action queue が空であることだけを意味し、作業が正しい・merge 済み・deploy 済み・release 可能であることは意味しません。](docs/assets/diagrams/lifecycle.svg)

> [!IMPORTANT]
> `slipway --help` に `install`、`uninstall`、`list`、`doctor`、`run`、
> `status`、`stop` と、generated アダプター が呼び出す `protocol` group が表示される build を
> 使用してください。Package channel は リポジトリ より遅れる場合があります。この interface を
> 含む tag が公開されるまでは、現在のチェックアウト から build してください。

## Slipway を使う理由

- **明示的な開始:** ambient に起動しません。生成された capability または CLI を、特定の task に対して呼び出します。
- **ユーザー制御:** 理由を説明せずに skip、stop、resume、reorder、take over できます。
- **質問より先に事実確認:** ホストは リポジトリ を調査してから、本当に人間が決める必要のある事項だけを質問します。
- **復旧可能な Run:** 追記専用ジャーナル と 固定されたソース素材 により、chat history に依存せず再開できます。
- **任意の GitHub source:** Durable な source が必要なら 自己完結型 Change Issue を使い、Issue が不要・利用不能・不適切なら アドホック で開始します。
- **正直な結果:** 実際の コマンド、終了結果、発見事項、既知の問題、不確実性を報告します。Run の ended は Action queue が空であることだけを示します。

## Current checkout からの クイックスタート

[`go.mod`](go.mod) が指定する Go version で build し、AIコーディングツール の アダプター を install します。

```bash
go build -o ./slipway .
./slipway install --tool claude
./slipway doctor
```

Claude では生成された `slipway-run` skill を明示的に呼び出し、1つの task を説明します。別の ホスト では `claude` を次の ID に置き換えます。

```text
codex  copilot  cursor  kilo  kiro  opencode  pi  qwen  windsurf
```

Kiro の初回 install では surface が必要です。

```bash
./slipway install --tool kiro --surface ide   # または: --surface cli
```

生成された capability が マシンプロトコル を操作します。ホストを直接 integration する場合は、アドホック Run を次のように開始できます。

```bash
./slipway run -- "reports に CSV export を追加する"
```

```text
Run 32e483f2-d92e-467c-abd8-1d1649e43778 started.
State: active
Goal: reports に CSV export を追加する
Budget remaining: 7
Current action: orient (bd6200d6-bd4b-4255-b877-d650b92404d6)
Next choices:
- submit-outcome-file: requires outcome_file (path via --outcome-file)
- submit-outcome-stdin: slipway protocol submit --run 32e483f2-… --action bd6200d6-… --root /path/to/reports --outcome-stdin
- skip-action: slipway protocol skip --run 32e483f2-… --action bd6200d6-… --root /path/to/reports
```

これが Run 全体の形です。境界の明確な Action が1つ、remaining budget、そして ホストが取りうる各分岐に対する正確な次の command——理由が一切不要な skip も含みます。Action を選び記録したのは CLI であり、code は変更していません。`--json` を付けると machine-readable 形式になります。

完全な flow は[はじめに](docs/ja/start-here.md)、integration の詳細は[マシンプロトコル](docs/ja/reference/machine-protocol.md)を参照してください。

## Source と Run

| Source | 使う場面 | Slipway が保存するもの |
| --- | --- | --- |
| Ad hoc | Task が小さい、機微、緊急、offline、または意図的に GitHub で管理しない場合。 | Goal と、その後の Run event。 |
| GitHub Change Issue | Review 可能で revision-pinned な requirements source が必要な場合。 | Stable Issue identity、bounded section catalog、digest で保存した accepted section。 |

GitHub Objective Issue は複数の Change をまとめられますが、issue-backed Run を開始できるのは 自己完結型 Change だけです。Source format は リポジトリ が個人 account と Organization のどちらに所有されるかを区別しません。Slipway は GitHub Projects、Organization 専用 Issue Types、Organization 専用 field を必要としません。

生成された `propose` と `decompose` capability は Issue の準備を支援します。これらは host-side operation です。ホストが external write を preview し、ユーザーの GitHub access を使い、partial success や failure を報告します。Run/source core は GitHub data を fetch/publish せず、認証情報 も保存しません。独立した `doctor` command は read-only diagnosis のためにユーザー環境の `gh` を呼び出す場合があります。

## 制御と復旧

```bash
./slipway status
./slipway status <run-id> --json
./slipway stop <run-id>
```

Run が input を必要とするとき、生成された ホストは exact decision、source choice、environment problem、または destructive scope を示します。対応する明示的な応答があるまで続行しません。`stop` は 復旧データ を残します。Run directory の削除は local recovery を失わせますが、secure erase ではありません。

Run data は リポジトリ の Git common directory にある `<git-common-dir>/slipway/runs/` に保存されます。Goal、accepted requirements、answer、command summary が含まれる場合があります。Slipway は collection を最小化しますが、ジャーナル に secret がないとは保証しません。Private local data として扱ってください。

機微な情報を扱う前に [Run、復旧、プライバシー](docs/ja/guides/runs-and-recovery.md)を読んでください。

## ドキュメント

### Slipway を使う

- [はじめに](docs/ja/start-here.md)
- [インストール](docs/ja/installation.md)
- [GitHub Issue ワークフロー](docs/ja/guides/github-issues.md)
- [Run、復旧、プライバシー](docs/ja/guides/runs-and-recovery.md)
- [コア概念](docs/ja/explanation/concepts.md)

### Exact surface を調べる

- [コマンドリファレンス](docs/ja/reference/commands.md)
- [ホストアダプター](docs/ja/reference/adapters.md)
- [マシンプロトコル](docs/ja/reference/machine-protocol.md)
- [アーキテクチャ](docs/ja/explanation/architecture.md)

### 開発に参加する

- [コントリビューション](CONTRIBUTING.md)
- [開発リファレンス](docs/ja/contributing.md)
- [Acceptance suite](acceptance/README.md)
- [Architecture Decision Records](adr/README.md)

## ライセンス

Slipway は [BSD 3-Clause License](LICENSE) で提供されます。
