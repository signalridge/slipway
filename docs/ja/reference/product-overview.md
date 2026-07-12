# 製品概要（非規範）

> **このページは non-normative summary です。** 実装 authority は完全な[中国語製品契約](../../zh/reference/product-contract.md)と versioned [machine protocol schema](../../reference/machine-protocol.schema.json)です。このページは第2の仕様ではありません。

Slipway は、ユーザーが明示的に起動する issue-first かつ issue-gated ではない AI coding 用 soft autopilot です。

```text
Objective Issue（任意の planning parent、実行不可）
  └─ self-contained Change Issue（唯一の issue-backed source）
       └─ Run（revision を固定した中断可能な1回の試行）
```

Requirements は一時的な delivery contract です。Objective は複数の独立 Change が必要な結果だけをまとめ、decompose は適用可能な要件と制約を各 child に物化します。Change は独立して deliver/verify/rollback でき、parent や comments を runtime inheritance しません。GitHub が使えない、機微、微小、または Issue を作らない選択では ad-hoc Run が使えます。

Level、Kind、5つの Requirements section、Status は直交します。本文先頭の厳密な marker が Level authority で、title、label、`ready-for-agent`、Project、test、finding、Issue status は Run gate ではありません。

10 adapter は run/clarify/propose/decompose/implement/review の6つだけを明示 capability として生成し、CLI は install/uninstall/list/doctor/run/status/stop の7 command だけを公開します。Review は read-only で repair loop を作りません。`ended` は自動 queue が空であることだけを示します。

Host は GitHub fetch の trusted attester ですが Issue 内容は untrusted data です。Source revision は固定され、amendment は current candidate の明示選択、destructive work は exact-scope one-shot grant を必要とします。GitHub publication は exactly-once ではなく approved UUID markers と reconciliation を使います。

Git common directory の append-only journal は復旧 authority であり、accepted Requirements、goal、answer、command summary を含む可能性があります。認識可能な credential は command identity を残して redact しますが、secret-free は保証しません。run directory の削除は復旧能力を失わせるだけで secure erase や backup purge ではありません。

[Issue workflow](issue-workflow.md)、[commands](commands.md)、[machine protocol](machine-protocol.md)、[Windows](windows-rendering-and-durability.md)、[acceptance evidence](acceptance-evidence.md)へ進んでください。
