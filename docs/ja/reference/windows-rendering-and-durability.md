# Windows rendering と durability（非規範）

> このページは non-normative です。[中国語製品契約](../../zh/reference/product-contract.md)と [machine schema](../../reference/machine-protocol.schema.json) が authority です。

## Structured argv が authority

Recovery/pause は `next.operation`、workspace identity、typed variants を返します。各 variant の完全な `base_argv` に、schema 順で input flag と exact unquoted value を1 argv element として追加します。Display prose から command を再構築せず、`<answer>`/`<file>` placeholder を作りません。

POSIX、`cmd.exe`、PowerShell の rendering は display/copy 専用で journal に保存されず、machine operation を変えません。Root、Issue URL、source/outcome file、answer、candidate recovery の space、quote、Unicode、CR/LF、`%`、`!`、`&`、`^` をそのまま保ちます。cmd expansion を避けるため PowerShell UTF-16LE `EncodedCommand` または同等の safe argv path を利用できます。`cmd.exe /v:on` と PowerShell の native process で実 argv を捕捉する必要があり、Linux cross-build は W evidence ではありません。

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File tests\acceptance\windows\native-powershell.ps1 -SlipwayExe C:\path\slipway.exe
cmd.exe /d /v:on /c tests\acceptance\windows\native-cmd.cmd C:\path\slipway.exe
```

CI matrix は `slipway.exe` を build して両方の native entry point を実行します。Scripts は doctor、initial Orient、issue source、Outcome file/stdin（対応する shell）、decision answer、ad-hoc resume、current-candidate keep/adopt、special argv を検査します。Workflow wiring は W collector にすぎず、同じ completed `windows-latest` run で両方が成功するまでは not collected です。この local change はその run を取得済みとは主張しません。

## Symbolic-link transaction の境界

Windows file transaction は anchored reparse-point handle で symbolic link を検査しますが、file link と directory link の再作成には既存 object の検査・移動には不要だった権限が必要な場合があります。そのため Slipway はすべての pre-existing Windows symbolic link を最初の transaction mutation 前に typed error で拒否し、link を移動した後で symlink creation privilege に rollback を依存させません。Skip できない policy test は link を作らずにこの fail-closed 判断を証明し、native fixture は runner が作成可能な場合の補足です。

## Crash durability と ACL

Windows は journal/lock/projection file を flush しますが Unix と同じ run-directory fsync は提供しません。Doctor は `level:"file_fsync_only"`、`directory_sync:false`、`limitation:"directory_fsync_unsupported"` を返し、新規/rename directory entry に同等の crash durability を主張しません。

Run dir は current-user ACL を目標としますが inherited ACL、administrator、backup agent、malware、same-account process は絶対的な保護範囲外です。機微な repository では ACL、retention、backup を確認してください。run directory の削除は復旧能力だけを除き、secure erase、backup purge、key destruction ではありません。
