# Requirements

## Requirements

### Requirement: Transactional governed file sets
REQ-001: Governed stage transitions that mutate more than one lifecycle artifact or authority file MUST apply those file mutations all-or-nothing: if any write or remove operation in the transition fails after an earlier operation succeeded, the system MUST restore pre-transition file contents and remove files created by the failed transaction.

#### Scenario: failed write after a created artifact
GIVEN a governed transition that creates a missing artifact file and then persists another lifecycle file
WHEN the second file mutation fails after the first file has been created
THEN the created artifact file is absent after the command returns
AND the lifecycle authority remains at the pre-transition state
AND the command reports failure instead of advancing the change.

#### Scenario: failed write after replacing existing content
GIVEN a governed transition that replaces an existing file before a later file mutation
WHEN the later file mutation fails
THEN the existing file is restored to its original bytes
AND no partially updated file-set state is reported as successful.

### Requirement: Transactional deletes for stale evidence recovery
REQ-002: Governed stale-evidence recovery MUST treat evidence-file removals and the reopened lifecycle state save as one file-set transaction, so a failure to save the reopened state SHALL restore any verification, wave-plan, or execution-summary files removed earlier in the same recovery attempt.

#### Scenario: failed reopen after evidence removal
GIVEN a governed change with verification evidence that must be removed during stale-evidence recovery
WHEN the recovery removes one or more evidence files but then fails to save the reopened `change.yaml`
THEN the removed evidence files are restored
AND the change remains in its pre-recovery lifecycle state
AND governance reports a failure requiring rerun rather than a successful reopen.

### Requirement: Fail-closed rollback diagnostics
REQ-003: If rollback after a failed multi-file transition cannot fully restore or remove a file, the system MUST fail closed and include the original operation error plus the path of each file that may require inspection.

#### Scenario: rollback cannot restore a file
GIVEN a governed file-set transaction where a later operation fails
AND rollback of an earlier operation also fails
WHEN the command returns the transaction error
THEN the error names the file path that could not be rolled back
AND the command does not report the transition as successful
AND no bypass, force-pass, or private attestation path is accepted for the irreversible-operations guardrail.

### Requirement: Covered transition surfaces
REQ-004: The implementation SHALL apply the transactional file-set mechanism to the issue #164 transition surfaces identified in research: S1 planning bundle scaffold before state save, stale-evidence reopen file removals before state save, and S1-to-S2 wave-plan materialization before state save.

#### Scenario: covered transitions use the shared guardrail
GIVEN tests or code inspection of the identified transition surfaces
WHEN any of those paths performs multiple file mutations before reporting advancement
THEN the path uses the shared transaction mechanism or an equivalent wrapper with the same all-or-nothing and rollback diagnostic behavior
AND targeted tests cover at least one mid-transition injected failure for artifact creation and one for evidence removal.

### Requirement: Regression proof
REQ-005: The change MUST include deterministic regression tests that simulate a mid-transition failure after at least one file mutation and prove no partial governed bundle or evidence state remains.

#### Scenario: injected mid-transition failure
GIVEN a test seam that fails a file mutation after one operation in a governed transition has succeeded
WHEN the transition is executed under that injected failure
THEN assertions verify newly-created files are absent
AND assertions verify pre-existing files are restored
AND assertions verify the persisted lifecycle authority did not advance.
