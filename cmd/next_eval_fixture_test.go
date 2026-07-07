package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGovernedAgentEvalFixtures(t *testing.T) {
	t.Parallel()

	type fixture struct {
		name           string
		setup          func(t *testing.T, root string) string
		execute        func(t *testing.T, root, slug string) nextView
		wantNextSkill  string
		wantBlockers   []string
		wantGateStatus map[string]string
		wantEvents     []string
	}

	fixtures := []fixture{
		{
			name: "plan audit missing evidence surfaces deterministic next skill and gate blockers",
			setup: func(t *testing.T, root string) string {
				slug := createGovernedRequest(t, root, levelNonDiscovery, "eval fixture plan audit missing")
				change, err := state.LoadChange(root, slug)
				require.NoError(t, err)
				change.PlanSubStep = model.PlanSubStepAudit
				require.NoError(t, state.SaveChange(root, change))
				return slug
			},
			execute: func(t *testing.T, root, slug string) nextView {
				view, err := buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{Preview: true, AutoSkipEvidence: true, Command: "run"})
				require.NoError(t, err)
				return view
			},
			wantNextSkill: "plan-audit",
			wantBlockers:  []string{"required_skill_missing:plan-audit"},
			wantGateStatus: map[string]string{
				"G_plan": "blocked",
			},
		},
		{
			name: "run consumes intake evidence and emits lifecycle event fragment",
			setup: func(t *testing.T, root string) string {
				slug := createIntakeChangeFixture(t, root, "eval fixture consume intake evidence")
				bundleDir := filepath.Join(root, "artifacts", "changes", slug)
				require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte(`# Intent

## Summary
Refresh operator docs.

## In Scope
Update docs.

## Out of Scope
No code changes.

## Acceptance Signals
Docs render.
`), 0o644))
				writeSkillVerification(t, root, slug, progression.SkillIntakeClarification, model.VerificationRecord{
					Verdict:    model.VerificationVerdictPass,
					Blockers:   []model.ReasonCode{},
					Timestamp:  time.Now().UTC(),
					RunVersion: 0,
				})
				return slug
			},
			execute: func(t *testing.T, root, slug string) nextView {
				view, err := runGovernedLoop(root, changeRef{Slug: slug}, false)
				require.NoError(t, err)
				return view
			},
			wantBlockers: []string{"no_skill_required:S0_INTAKE"},
			wantEvents:   []string{"state.substep_transitioned", "skill.evidence_recorded"},
		},
	}

	for _, fx := range fixtures {
		fx := fx
		t.Run(fx.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			ensureTestGitRepo(t, root)
			initTestWorkspace(t, root)

			slug := fx.setup(t, root)
			view := fx.execute(t, root, slug)

			assert.Equal(t, slug, view.Slug)
			if fx.wantNextSkill != "" {
				require.NotNil(t, view.NextSkill)
				assert.Equal(t, fx.wantNextSkill, view.NextSkill.Name)
			}
			for _, want := range fx.wantBlockers {
				assert.Contains(t, strings.Join(model.ReasonSpecs(view.Blockers), "\n"), want)
			}
			for gateID, status := range fx.wantGateStatus {
				require.NotNil(t, view.InputContext.GateStatus)
				assert.Equal(t, status, view.InputContext.GateStatus[gateID])
			}
			if len(fx.wantEvents) > 0 {
				change, err := state.LoadChange(root, slug)
				require.NoError(t, err)
				events, err := state.ReadLifecycleEvents(root, change)
				require.NoError(t, err)
				eventTypes := make([]string, 0, len(events))
				for _, event := range events {
					eventTypes = append(eventTypes, event.EventType)
				}
				for _, want := range fx.wantEvents {
					assert.Contains(t, eventTypes, want)
				}
			}
		})
	}
}
