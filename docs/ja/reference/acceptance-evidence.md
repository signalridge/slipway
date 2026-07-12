# Acceptance と evidence（非規範）

> このガイドは evidence collection の説明であり release verdict や CLI gate ではありません。35 scenario は[中国語製品契約](../../zh/reference/product-contract.md)、current artifact/gap は[実行可能 matrix](../../../tests/acceptance/README.md)が示します。

Label は補完関係です。C=deterministic Go contract/property/race/static template、S=built binary を呼ぶ Shell、G=isolated live GitHub.com user-owned fixture、H=sanitized Claude/Codex/Pi transcript+evaluator notes、W=native Windows cmd.exe+PowerShell、R=docs/website/package/release validation。

C は host autonomy H を証明せず、local fake endpoint や deterministic publication fault harness は reproducible H/G-adjacent で live G ではありません。Windows cross-build は native W ではありません。Missing evidence は `not collected`/`external` と記録し、Run routing、Issue status、Review、delivery、CLI exit を制御しません。

Local harness は credential なしで timeout-after-success、partial relation failure、duplicate markers、index delay、0/1/multiple reconciliation を再現します。Live G は protected test account/repository を使い、fork PR に secret を渡しません。Transcript は `tests/acceptance/transcripts/` の sanitized format を使い、raw conversation や架空の model run を保存しません。

CI matrix は `windows-latest` で `slipway.exe` を build し、native PowerShell と `cmd.exe` asset の両方を実行します。Workflow は W collector であって事前の evidence ではありません。同じ completed run で両方が成功した場合だけ W を記録し、この local change は実行済みとは主張しません。R gate は `tests/acceptance/` の stdlib link/release-artifact checker で built-site route、archive LICENSE bytes、Scoop、AUR、package path を検査します。

Executable acceptance asset は `tests/acceptance/` だけに置き、`scripts/` に複製しません。Command と current status は matrix を参照してください。
