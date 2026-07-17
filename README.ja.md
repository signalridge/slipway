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

Slipway は、ユーザーが明示的に起動する AI coding 用の soft autopilot です。AI coding host に小さく復旧可能な workflow を提供しながら、判断と制御はユーザーに残します。

1回の Run は、境界の明確な Action を1つずつ進みます。

```text
orient → 必要なら clarify → implement → code change があり有効な場合は review → summarize
```

Host が実際の作業を行い、Slipway CLI は Run の記録、次の Action の選択、repository change の独立観測、structured recovery を担当します。Model を呼び出さず、GitHub token を保持せず、software が merge や release 可能かをユーザーの代わりに判断しません。

![Slipway Run の lifecycle: 明示的な start から、1回1 Action の loop に入ります。CLI が Action を1つ発行し、host が実行して structured Outcome を返し、CLI が検証・記録して Git を独立に観測してから次を決めます。ユーザーは理由なく skip でき、stop や resume も可能です。Ended は automatic Action queue が空であることだけを意味し、作業が正しい・merge 済み・deploy 済み・release 可能であることは意味しません。](docs/assets/diagrams/lifecycle.svg)

> [!IMPORTANT]
> `slipway --help` に `install`、`uninstall`、`list`、`doctor`、`run`、
> `status`、`stop` が表示される build を使用してください。Package channel は repository より
> 遅れる場合があります。この interface を含む tag が公開されるまでは、current checkout から build してください。

## Slipway を使う理由

- **明示的な開始:** ambient に起動しません。生成された capability または CLI を、特定の task に対して呼び出します。
- **ユーザー制御:** 理由を説明せずに skip、stop、resume、reorder、take over できます。
- **質問より先に事実確認:** Host は repository を調査してから、本当に人間が決める必要のある事項だけを質問します。
- **復旧可能な Run:** Append-only journal と pinned source material により、chat history に依存せず再開できます。
- **任意の GitHub source:** Durable な source が必要なら self-contained Change Issue を使い、Issue が不要・利用不能・不適切なら ad-hoc で開始します。
- **正直な結果:** 実際の command、exit result、finding、known issue、不確実性を報告します。Run の ended は Action queue が空であることだけを示します。

## Current checkout からの quick start

[`go.mod`](go.mod) が指定する Go version で build し、AI coding host の adapter を install します。

```bash
go build -o ./slipway .
./slipway install --tool claude
./slipway doctor
```

Claude では生成された `slipway-run` skill を明示的に呼び出し、1つの task を説明します。別の host では `claude` を次の ID に置き換えます。

```text
codex  copilot  cursor  kilo  kiro  opencode  pi  qwen  windsurf
```

Kiro の初回 install では surface が必要です。

```bash
./slipway install --tool kiro --surface ide   # または: --surface cli
```

生成された capability が machine protocol を操作します。Host を直接 integration する場合は、ad-hoc Run を次のように開始できます。

```bash
./slipway run --json -- "reports に CSV export を追加する"
```

この command は最初の Action を返すだけで、code を変更しません。完全な flow は[はじめに](docs/ja/start-here.md)、integration の詳細は[マシンプロトコル](docs/ja/reference/machine-protocol.md)を参照してください。

## Source と Run

| Source | 使う場面 | Slipway が保存するもの |
| --- | --- | --- |
| Ad hoc | Task が小さい、機微、緊急、offline、または意図的に GitHub で管理しない場合。 | Goal と、その後の Run event。 |
| GitHub Change Issue | Review 可能で revision-pinned な requirements source が必要な場合。 | Stable Issue identity、bounded section catalog、digest で保存した accepted section。 |

GitHub Objective Issue は複数の Change をまとめられますが、issue-backed Run を開始できるのは self-contained Change だけです。Source format は repository が個人 account と Organization のどちらに所有されるかを区別しません。Slipway は GitHub Projects、Organization 専用 Issue Types、Organization 専用 field を必要としません。

生成された `propose` と `decompose` capability は Issue の準備を支援します。これらは host-side operation です。Host が external write を preview し、ユーザーの GitHub access を使い、partial success や failure を報告します。Run/source core は GitHub data を fetch/publish せず、credential も保存しません。独立した `doctor` command は read-only diagnosis のためにユーザー環境の `gh` を呼び出す場合があります。

## 制御と復旧

```bash
./slipway status
./slipway status <run-id> --json
./slipway stop <run-id>
```

Run が input を必要とするとき、生成された host は exact decision、source choice、environment problem、または destructive scope を示します。対応する明示的な応答があるまで続行しません。`stop` は recovery data を残します。Run directory の削除は local recovery を失わせますが、secure erase ではありません。

Run data は repository の Git common directory にある `<git-common-dir>/slipway/runs/` に保存されます。Goal、accepted requirements、answer、command summary が含まれる場合があります。Slipway は collection を最小化しますが、journal に secret がないとは保証しません。Private local data として扱ってください。

機微な情報を扱う前に [Run、復旧、プライバシー](docs/ja/guides/runs-and-recovery.md)を読んでください。

## ドキュメント

### Slipway を使う

- [はじめに](docs/ja/start-here.md)
- [インストール](docs/ja/installation.md)
- [GitHub Issue workflow](docs/ja/guides/github-issues.md)
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
- [Acceptance suite](tests/acceptance/README.md)
- [Architecture Decision Records](adr/README.md)

## ライセンス

Slipway は [BSD 3-Clause License](LICENSE) で提供されます。
