# はじめに

Current Slipway build を使い、空の checkout から1回の user-controlled Run を開始します。

## 1. CLI generation を確認する

この documentation が対象とする interface には7つの user コマンド と machine protocol があります。

```bash
./slipway --help
```

Output に `install`、`uninstall`、`list`、`doctor`、`run`、`status`、`stop` が必要です。表示されない場合は旧 release です。Current checkout を build するか、より新しい compatible tag を選んでください。詳細は[インストール](installation.md)を参照してください。

## 2. Host アダプターを1つ install する

AI ホストが作業する Git worktree 内で実行します。

```bash
./slipway install --tool claude
./slipway doctor
```

`claude` は `codex`、`copilot`、`cursor`、`kilo`、`opencode`、`pi`、`qwen`、`windsurf` に置き換えられます。Kiro は初回に surface が必要です。

```bash
./slipway install --tool kiro --surface ide   # または: --surface cli
```

`install` は host-local capability file だけを書き、hash を記録します。グローバルな ホスト 設定や ambient hook は変更しません。Generated path は[ホストアダプター](reference/adapters.md)を参照してください。

## 3. Run を明示的に開始する

AIコーディングツール で、生成された `slipway-run` capability を明示的に呼び出して1つの task を与えます。

> reports コマンドに CSV export を追加し、test を追加する。

ホストは CLI に Action を要求し、それを実行して structured Outcome を報告します。Run が pause または summary に達するまで繰り返します。使用される `protocol` 操作は公開され documented ですが、手動で操作する必要はありません。各 response が正確な次の コマンドを既に含んでいます。

CLI を直接 integration する ホストは、同じ ad-hoc Run を次のように開始できます。

```bash
./slipway run --json -- "reports コマンドに CSV export を追加する"
```

この コマンドは最初の `orient` Action を返します。CLI 自体は code を変更しません。

## 4. Source を選ぶ

| Source | 選ぶ場面 |
| --- | --- |
| Ad hoc | Task が小さい、機微、緊急、offline、または Issue が不要な場合。 |
| GitHub Change Issue | Durable で review 可能な revision-pinned requirements source が必要な場合。 |

Issue-backed Run では、生成された `slipway-run` capability に GitHub Change Issue を渡します。Host が Issue を fetch し、temporary ソースエンベロープ を作成して CLI に渡します。Host integration を実装している場合を除き、envelope を手書きしないでください。

Host を離れずに rough idea を work-item draft まで進めるには、生成された `slipway-workflow` capability を呼び出します。1 回の呼び出しで自律的に調査し、genuine な human decision を 1 つずつ問い、self-contained な Change または planning Objective を合成して次の explicit capability を示します。User-only external skill の呼び出し、Issue publish、Run start は行いません。[アイデアから Run までのワークフロー](guides/idea-to-run-workflow.md)を参照してください。

Objective は複数の Change をまとめられますが、Run は開始できません。Managed Issue を publish する前に [GitHub Issue ワークフロー](guides/github-issues.md)を読んでください。

## 5. 制御を保つ

Run が報告する pause reason は次の4つのいずれかです。

- 人間が決める必要のある decision——Issue source の変更や unavailable もここに含まれ、独立した reason ではなく decision として pause します；
- Environment dependency の unavailable；
- Action budget の exhaustion；
- Exact destructive scope の confirmation。

Generated ホストが選べる response を示します。ユーザーは理由を説明せず skip、stop、reorder、take over できます。通常の implementation で繰り返し authorization を求めません。

Inspection command：

```bash
./slipway status
./slipway status <run-id> --json
./slipway stop <run-id>
```

`stop` は recovery data を保存します。Ended は Slipway に自動 Action が残っていないことだけを示します。Test、Review finding、リポジトリ policy、merge approval、release decision は独立しています。

## 6. 保存内容を理解する

Run data は `<git-common-dir>/slipway/runs/` にあり、goal、accepted requirements、user answer、Outcome、コマンド summary を含む場合があります。Slipway は token、environment dump、unrelated file、full conversation、hidden reasoning を意図的には収集しませんが、ジャーナル に secret がないとは保証できません。

機微な content を扱う前に [Run、復旧、プライバシー](guides/runs-and-recovery.md)を読んでください。

## 次に読む

- [コア概念](explanation/concepts.md)
- [コマンドリファレンス](reference/commands.md)
- [ホストアダプター](reference/adapters.md)
- [マシンプロトコル](reference/machine-protocol.md)
