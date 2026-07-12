# ホストアダプター

各ホストには `slipway-run`、`slipway-clarify`、`slipway-propose`、`slipway-decompose`、`slipway-implement`、`slipway-review` の六 capability だけを生成します。Clarify は stateless です。

配置先は `.claude/skills`、`.codex/skills`、`.github/skills`、`.cursor/skills`、`.kilocode/skills`、`.kiro/skills`、`.opencode/skills`、`.pi/skills`、`.qwen/skills`、`.windsurf/skills` です。

ambient session hook、prompt-submit hook、launcher、global router、独立した技術検査 capability は生成しません。ホスト settings は adapter ownership の対象外であり、install、refresh、uninstall は一切変更しません。すべての生成 skill は共通の untrusted Issue、trusted attester、confirmed publication、exact destructive authorization boundary を持ちます。Clarify だけが一つの decision interview reference を含み、Matt Pocock の MIT `grill-me` 由来の attribution を保持します。

Clarify は Matt Pocock `grill-me`/`grilling` の fact investigation、dependency order、one question+recommendation、changed shared-understanding confirmation、stateless、immediate wrap-up を保ちます。暗黙の clarification-document capability はありません。
Codex の各 capability には管理対象の `agents/openai.yaml` も配置し、`allow_implicit_invocation: false` を設定します。Codex は汎用 frontmatter の同等設定を解釈しないため、このポリシーによりユーザーが明示的に呼び出すまで Slipway capability は暗黙選択されません。

version 2 ownership manifest だけを受け付けます。他の version はすべて unreadable とし、install、refresh、uninstall、list を認可しません。Version 2 は path と SHA-256 を記録します。Refresh/uninstall はハッシュ一致ファイルだけを扱い、ユーザー変更ファイルは保持して管理対象から外します。
初回導入は新規作成ファイルだけを管理対象にします。current manifest が存在した後の更新には `slipway install --refresh` が必要です。marker だけで current manifest がない状態は ownership を確立しません。install、refresh、uninstall は surface を変更せず、migration や推論をせずに current ownership の欠落を中立的に報告します。

## Publication と privacy boundary

Propose/decompose は `gh` を検出し、2.94.0+ は first-class relation、それ以外は official REST API または `environment_unavailable` を使います。Exact Level/Kind labels、same-`github.com` transfer identity refetch、100/50 limits、approved operation/item UUID markers、body files、expected revisions、readback、0/1/multiple match の `created|matched|failed|ambiguous` reconciliation を要求し、blind retry しません。

全 capability は accepted Requirements、goal、answer、command summary が機微であり得ると警告します。Public Issue に private switch はありません。認識した credential value は command identity を残して redact し、token、raw comments、env dump、transcript、hidden reasoning を収集しません。[Issue workflow](issue-workflow.md)と[privacy](../explanation/runs-and-privacy.md)を参照してください。
