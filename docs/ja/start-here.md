# はじめに

このページは、「リポジトリがある」状態から「自分の管理下で、Slipway が範囲を限定した1件の変更を実行している」状態までの最短経路です。

Slipway は明示的に起動され、Issue 駆動でありながら Issue の有無に制約されず、「完了」を認定することもありません。作業は、少数の安定したサーフェスを介して進みます。

| Slipway のサーフェス | 役割 |
| --- | --- |
| Objective Issue | 複数の独立した delivery を計画するための任意の親です。実行対象にはなりません。 |
| Change Issue | Issue に紐づく Run の唯一の source です。自己完結しており、有効な Requirements をすべて保持します。 |
| Run | `.git/slipway/runs/<run-id>/` 配下に置かれる、revision が固定された中断可能な1回の実行試行です。 |
| Host capabilities | 正確に6つです：`run`、`clarify`、`propose`、`decompose`、`implement`、`review`。 |
| Pinned source | manifest で参照され、digest で固定された chapter catalog です。生の本文は一切永続化されません。 |

CLI が authority です。Host が Action を実行し、Slipway は Action をスケジュールして Git を独立に観測し、復旧履歴を保存します。Host は Issue draft を作成して技術作業を実行できますが、lifecycle state を創作したり、evidence を手作業で編集したり、Issue の文章を命令として扱ったりしてはなりません。

> 日本語版は非規範のガイドです。完全な[中国語製品契約](../zh/reference/product-contract.md)と [machine schema](../reference/machine-protocol.schema.json) が実装上の正本です。

## 進み方を選ぶ

| 状況 | 最初に読むもの |
| --- | --- |
| Slipway を初めて使い、小規模な一連の Run を試したい。 | [Issue workflow](reference/issue-workflow.md)を読み、その後で下記の `slipway run` を実行します。 |
| プラットフォーム別の導入方法と adapter command を知りたい。 | [インストール](installation.md)と[ホストアダプター](reference/adapters.md)。 |
| Run が一時停止・停止した、または状態が分かりにくい。 | [コマンド](reference/commands.md)の `status`、`stop`、resume と、[Run とプライバシー](explanation/runs-and-privacy.md)。 |
| 設計を評価したい。 | [製品概要](reference/product-overview.md)と[アーキテクチャ](explanation/architecture.md)。 |
| machine contract が必要。 | [マシンプロトコル](reference/machine-protocol.md)。 |

## インストールして確認する

利用するプラットフォームに対応した、公式 release に基づく方法を選びます。

