package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/require"
)

func writeAuthReviewGovernedBundle(t *testing.T, root, slug string) {
	t.Helper()

	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundlePath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "intent.md"), []byte(`# Intent
INT-001: protect auth flows
## Open Questions
(none)
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "requirements.md"), []byte(`# Requirements
### Requirement: auth review
REQ-001: Auth changes must keep MFA intact. Traces to INT-001.
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "decision.md"), []byte(`# Decision
## Alternatives Considered
### Option A
Keep existing MFA policy and update login checks.
### Option B
Refactor login middleware with the same MFA contract.

## Selected Approach
Adopt Option A because it changes less code.

## Interfaces and Data Flow
Auth entrypoints keep the existing MFA contract.

## Rollout and Rollback
Roll forward with a guarded rollout and roll back by restoring the prior auth handler.

## Risk
Regression risk is concentrated in auth flows.
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`# Tasks
- [ ] audit auth flow
  covers: [REQ-001]
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "assurance.md"), []byte(`# Assurance
## Scope Summary
Auth review.

## Verification Verdict
Pending.

## Evidence Index
Pending.

## Requirement Coverage
REQ-001: pending review evidence

## Residual Risks and Exceptions
Pending.

## Archive Decision
Not ready.
`), 0o644))
}

func requireBlockerContains(t *testing.T, blockers []model.ReasonCode, want string) {
	t.Helper()
	require.Contains(t, strings.Join(model.ReasonSpecs(blockers), "\n"), want)
}
