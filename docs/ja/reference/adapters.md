# ホストアダプター

各ホストには `slipway-run`、`slipway-clarify`、`slipway-propose`、`slipway-decompose`、`slipway-implement`、`slipway-review` の六 capability だけを生成します。Clarify は stateless です。

ネイティブ surface と呼び出し方は次の通りです。Claude、Cursor、Qwen は各 `skills` directory から `slipway-<name>` skill を明示的に呼び出し、Codex は `.codex/skills` の `$slipway-<name>`、Pi は `.pi/skills` の `/skill:slipway-<name>` を使います。Copilot は `.github/copilot/agents` の custom agent を agent picker で選択します。Kilo、OpenCode、Windsurf はそれぞれ `.kilo/commands`、`.opencode/commands`、`.windsurf/workflows` の `/slipway-<name>` を使います。Kiro IDE は `.kiro/steering` の `#slipway-<name>` を手動 include し、Kiro CLI は `kiro-cli chat --agent slipway-<name>` で `.kiro/agents` の agent を選択します。

Command、workflow、steering、agent surface は各ホストの `slipway/capabilities` にある canonical body を参照します。Copilot auto-detection は引き続き `.github/copilot`、`.github/prompts`、`.github/skills` のいずれかを認識します。Kiro の初回 install では `--surface ide|cli` が必須で、選択は ownership manifest に記録され refresh/uninstall でも維持されます。

ambient session hook、prompt-submit hook、launcher、global router、独立した技術検査 capability は生成しません。ホスト settings は adapter ownership の対象外であり、install、refresh、uninstall は一切変更しません。すべての生成 skill は共通の untrusted Issue、trusted attester、confirmed publication、exact destructive authorization boundary を持ちます。Clarify だけが一つの decision interview reference を含み、Matt Pocock の MIT `grill-me` 由来の attribution を保持します。

Clarify は Matt Pocock `grill-me`/`grilling` の fact investigation、dependency order、one question+recommendation、changed shared-understanding confirmation、stateless、immediate wrap-up を保ちます。暗黙の clarification-document capability はありません。
Codex の各 capability には管理対象の `agents/openai.yaml` も配置し、`allow_implicit_invocation: false` を設定します。Codex は汎用 frontmatter の同等設定を解釈しないため、このポリシーによりユーザーが明示的に呼び出すまで Slipway capability は暗黙選択されません。

Mutation を認可できるのは version 2 ownership manifest だけです。他の version はすべて unreadable とし、install、refresh、uninstall はファイルを変更する前に失敗します。Read-only の `list` はそのホストだけを未インストール advisory に降格し、他のホストの報告を続けます。Version 2 は path と SHA-256 を記録します。Refresh/uninstall はハッシュ一致ファイルだけを扱い、ユーザー変更ファイルは保持して管理対象から外します。
初回導入は新規作成ファイルだけを管理対象にします。current manifest が存在した後の更新には `slipway install --refresh` が必要です。marker だけで current manifest がない状態は ownership を確立しません。install、refresh、uninstall は surface を変更せず、migration や推論をせずに current ownership の欠落を中立的に報告します。

Install/uninstall report は通常の ownership preservation と transaction recovery を分離します。`transaction_outcome` は `committed|rolled_back|not_committed|ambiguous` で、planned `written`/`removed` を残すのは committed の場合だけです。Concurrent object や quarantine path は `recovery_artifacts` にだけ入り、`preserved` と混同しません。Error も同じ report を返し、blind-retry command は提示しません。

`.adapter-generated` sentinel は health evidence であって ownership authority ではありません。Missing なら `install --refresh` で再作成できます。Modified sentinel は user content として refresh/uninstall が保持し、doctor は必要なら確認後に手動削除してから refresh するよう案内します。Refresh が user edit を上書きできるとは説明しません。

## Publication と privacy boundary

Propose/decompose は `gh` を検出し、2.94.0+ は first-class relation、それ以外は official REST API または `environment_unavailable` を使います。Exact Level/Kind labels、same-`github.com` transfer identity refetch、100/50 limits、approved operation/item UUID markers、body files、expected revisions、readback、0/1/multiple match の `created|matched|failed|ambiguous` reconciliation を要求し、blind retry しません。

全 capability は accepted Requirements、goal、answer、command summary が機微であり得ると警告します。Public Issue に private switch はありません。認識した credential value は command identity を残して redact し、token、raw comments、env dump、transcript、hidden reasoning を収集しません。[Issue workflow](issue-workflow.md)と[privacy](../explanation/runs-and-privacy.md)を参照してください。