| プラットフォーム | 推奨方法 |
| --- | --- |
| macOS | `brew install --cask signalridge/tap/slipway` |
| Windows | `scoop bucket add signalridge https://github.com/signalridge/scoop-bucket`<br>`scoop install slipway` |
| Linux | [インストール](installation.md#linux-package)に記載された `.deb`、`.rpm`、`.apk`、`tar.gz`、AUR、または container image を使用します。 |
| Go の代替手段 | `go install github.com/signalridge/slipway@latest` |

続いて、binary を実行できることを確認します。

```bash
slipway --version
slipway doctor
```

プラットフォームの全対応表、release archive の取得方法、checksum の検証方法、source build の手順については、[インストール](installation.md)を参照してください。

## Host capability を生成する

使用する host 向けの6 capability をインストールするには、Git worktree 内の任意のディレクトリで実行します。

```bash
slipway install --tool claude
slipway install --tool codex,cursor,pi
slipway install --tool all
slipway install --tool kiro --surface ide   # or: --surface cli
```

`--tool` を指定しない場合、Slipway は検出した host directory に対応する adapter をインストールします。`--refresh` は、ownership hash が一致しているファイルだけを更新します。Kiro の初回インストールでは `--surface ide|cli` が必須です。以後の refresh と uninstall では、記録済みの surface が推定されます。

## 1つの Run を開始する

作業は Issue 駆動ですが、Issue の有無には制約されません。複数の独立した delivery を計画する場合に限って Objective を使います。Change は Issue に紐づく唯一の source であり、有効な Requirements をすべて保持していなければなりません。

### Ad-hoc

小規模、機微、緊急、offline の作業、または単に Issue を作りたくない場合は、次のように実行します。

```bash
slipway run --budget 8 --json --root "$PWD" -- "add a CSV export to reports"
```

### Issue-bound

trusted host が厳密な GitHub Change envelope を一度だけ取得し、一時的な raw envelope を CLI に渡します。

```bash
slipway run --budget 8 --json --root "$PWD" \
  --source-file /safe/temp/change-envelope.json -- "implement the bounded Change"
```

marker が有効な本文が Level authority です。title や label の drift は警告されますが、実行を妨げません。CLI は Issue 本文の manifest と、そこから厳密に参照された comment だけを検証し、各 chapter を digest で固定して、範囲を限定した catalog のみを保存します。一時ファイルも GitHub も、ローカルで material を読み出したり resume したりする際には不要です。

公開前に [Issue workflow](reference/issue-workflow.md)を確認してください。Public Issue を private に切り替える機能はありません。機微な作業では、private repository、適切な security channel、または ad-hoc Run が必要になる場合があります。

## ユーザーによる制御

Slipway は、ユーザーが明示的に起動した場合にのみ開始します。Run の実行が許可されると、versioned Action を1つずつ進め、実際の意思決定、source amendment、environment failure、または destructive confirmation が必要な場合にだけ一時停止します。操作に理由は必要ありません。

| 意図 | 動作 |
| --- | --- |
| **Skip this** | 未処理の Action に対する、その時点の正確な skip control を呼び出します。 |
| **Stop** | `slipway stop` を実行します。journal は保持され、Run は resume できます。 |
| **Take over** | 先に停止し、Run ID を保持して報告します。未処理の Action は実行しません。 |
| **Reorder / do X first** | automatic loop を停止して制御をユーザーに戻します。queue を暗黙に変更せず、依頼を skip に読み替えることもありません。 |

作業は、明示的に resume されるまで再開しません。Host は質問する前に repository の事実を調査します。Clarify は [Matt Pocock の `grill-me`](https://github.com/mattpocock/skills) の原則に従います。依存関係のある human decision を、推奨案と trade-off とともに一度に1つだけ尋ねます。request が完全なら質問はせず、grilling によって実行内容の理解が変わった場合にのみ確認を求め、wrap-up を求められたら状態を残さず直ちに停止します。

Review は read-only で Intent/Quality の finding を報告し、repair loop を開始しません。`ended` が意味するのは automatic queue が空になったことだけであり、正しさ、delivery、release readiness を保証するものではありません。

## 一時停止した場合

一時停止は機能の一部です。Slipway が、ユーザーまたは実行環境による対応を必要とする地点に到達したことを示します。すべての一時停止と error は、型付けされ解決可能な variant を持つ構造化された `next` object を返します。組み立て直す必要のある shell string を返すことはありません。

```bash
slipway status --json          # current state and the fresh derived next
slipway status <run-id> --json
```

その後、名前が示す recovery variant に従います。Issue-bound Run の resume では、source mode を正確に1つ選ぶ必要があります。新しい envelope を import する、固定済み snapshot を使って続行すると明示する、または現在の candidate を正確な ID で解決する、のいずれかです。詳しくは[マシンプロトコル](reference/machine-protocol.md)を参照してください。

## 次に読む

- [製品概要](reference/product-overview.md) — 4軸モデル。
- [Issue workflow](reference/issue-workflow.md) — marker、label、公開手順。
- [コマンド](reference/commands.md) — 7つの public command。
- [ホストアダプター](reference/adapters.md) — 10種類の host。
- [Run とプライバシー](explanation/runs-and-privacy.md) — journal に保存される内容。
