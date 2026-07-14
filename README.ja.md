<div align="center">

<img alt="Slipway" src="docs/assets/brand/slipway-wordmark.svg" width="480">

<br/>
<br/>

<p>
  <a href="https://github.com/signalridge/slipway/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/signalridge/slipway/ci.yml?branch=main&style=for-the-badge&logo=github&label=CI"></a>&nbsp;
  <a href="https://github.com/signalridge/slipway/actions/workflows/docs.yml"><img alt="Docs" src="https://img.shields.io/github/actions/workflow/status/signalridge/slipway/docs.yml?branch=main&style=for-the-badge&logo=astro&label=Docs"></a>&nbsp;
  <a href="https://github.com/signalridge/slipway/releases"><img alt="Release" src="https://img.shields.io/github/v/release/signalridge/slipway?style=for-the-badge&logo=github"></a>&nbsp;
  <a href="https://pkg.go.dev/github.com/signalridge/slipway"><img alt="Go Reference" src="https://img.shields.io/badge/Go-Reference-00ADD8?style=for-the-badge&logo=go&logoColor=white"></a>
</p>

<p>
  <a href="docs/ja/installation.md"><img alt="Homebrew Cask" src="https://img.shields.io/badge/Homebrew_Cask-FBB040?style=flat-square&logo=homebrew&logoColor=black"></a>
  <a href="docs/ja/installation.md"><img alt="Scoop" src="https://img.shields.io/badge/Scoop-00BFFF?style=flat-square&logo=windows&logoColor=white"></a>
  <a href="docs/ja/installation.md"><img alt="AUR" src="https://img.shields.io/badge/AUR-1793D1?style=flat-square&logo=archlinux&logoColor=white"></a>
  <a href="docs/ja/installation.md"><img alt="Nix" src="https://img.shields.io/badge/Nix-5277C3?style=flat-square&logo=nixos&logoColor=white"></a>
  <a href="docs/ja/installation.md"><img alt="Docker" src="https://img.shields.io/badge/Docker-2496ED?style=flat-square&logo=docker&logoColor=white"></a>
  <a href="docs/ja/installation.md"><img alt="deb" src="https://img.shields.io/badge/deb-A81D33?style=flat-square&logo=debian&logoColor=white"></a>
  <a href="docs/ja/installation.md"><img alt="rpm" src="https://img.shields.io/badge/rpm-EE0000?style=flat-square&logo=redhat&logoColor=white"></a>
  <a href="docs/ja/installation.md"><img alt="apk" src="https://img.shields.io/badge/apk-0D597F?style=flat-square&logo=alpinelinux&logoColor=white"></a>
  <a href="docs/ja/installation.md"><img alt="Go" src="https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white"></a>
</p>

