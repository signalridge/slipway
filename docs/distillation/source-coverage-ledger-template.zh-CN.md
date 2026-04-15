# Source Coverage Ledger 模板

当 strengthening PR 需要把上游 references、scripts 或 frontmatter 契约映射为
Slipway 产物时，在 PR notes 中从这份模板起稿。某一节只有在该 PR 完全没有对应
内容时才可以删除。

## 1. Reference Coverage Ledger

| skill id | upstream source/ref | disposition (`mapped` / `collapsed` / `deferred`) | rendered target | selected source bytes | rendered reference bytes | reason |
|----------|----------------------|---------------------------------------------------|-----------------|-----------------------|--------------------------|--------|
| `<skill-id>` | `<repo/path/or section family>` | `mapped` | `references/<file>.md` | `<n>` | `<n>` | `mapped 行可写 n/a` |

规则：

- 一行只对应一个上游 reference 文件，或一个显式命名的 source-section family。
- `rendered target` 可以是 `references/<file>.md`、`CHECKLIST.tmpl`、
  `PROSE.tmpl`、`VERDICT.tmpl` 或 `n/a`。
- 任何 `collapsed` 或 `deferred` 行都必须写明具体原因。

## 2. Curator Additions

| skill id | added artifact | why no upstream 1:1 source exists | provenance note |
|----------|----------------|-----------------------------------|-----------------|
| `<skill-id>` | `<artifact>` | `<reason>` | `<source pointer or synthesized note>` |

## 3. Script Selection / Defer Ledger

| skill id | script candidate | outcome (`shipped` / `deferred` / `not-in-scope`) | reason | first expected caller |
|----------|------------------|----------------------------------------------------|--------|-----------------------|
| `<skill-id>` | `scripts/<name>` | `shipped` | `<reason>` | `<repo/workflow or no-caller-yet>` |

## 4. PR-1 到 PR-4a 的交接表

| skill id | sample frontmatter record | resolves-to file | expected runtime hydrate key | reviewer sign-off |
|----------|---------------------------|------------------|------------------------------|-------------------|
| `<skill-id>` | ``- name: <reference-basename>`` | `references/<file>.md` | `<skill-id>/<reference-basename>` | `<initials/date>` |

说明：

- `sample frontmatter record` 必须保持与 `hydrate_references:` 相同的 typed
  record 形状。
- `resolves-to file` 应写成磁盘上的实际文件路径，也就是
  `TestHydrateReferencesResolveToFiles` 预期能通过的目标。
- `reviewer sign-off` 应指向明确的验收证据，例如 PR comment、checklist item
  或其它可追踪记录。
