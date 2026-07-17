# ADR-0001: Use manifest-addressed source bundles

## Status

Accepted — 2026-07-13. This record captures the rationale for source bundle version 2. The JSON schemas and runtime validation define the current machine surface; this ADR explains why that design was selected.

## Context

The version 1 Change source placed five accepted Markdown sections in one Issue
body and copied all five sections into every Action. A 64 KiB raw-text limit,
a 128 KiB context limit, and a 256 KiB encoded Action limit consequently used
different measurement boundaries. Large but valid requirements could make a
Run unable to issue another Action. The monolithic body also prevented chapters
from having stable permalinks or independent publication.

GitHub Issue comments are independently editable and removable, so comment
order or marker discovery cannot safely establish authority. Ordinary discussion
comments must never become executable requirements merely because they resemble
a managed chapter.

## Decision drivers

- Keep the Issue as the explicit source HEAD without requiring a repository file.
- Give every normative chapter a stable, independently linkable identity.
- Make partial multi-comment publication harmless and detectable.
- Preserve exact accepted bytes for offline Run recovery.
- Keep every Action bounded without truncating normative material.
- Keep GitHub credentials and network access in the trusted host, not the core.
- Bound every untrusted collection and byte stream.

## Decision

Source protocol version 2 uses a manifest-addressed bundle:

1. The first nonempty Issue-body line is the exact `change/v2` marker. The next
   nonempty block is one strict `slipway-manifest` JSON fence.
2. The manifest is the only source HEAD. Its ordered entries bind a stable
   section key and role to a GitHub comment node ID and an exact body digest.
3. The raw envelope contains exactly the referenced comments. Slipway never
   scans or accepts unreferenced discussion comments.
4. Each referenced comment starts with an exact section marker. Its normalized
   payload is content-addressed and written durably under the Run before a
   journal event may reference it.
5. Pinned source, status, candidates, journals, and Actions retain only the
   ordered catalog, provenance, byte counts, and domain-separated revisions.
   Markdown is returned only by the local `protocol material` operation.
6. The Issue body manifest is updated last during publication. New unreferenced
   chapter comments are drafts. An accepted comment identity is immutable across
   manifest heads: the service rejects in-place changes, so an amendment publishes
   a replacement comment and a new manifest.
7. A missing, minimized, edited, duplicated, or hash-mismatched referenced
   comment fails closed. Existing Runs may continue only from their explicitly
   selected local pinned bundle.

The bounded v2 profile permits at most 64 chapters, 256 KiB per normalized
chapter, 4 MiB in aggregate, a 256 KiB manifest, and a 16 MiB raw JSON envelope.
A raw observation carries at most 100 labels, and a pinned projection retains at
most 64 prior transfer URL aliases. These are input and storage safety limits,
not Action-context limits.

Revision framing uses explicit domains:

- `slipway-comment-body/v1`
- `slipway-material/v1`
- `slipway-section/v2`
- `slipway-manifest/v2`
- `slipway-requirements/v2`
- `slipway-source/v2`

Timestamps, REST database IDs, URLs, and fetch metadata are provenance only and do not determine requirements identity. The manifest revision nevertheless commits each referenced comment's GraphQL node ID and REST database ID so their immutable binding cannot drift beneath one manifest head. The GraphQL node ID remains the canonical remote object identity; manifest array order is the canonical chapter order.

## Publication protocol

1. Refetch the current Issue head and expected parent requirements revision. Every
   amendment manifest, including a content-identical comment replacement, declares
   that parent; an initial manifest omits it.
2. Preview the complete chapter drafts, exact body digests, section order/roles,
   operation/item IDs, and all planned external writes. Obtain confirmation to
   create non-authoritative draft resources.
3. For a new Change, create a reconcilable draft Issue shell with publication
   markers but **without** the `change/v2` marker. For an amendment, keep the
   accepted Issue body unchanged.
4. Create replacement chapter comments, then refetch and verify their node IDs,
   database IDs, bodies, and visibility. Unreferenced comments remain drafts and
   have no execution authority.
5. Build and preview the exact final manifest now that remote IDs exist. Obtain a
   second current confirmation for the Issue-body commit.
6. Immediately refetch the accepted head and reject parent-revision drift.
7. Update the Issue body with the `change/v2` marker and manifest; preserve
   reconciliation markers after the manifest fence.
8. Refetch and reconcile the complete accepted graph.

There is no GitHub multi-comment transaction or body compare-and-swap. The final
manifest update is the single logical commit point. If the second confirmation is
declined or any earlier step fails, draft comments (and a new draft shell, if any)
remain unreferenced and non-authoritative; do not silently delete or promote them.
Concurrent parent-revision drift requires a new preview instead of
last-writer-wins reconciliation.

## Current references

This ADR records rationale; the following artifacts implement, describe, and observe the current contract:

- [`docs/reference/v2/source-envelope.schema.json`](../docs/reference/v2/source-envelope.schema.json) and [`machine-protocol.schema.json`](../docs/reference/v2/machine-protocol.schema.json) are the canonical version 2 serialization schemas.
- [`internal/autopilot/source.go`](../internal/autopilot/source.go) and [`source_bundle.go`](../internal/autopilot/source_bundle.go) implement parsing, identity, and validation; [`service.go`](../internal/autopilot/service.go) coordinates durable writes through [`internal/runstore/materials.go`](../internal/runstore/materials.go), and [`material.go`](../internal/autopilot/material.go) serves verified reads.
- The [machine protocol reference](../docs/en/reference/machine-protocol.md) and [v2 tutorial](../docs/en/guides/machine-protocol-v2.md) describe the integration surface.
- [`tests/acceptance/machine-protocol.sh`](../tests/acceptance/machine-protocol.sh) exercises issue-backed import, material reads, refresh, candidate choice, and idempotency.

If these artifacts and this historical record differ, reconcile them with the complete Chinese contract in issue #434 and the versioned schemas. Implementation describes the behavior users can currently observe; executable evidence records particular runs and never becomes runtime, merge, or release authority. Update or supersede the ADR rather than silently treating its prose as runtime specification.

## Consequences

Positive consequences are independently addressable chapters, deterministic
authority, offline recovery, smaller Actions, and explicit amendment failures.
The host fetches only manifest-referenced comments, while the core remains
network- and credential-free.

Negative consequences are an intentionally breaking v2 wire/storage contract,
additional host publication steps, and local material blob lifecycle management.
No released CLI consumed v1, so no ambient v1 compatibility reader is kept. Any
later migration must be an explicit, previewed operation rather than a runtime
alias.

## Rejected alternatives

- A single body remains atomic but repeats the original size and addressability
  problems.
- Scanning marker-like comments makes discussion content, order, pagination, and
  concurrent authors implicit authority.
- One child Issue per chapter creates status, label, and relationship noise.
- Repository Markdown gives stronger Git history but makes an external repository
  ref a hard source dependency and weakens the Issue-first product boundary.
- A second manifest comment adds another mutable object and fetch without removing
  the need for an Issue-body HEAD.