[Documentation](https://signalridge.github.io/slipway/) |
[Start Here](docs/ja/start-here.md) |
[Quick Start](#quick-start) |
[Installation](docs/ja/installation.md) |
[Release Notes](CHANGELOG.md)

<br/>

**English** · [简体中文](README.zh.md) · [日本語](README.ja.md)

</div>

# Slipway

**明示的に起動される、Issue 駆動だが Issue で制限されない、AI コーディング向けの
ソフトオートパイロット。コードを書くのはホストです。Slipway は範囲を限定した作業を
スケジュールし、ソースを固定し、「完了」を認定することなく事実を報告します。**

> **日本語版は非規範的な概要です。** 完全な
> [中国語版製品契約](docs/zh/reference/product-contract.md)と、バージョン管理された
> [マシンプロトコルスキーマ](docs/reference/machine-protocol.schema.json)が、
> 実装上の正規な根拠です。

AI コーディングホストは高速ですが、目標から逸れたり、セッションをまたぐと文脈を
見失ったり、Issue を形式的な承認印として扱ったりすることがあります。Slipway は、
1 つの作業単位を、統制され復旧可能な Run に変えます。ホストはリポジトリを調査し、
人間による真の判断が必要な点を明確化し、範囲を限定した変更を実装し、観測された差分を
レビューして要約します。これらはすべて、ユーザーの明示的な制御下で行われます。

Slipway は、ホステッドサービスでも、プロジェクトトラッカーでも、AI コーディング
ツールの代替でもありません。エージェントの作業を限定的かつ復旧可能で、誠実なものにする
コントロールプレーンです。モデルプロバイダーを内包せず、GitHub トークンも保持しません。
信頼されたホストがソースを取得し、CLI がそれを検証します。

```text
Objective Issue（任意の計画上の親。実行対象にはならない）
  └─ 自己完結した Change Issue（Issue に基づく唯一のソース）
       └─ Run（リビジョンが固定された、中断可能な 1 回の試行）
            orient → 必要に応じて clarify → implement → 観測された差分を review → summarize
```

## Slipway を選ぶ理由

| Capability | もたらす変化 |
| --- | --- |
| **Requirements のみ** | Slipway は Spec、Delta、永続的な要件レジストリを保持しません。open 状態の Issue は一時的なデリバリー契約であり、デリバリー後はコードとテストが現時点の事実になります。 |
| **Issue 駆動だが Issue で制限されない** | 自明でない作業は自己完結した Change Issue から始めますが、GitHub が Run を妨げることはありません。微小、機微、緊急、オフライン、または意図的に追跡しない作業はアドホックに開始します。 |
| **固定されたソース** | Run は変更可能な `#42` を決して信用しません。CLI は厳格なマニフェストを決定論的に解析し、各チャプターをダイジェストで固定して、上限のあるカタログとドメイン分離されたリビジョンだけを保存します。 |
| **明示的な 6 つの Capability** | 10 個のアダプターが、`run`、`clarify`、`propose`、`decompose`、`implement`、および読み取り専用の `review` だけを生成します。常駐フック、グローバルルーター、暗黙的な起動はありません。 |
| **7 つの公開 Command** | `install`、`uninstall`、`list`、`doctor`、`run`、`status`、`stop` に加え、バージョン管理された非公開の `_machine` 操作があります。マシンプロトコルは安定した契約です。 |
| **誠実な復旧** | `.git/slipway/runs/` 配下の追記専用ジャーナルが復旧の正規な根拠です。Run は停止、再開、リプレイが可能です。`ended` が意味するのは、キューが空であることだけです。 |
| **完了を認定しない** | テスト失敗、未実行のテスト、Review の指摘、dirty なワークツリー、Issue の状態は、いずれも進行を妨げません。Slipway は事実を報告しますが、「完了」、デプロイ可能、リリース可能であることを認定しません。 |
| **信頼しない Issue 内容** | Issue の本文、コメント、ラベルはデータであり、指示ではありません。Issue 内のプロンプトインジェクションや認証情報の要求には、ホストに対する権限がありません。 |
| **破壊的操作に対する厳密な権限** | 破壊的な作業には、1 回限りで対象範囲に拘束された構造化 grant が必要です。自然言語の「yes」が権限を与えることはありません。また、信頼されたホストは内容を証言するだけであり、人間による承認を暗号学的に証明するものではありません。 |

## 設計思想

Slipway は、拘束力を持つ 3 つの原則に従います。

- **プロセスを所有するのはユーザーです。** Slipway は明示的に起動された場合にのみ
  開始します。ユーザーは理由を示すことなく、任意の Action をスキップ、停止、再開、
  または自ら引き継ぐことができます。通常の実装で承認を何度も求めることはありません。
  人間による真の判断、ソースの修正、環境障害、破壊的な作業が必要な場合には一時停止します。
- **質問より先に事実を確認します。** ホストは質問する前に、リポジトリ、Git の状態、
  規約を調査します。コードから解決できる判断をユーザーに委ねることはありません。
  人間による真の判断が必要な事項だけを、推奨案、理由、代替案とともに 1 つずつ尋ねます。
- **事実に忠実に報告します。** Slipway は、観測された変更、実行した正確な作業、終了
  コード、指摘事項、既知の問題、不確実性を報告します。実行していない作業を実行済みと
  主張することはなく、空のキューを、正しさ、デリバリー完了、リリース準備完了の認定に
  すり替えることもありません。

Requirements は一時的なデリバリー契約であり、システムの永続的なモデルではありません。
Objective が存在するのは、1 つの成果に複数の独立した Change がどうしても必要な場合だけです。
各 Change は自己完結しており、親や通常のディスカッションコメントから実行時の Requirements を
継承しません。本文で最初に現れる厳密なマーカーが Level の正規な根拠です。ラベル、タイトル、
`ready-for-agent`、Project フィールド、テスト、指摘事項は警告としてのみ扱われる投影情報であり、
マーカーが有効な Run を妨げることはありません。

完全なモデルについては、[製品の正規な根拠](docs/ja/reference/product-overview.md)、
[Issue ワークフロー](docs/ja/reference/issue-workflow.md)、
[アーキテクチャ](docs/ja/explanation/architecture.md)を参照してください。

## Quick Start

公式リリースに裏付けられた配布チャネルから Slipway をインストールし、実際に使用する
AI ツール向けのホストアダプターを生成します。

| プラットフォーム | 推奨方法 |
| --- | --- |
| macOS | `brew install --cask signalridge/tap/slipway` |
| Windows | `scoop bucket add signalridge https://github.com/signalridge/scoop-bucket`<br>`scoop install slipway` |
| Linux | [インストール](docs/ja/installation.md#linux-package)に記載された `.deb`、`.rpm`、`.apk`、`tar.gz`、AUR、またはコンテナイメージを利用します。 |
| Go フォールバック | `go install github.com/signalridge/slipway@latest` |

```bash
slipway --version
cd your-repository
slipway install --tool claude
```

対応するツール ID は `claude`、`codex`、`copilot`、`cursor`、`kilo`、`kiro`、
`opencode`、`pi`、`qwen`、`windsurf` です。`--tool` を繰り返し指定するか、カンマ区切りの
値、または `--tool all` を使用します。Kiro は初回インストール時に `--surface ide|cli` が
必要です。

### アドホックなエスケープハッチ

微小、機微、緊急、オフラインの作業、または単に Issue を使いたくない作業は、ソースなしで
開始できます。

```bash
slipway run --budget 8 --json --root "$PWD" -- "add a CSV export to reports"
```

### Issue に紐づく Run

信頼されたホストは、厳格なマニフェストで指定された 1 つの Source Bundle を一度だけ取得し、
一時的な raw envelope を CLI に渡します。

```bash
slipway run --budget 8 --json --root "$PWD" \
  --source-file /safe/temp/change-envelope.json -- "implement the bounded Change"
```

CLI は Issue 本文のマニフェストと、そこから厳密に参照されたコメントを検証し、各チャプターを
ダイジェストで固定して、上限のあるカタログだけを保存します。ローカルの material 読み取りや
再開に、一時ファイルや GitHub が必要になることはありません。

操作モデルはこれだけです。AI ツールのセッションで `slipway-<name>` Capability を明示的に
起動すると、ホストは一度に 1 つの Action を実行します。一時停止するのは、人間による真の判断、
ソースの修正、環境障害、または破壊的操作の確認が必要な場合だけです。制御に理由は不要です。
「skip this」「stop」「take over」「do X first」は、いずれも文字どおりに尊重されます。

<details>
<summary><strong>CLI が保持するもの、保持しないもの</strong></summary>

CLI は、次のものを**保持・実行しません**。

- GitHub トークンを保持すること、またはモデルプロバイダーを呼び出すこと。
- GitHub/Project プロバイダーやトラッカーのランタイムを実装すること。
- 通常のディスカッションコメントを走査すること、またはコメント順を正規な根拠として扱うこと。
- ワークツリーを作成、切り替え、関連付け、削除すること。
- Issue 作成の exactly-once 性や、本文の compare-and-swap を主張すること。
- ジャーナルに機密情報が一切含まれないと保証すること。

CLI は、次のことを**行います**。

- ホストが証言した raw envelope を厳格に検証し、未知のフィールド、重複した JSON キー、
  不正な UTF-8、BOM、末尾の余分なデータを拒否する。
- マニフェストを決定論的に解析し、チャプターをドメイン分離されたダイジェストで固定して、
  非公開のコンテンツアドレス型 material ストアに保存する。
- 構造化されたローカルリーダーとともに、上限のあるバージョン管理された Action を一度に
  1 つ返す。
- 追記専用ジャーナルを復旧の正規な根拠として保持し、置換可能な投影も保持する。
- 7 つの公開 Command と、バージョン管理された非公開の `_machine` 操作を提供する。

</details>

## 仕組み

| ステージ | Slipway が求めること |
| --- | --- |
| `orient` | 質問する前に、リポジトリの事実、Git の状態、規約を調査します。次の Action を提案するか、人間による真の判断が必要なら一時停止します。 |
| `clarify` | 依存関係に従い、人間による判断を 1 つずつ、推奨案とトレードオフとともに確認します。ステートレスであり、ファイルを書き込まず、Issue を作成せず、wrap-up の指示があれば停止します。依頼が完全なら質問は 0 件です。 |
| `implement` | 現在の Action が許可した、範囲を限定された変更を実行します。正確なコマンド、終了コード、変更ファイル、既知の問題、不確実性を報告します。 |
| `review` | Intent（固定された Requirements を満たしているか）と Quality を読み取り専用で確認します。コードを編集せず、`needs_input` にせず、修正ループを開始しません。 |
| `summarize` | 指摘事項と実行内容を集約します。受理後、Run は `ended` になります。 |

Run は、バージョン管理された Action を 1 つずつ進めます。ルーティングは差分を最優先します。
CLI が不変の Run 開始時 Git フィンガープリントからの変更を観測し、Review が有効であれば、
ホストの報告内容にかかわらず Review に進みます。Review は常に Summary に、Summary は `ended` に
進みます。失敗した作業と Review の指摘は**データ**であって、ゲートではありません。自動修正
ループを作ることなく Summary に引き継がれます。

## 6 つの Capability、7 つの Command

```text
アダプターが生成:      run  clarify  propose  decompose  implement  review
公開 CLI Command:      install  uninstall  list  doctor  run  status  stop
```

すべての Capability は明示的な起動を必要とします。Clarify は、出典を明示した
[Matt Pocock の `grill-me` / `grilling`](https://github.com/mattpocock/skills)の規律に従います。
事実を調査し、依存する判断を推奨案とともに 1 つずつ確認し、共有理解が変わった場合は確認を取り、
ステートレスを保ち、wrap-up の指示があれば直ちに停止します。Review は読み取り専用であり、
修正も再レビューのループも行いません。

非公開でバージョン管理された `_machine submit/answer/skip/resume/material` 操作が
オートパイロットのループを駆動します。詳細は
[マシンプロトコル](docs/ja/reference/machine-protocol.md)に記載されています。

<details>
<summary><strong>AI ツールアダプター</strong></summary>

`slipway install --tool <id>` でホストツール用のサーフェスを生成し、
`slipway install --refresh` で管理対象ファイルを更新します。生成ファイルの所有権は追跡されるため、
隣接するユーザー所有のカスタマイズを削除せずに、Slipway 所有のファイルだけを置き換えられます。

| ツール | ネイティブサーフェス | 明示的な起動方法 |
| --- | --- | --- |
| `claude` | `.claude/skills` | `slipway-<name>` skill を起動する |
| `codex` | `.codex/skills`（skill ごとに `agents/openai.yaml`） | `$slipway-<name>` |
| `copilot` | `.github/copilot/agents/*.agent.md` | `slipway-<name>` カスタムエージェントを選択する |
| `cursor` | `.cursor/skills` | `slipway-<name>` skill を起動する |
| `kilo` | `.kilo/commands/*.md` | `/slipway-<name>` |
| `kiro` IDE | `.kiro/steering/*.md` | `#slipway-<name>` を手動で含める |
| `kiro` CLI | `.kiro/agents/*.json` | `kiro-cli chat --agent slipway-<name>` |
| `opencode` | `.opencode/commands/*.md` | `/slipway-<name>` |
| `pi` | `.pi/skills` | `/skill:slipway-<name>` |
| `qwen` | `.qwen/skills` | `slipway-<name>` skill を起動する |
| `windsurf` | `.windsurf/workflows/*.md` | `/slipway-<name>` |

アダプターは、常駐セッションフック、prompt-submit フック、ランチャー、グローバルルーター、
独立した技術検証 Capability をインストールしません。ホスト設定はアダプターの所有範囲外であり、
変更されることはありません。

正確なサーフェス、所有権ルール、Kiro の `--surface` の扱いについては、
[ホストアダプター](docs/ja/reference/adapters.md)と
[インストール](docs/ja/installation.md)を参照してください。

</details>

## 他のツールとの比較

多くの AI ワークフローシステムは、Spec ファイルとフェーズごとのプロンプトで作業を構造化します。
Slipway がより狭く焦点を当てるのは、**範囲が限定され、誠実に扱われる権限**です。エージェントの
要約を信用するのではなく、リポジトリから事実を再計算し、ソースをダイジェストで固定する決定論的な
CLI がライフサイクルの状態を保持します。

<details>
<summary><strong>周辺ツールとトレードオフ</strong></summary>

| ツール | モデル | 完了判定の強制 |
| --- | --- | --- |
| [Spec Kit](https://github.com/github/spec-kit) | Spec ファイル + スラッシュコマンド | 助言的なチェックリストとフェーズプロンプト。 |
| [OpenSpec](https://github.com/Fission-AI/OpenSpec) | Spec 駆動ワークフロー | 柔軟な Spec ワークフロー。検証は任意。 |
| [GSD Core](https://github.com/open-gsd/gsd-core) | ランタイムサーフェス + フェーズコマンド | 強力なフェーズループ。最終的な証明は成果物を介する。 |
| [superpowers](https://github.com/obra/superpowers) | 自動発火する skill | 強力なエージェント規律。ルールはモデルのコンテキスト内に置かれる。 |
| **Slipway** | 明示的な Capability + 固定されたソース | 範囲が限定され復旧可能な Run、誠実な報告、完了ゲートなし。 |

Slipway は、幅広さよりも誠実さと制御を優先します。1 行の編集なら完全な Spec フレームワークより
軽量です（アドホックを使うだけです）。一方で、復旧可能で、ソースが固定され、差分が観測された
履歴が必要な場合には、はるかに厳格です。履歴を黙って書き換えたり、Issue に形式的な承認印を
押したりすることはありません。

</details>

## 復旧、プライバシー、エビデンス

復旧の正規な根拠は `.git/slipway/runs/<run-id>/` にあります。

```text
.git/slipway/runs/<run-id>/
├── journal.jsonl   追記専用の状態遷移に関する正規な記録
├── run.json        置換可能な投影
├── run.lock        ジャーナルの変更を直列化
└── materials/      コンテンツアドレス型のチャプター blob（0600）
```

ジャーナルには、受理された Requirements、目標、ユーザーの回答、事実に即したコマンドの要約が
含まれる場合があります。Slipway はデータを最小化し、認識した認証情報を秘匿化しますが、
**ジャーナルに機密情報が一切含まれないとは保証しません**。Run ディレクトリはローカルの非公開
データとして扱ってください。Unix のファイルモードと Windows の現ユーザー向け ACL による保護には、
root、バックアップ、マルウェア、継承 ACL、同一アカウントからのアクセスという限界があります。

Run ディレクトリを削除して失われるのは復旧機能だけです。安全な消去、バックアップからの削除、
鍵の破棄にはなりません。[Run とプライバシー](docs/ja/explanation/runs-and-privacy.md)、
[Windows の動作](docs/ja/reference/windows-rendering-and-durability.md)、および事実に忠実な
[受け入れエビデンスマトリクス](docs/ja/reference/acceptance-evidence.md)を参照してください。

## ドキュメント

ドキュメントはタスク別に整理されています。

- [はじめに](docs/ja/start-here.md) — インストールから 1 回の Run までの最短手順。
- [製品の正規な根拠](docs/ja/reference/product-overview.md) — 4 軸モデル、6 つの Capability、
  7 つの Command。
- [Issue ワークフロー](docs/ja/reference/issue-workflow.md) — Objective/Change マーカー、
  ラベル、自己完結性、GitHub の制限、公開時の reconciliation。
- [インストール](docs/ja/installation.md) — プラットフォーム別の手順とアダプターコマンド。
- [Command](docs/ja/reference/commands.md) — 公開 Command と JSON サーフェス。
- [マシンプロトコル](docs/ja/reference/machine-protocol.md) — バージョン管理された Action /
  Outcome 契約と非公開操作。
- [ホストアダプター](docs/ja/reference/adapters.md) — 10 個のホスト、6 つの Capability、
  所有権の安全性。
- [アーキテクチャ](docs/ja/explanation/architecture.md) — パッケージ構成と依存関係の方向。
- [Run とプライバシー](docs/ja/explanation/runs-and-privacy.md) — ジャーナルの内容、
  保持期間、プライバシーに関する約束。
- [Windows のレンダリングと耐久性](docs/ja/reference/windows-rendering-and-durability.md)
  — argv のレンダリングとクラッシュ耐久性。
- [受け入れエビデンス](docs/ja/reference/acceptance-evidence.md) — エビデンスの種類と
  35 シナリオのマトリクス。

## 検証

開発中に役立つローカルチェックは次のとおりです。

```bash
gofmt -w .
go vet ./...
go run ./internal/testlint/cmd/testlint ./...
go test -timeout=20m ./... -count=1
go test -timeout=20m ./... -race -count=1
go build ./...
git diff --check
```

CI では、Markdown/YAML/Action の lint、Go の lint、Slipway testlint、各プラットフォームでの
Go テスト、race テスト、ビルドチェック、Windows ネイティブの cmd/PowerShell スイート、
アダプターのシェル受け入れテスト、ドキュメントビルドを実行します。

## コントリビューション

コントリビューションには fork-and-pull-request ワークフローを使用します。手順については
[CONTRIBUTING.md](CONTRIBUTING.md)、開発の詳細については
[開発リファレンス](docs/ja/contributing.md)を参照してください。

## ライセンス

Slipway は [BSD 3-Clause License](LICENSE) の下で配布されます。
