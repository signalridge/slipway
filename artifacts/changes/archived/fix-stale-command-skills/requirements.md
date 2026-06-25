# Requirements

## Requirements

### Requirement: Retired command skills are pruned by class, not by name
REQ-001: The system MUST remove every generated command-skill directory whose
`command_id` is absent from the current command registry during adapter refresh,
regardless of whether the command was retired before or after this change, and
without relying on a hand-maintained list of specific retired command names.

#### Scenario: refresh removes residue for any retired command
GIVEN a Slipway-generated adapter and a generated-shape command-skill directory
whose `command_id` is no longer in the current command registry
WHEN `slipway init --refresh` regenerates the adapter
THEN the retired command-skill directory is removed from the adapter skill tree.

#### Scenario: refresh removes residue for a command never enumerated in this change
GIVEN a generated-shape command-skill directory for a synthetic retired
`command_id` that is not one of the originally reported retired names
WHEN `slipway init --refresh` regenerates the adapter
THEN the directory is removed, proving the cleanup generalizes to future
retirements rather than matching an enumerated set.

#### Scenario: residue recorded in the prior ownership manifest is pruned
GIVEN a command-skill directory that the previous generation recorded in the
ownership manifest for a now-retired command
WHEN `slipway init --refresh` regenerates the adapter
THEN the directory is removed through the manifest-tracked cleanup path.

### Requirement: No surviving command skill maps to a non-registry command
REQ-002: After adapter refresh, the system MUST NOT leave any command-skill
directory on disk whose `command_id` is absent from the live CLI command
surface, and generated command skills MUST advertise only command IDs that
resolve to a registered command on the live root command tree.

#### Scenario: post-refresh disk tree has no orphan command skills
GIVEN an adapter refreshed by Slipway
WHEN the reconciled on-disk skill tree is inspected
THEN every surviving `slipway-<id>` command-skill directory carries a
`command_id` that resolves to a registered command.

#### Scenario: generated command IDs resolve on the live root command
GIVEN generated command-skill surfaces for a host adapter
WHEN a contract test parses each generated command skill `command_id`
THEN every ID resolves to a command registered on the live root command tree
(`newRootCmd().Commands()`), not merely the registry slice the generator
iterates, so the check cannot pass tautologically.

### Requirement: User-owned and user-modified skills are preserved fail-closed
REQ-003: The system MUST preserve user-owned adjacent skill directories during
adapter refresh, and MUST fail closed (refuse to delete) when a directory under
a retired command-skill name holds user-authored or user-modified content
rather than pristine generated content.

#### Scenario: prefixed user-owned skill is preserved
GIVEN a user-owned `slipway-*` skill directory beside generated adapter skills
WHEN adapter refresh runs
THEN the user-owned skill directory remains in place.

#### Scenario: user-modified content under a retired name is preserved
GIVEN a directory under a retired command-skill name whose SKILL.md carries
generated-shape frontmatter but a user-modified body
WHEN adapter refresh runs
THEN the directory is preserved because the cleanup refuses to delete modified
managed content, rather than silently destroying the user's edits.

### Requirement: Pruning applies to every command-skill host
REQ-004: Retired command-skill cleanup MUST apply uniformly to every host
adapter that emits a command-skill surface, not only the host first reported.

#### Scenario: every command-skill host prunes retired residue
GIVEN retired generated command-skill residue under each host adapter that
generates a command-skill surface
WHEN `slipway init --refresh` regenerates those adapters
THEN the retired residue is removed for every such host.
