# Requirements

## Requirements

### Requirement: Documentation facts and links are current
REQ-001: The documentation SHALL remove stale factual references, broken
localized asset paths, and README parity defects identified by the audit.

#### Scenario: Stale docs stack reference is repaired
GIVEN the installation docs describe the documentation preview stack
WHEN a reader follows the changed English, Chinese, or Japanese installation docs
THEN the docs identify the current Astro Starlight stack rather than the retired
MkDocs Material stack.

#### Scenario: Localized assets resolve
GIVEN localized docs are nested under `docs/zh/` or `docs/ja/`
WHEN a renderer resolves image references in changed localized pages
THEN referenced SVG assets resolve through the correct relative path.

### Requirement: Command surfaces are complete and accurate
REQ-002: The command reference and generated surface manifest MUST describe the
current public command surface without omitted commands or stale JSON examples.

#### Scenario: Hook command is represented
GIVEN `slipway hook` is a public command surface
WHEN command references and the surface manifest are generated or checked
THEN `slipway hook` appears with the correct source and public classification.

#### Scenario: Config JSON example is valid
GIVEN a reader follows localized command examples
WHEN they copy the config JSON example
THEN the example uses a concrete supported subcommand rather than `slipway config --json`.

### Requirement: Localized terminology is consistent and natural
REQ-003: The localized docs SHALL use consistent high-impact terminology for
artifact, governed change, fail-closed behavior, freshness, and adjacent
governance concepts where the audit found drift or machine-translation wording.

#### Scenario: Chinese terminology is normalized
GIVEN a Chinese reader scans changed docs
WHEN the docs describe evidence freshness, artifacts, and governed changes
THEN the wording avoids misleading food-like or calque terms and uses consistent
technical Chinese.

#### Scenario: Japanese terminology is normalized
GIVEN a Japanese reader scans changed docs
WHEN the docs describe command behavior, authority, and enforcement
THEN the wording avoids incorrect or overly dramatic translations and uses
consistent technical Japanese.

### Requirement: Verification closes the audit findings
REQ-004: The change SHALL include targeted verification that the documented
audit findings were corrected and generated docs surfaces remain in sync.

#### Scenario: Docs/toolgen checks pass
GIVEN the repair has been applied
WHEN the toolgen tests, manifest check, and targeted reference scans run
THEN the checks pass or any intentionally retained wording is documented with a
specific rationale.
