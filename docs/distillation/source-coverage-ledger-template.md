# Source Coverage Ledger Template

Use this template in PR notes when a strengthening PR maps upstream
references, scripts, or frontmatter contracts into Slipway artifacts.
Delete a section only when the PR has nothing in that category.

## 1. Reference Coverage Ledger

| skill id | upstream source/ref | disposition (`mapped` / `collapsed` / `deferred`) | rendered target | selected source bytes | rendered reference bytes | reason |
|----------|----------------------|---------------------------------------------------|-----------------|-----------------------|--------------------------|--------|
| `<skill-id>` | `<repo/path/or section family>` | `mapped` | `references/<file>.md` | `<n>` | `<n>` | `n/a for mapped rows` |

Rules:

- Use one row per upstream reference file or explicitly named source-section
  family.
- `rendered target` may be `references/<file>.md`, `CHECKLIST.tmpl`,
  `PROSE.tmpl`, `VERDICT.tmpl`, or `n/a`.
- Every `collapsed` or `deferred` row needs a concrete reason.

## 2. Curator Additions

| skill id | added artifact | why no upstream 1:1 source exists | provenance note |
|----------|----------------|-----------------------------------|-----------------|
| `<skill-id>` | `<artifact>` | `<reason>` | `<source pointer or synthesized note>` |

## 3. Script Selection / Defer Ledger

| skill id | script candidate | outcome (`shipped` / `deferred` / `not-in-scope`) | reason | first expected caller |
|----------|------------------|----------------------------------------------------|--------|-----------------------|
| `<skill-id>` | `scripts/<name>` | `shipped` | `<reason>` | `<repo/workflow or no-caller-yet>` |

## 4. PR-1 to PR-4a Handoff Table

| skill id | sample frontmatter record | resolves-to file | expected runtime hydrate key | reviewer sign-off |
|----------|---------------------------|------------------|------------------------------|-------------------|
| `<skill-id>` | ``- name: <reference-basename>`` | `references/<file>.md` | `<skill-id>/<reference-basename>` | `<initials/date>` |

Notes:

- Keep `sample frontmatter record` in the same typed shape used by
  `hydrate_references:`.
- `resolves-to file` should be the on-disk file path that
  `TestHydrateReferencesResolveToFiles` is expected to validate.
- `reviewer sign-off` should point to the PR comment, checklist item, or other
  explicit acceptance evidence.
