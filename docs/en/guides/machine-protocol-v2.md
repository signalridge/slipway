# Machine protocol v2 tutorial

This tutorial runs one complete local protocol lifecycle: start a Run, submit Orient and Implement outcomes, and end with Summarize. It is intended for host and adapter authors. Generated host capabilities normally perform these protocol operations; they are not an alternative end-user workflow.

The canonical contracts are the versioned [machine protocol schema](../../reference/v2/machine-protocol.schema.json) and [source envelope schema](../../reference/v2/source-envelope.schema.json). Keep the version in the URL and the JSON `contract_version` or `source_version` together; unversioned compatibility aliases are intentionally not published.

![Slipway machine protocol exchange: the host starts a Run and receives the first orient Action with a structured next operation; for every Action it performs the work and submits one strict Outcome, and the CLI validates it, appends a journal event, observes Git independently, and returns the following Action; a needs_input Outcome pauses until the host answers or skips, and an explicit resume revalidates workspace identity and voids stale work.](../../assets/diagrams/protocol-sequence.svg)

## Prerequisites

Install `slipway`, `git`, and `jq`. Run every snippet in the same shell session. The tutorial works inside a disposable directory because it creates a real Run journal and changes a tracked file.

```bash
TUTORIAL_DIR=$(mktemp -d)
WORKSPACE="$TUTORIAL_DIR/workspace"
mkdir -p "$WORKSPACE"
cd "$TUTORIAL_DIR"
git -C "$WORKSPACE" init -q
git -C "$WORKSPACE" config user.name 'Protocol Tutorial'
git -C "$WORKSPACE" config user.email tutorial@example.invalid
printf '# Protocol tutorial\n' > "$WORKSPACE/README.md"
git -C "$WORKSPACE" add README.md
git -C "$WORKSPACE" commit -qm initial
```

## 1. Start the Run

Pass flags before the `--` separator and keep the goal as one literal argument. `--no-review` makes this short lifecycle proceed directly from Implement to Summarize.

```bash
slipway run \
  --budget 4 \
  --json \
  --root "$WORKSPACE" \
  --no-review \
  -- "add one tutorial line to README.md" > start.json

jq -e '
  .contract_version == 2 and
  .state == "active" and
  .action.kind == "orient" and
  .next.operation == "action"
' start.json

RUN_ID=$(jq -r '.run_id' start.json)
ORIENT_ID=$(jq -r '.action.action_id' start.json)
```

A production host should validate the complete response against the machine protocol schema. The `jq` expression only makes the important tutorial assertions visible. Preserve `next.variants[].base_argv` as an argument array; never parse a rendered command string.

## 2. Submit the Orient outcome

Every public Outcome field is present. Branches that do not apply are JSON `null`, and empty collections remain arrays.

```bash
jq -n --arg action "$ORIENT_ID" '{
  contract_version: 2,
  action_id: $action,
  action_kind: "orient",
  status: "completed",
  summary: "Repository facts observed.",
  observations: ["README.md is the only tracked file."],
  known_issues: [],
  suggested_actions: [{
    kind: "implement",
    brief: "Append the requested tutorial line."
  }],
  pause: null,
  implementation: null,
  review: null
}' > orient-outcome.json

slipway protocol submit \
  --run "$RUN_ID" \
  --action "$ORIENT_ID" \
  --root "$WORKSPACE" \
  --outcome-file orient-outcome.json > implement.json

jq -e '.contract_version == 2 and .action.kind == "implement"' implement.json
IMPLEMENT_ID=$(jq -r '.action.action_id' implement.json)
```

The `action_id` and `action_kind` must match the outstanding Action. A host must not copy an ID from an earlier response or invent the next Action.

## 3. Perform and report implementation

Make the observable change, run a check, and report the exact activity and exit code.

```bash
printf 'Protocol v2 lifecycle completed.\n' >> "$WORKSPACE/README.md"
git -C "$WORKSPACE" diff --check

jq -n --arg action "$IMPLEMENT_ID" \
  --arg check_command "git -C \"$WORKSPACE\" diff --check" '{
  contract_version: 2,
  action_id: $action,
  action_kind: "implement",
  status: "completed",
  summary: "Appended the requested README line.",
  observations: [],
  known_issues: [],
  suggested_actions: [],
  pause: null,
  implementation: {
    result: "applied",
    files_changed: ["README.md"],
    activities: [{
      kind: "test",
      command: $check_command,
      exit_code: 0,
      summary: "No whitespace errors."
    }],
    uncertainties: [],
    attempts: 1
  },
  review: null
}' > implement-outcome.json

slipway protocol submit \
  --run "$RUN_ID" \
  --action "$IMPLEMENT_ID" \
  --root "$WORKSPACE" \
  --outcome-file implement-outcome.json > summarize.json

jq -e '.action.kind == "summarize"' summarize.json
SUMMARIZE_ID=$(jq -r '.action.action_id' summarize.json)
```

`files_changed` is a host report, not proof of attribution. Slipway separately records bounded Git observations and preserves uncertainty about concurrent user or tool changes.

## 4. End the Run

```bash
jq -n --arg action "$SUMMARIZE_ID" '{
  contract_version: 2,
  action_id: $action,
  action_kind: "summarize",
  status: "completed",
  summary: "The requested README update is complete and git diff --check passed.",
  observations: [],
  known_issues: [],
  suggested_actions: [],
  pause: null,
  implementation: null,
  review: null
}' > summarize-outcome.json

slipway protocol submit \
  --run "$RUN_ID" \
  --action "$SUMMARIZE_ID" \
  --root "$WORKSPACE" \
  --outcome-file summarize-outcome.json > ended.json

jq -e '
  .contract_version == 2 and
  .state == "ended" and
  .next.operation == "none" and
  (.next.variants | length) == 0
' ended.json

rm -rf "$TUTORIAL_DIR"
```

Submitting the exact same Outcome bytes again is idempotent. Different bytes for the same completed Action fail with `outcome_conflict`; a stale Action ID fails closed. Branch on the structured error `code`, not message text.

## Issue-backed extension

For an issue-backed Run, the trusted host first validates and writes a private temporary source envelope, then passes it once with `--source-file`. The response adds `pinned_source`, `action.source`, `action.requirements`, and a structured `protocol material` reader. Fetch only manifest-referenced comments and never treat discussion comments as requirements. See the [machine protocol reference](../reference/machine-protocol.md) and [ADR-0001](../../../adr/0001-source-bundle-v2.md) for the authority and publication model.
