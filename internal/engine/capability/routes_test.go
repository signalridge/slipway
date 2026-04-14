package capability

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidModesForReviewIncludesIndependentReview(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	modes := ValidModesForCommand(reg, "review")
	assert.Contains(t, modes, "independent-review")
	assert.Contains(t, modes, "differential-review")
	assert.Contains(t, modes, "security-review")
}

func TestValidModesForValidateIncludesSpecTrace(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	modes := ValidModesForCommand(reg, "validate")
	assert.Contains(t, modes, "spec-trace")
	assert.Contains(t, modes, "coverage-analysis")
}

func TestValidModesForRepairIncludesRootCauseTracing(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	modes := ValidModesForCommand(reg, "repair")
	assert.Contains(t, modes, "root-cause-tracing")
	assert.Contains(t, modes, "ci-triage")
}

func TestValidViewsForStatusIncludesIncidentResponse(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	views := ValidViewsForCommand(reg, "status")
	assert.Contains(t, views, "incident-response")
}

func TestValidViewsForHealthIncludesIncidentResponse(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	views := ValidViewsForCommand(reg, "health")
	assert.Contains(t, views, "incident-response")
}

func TestValidModesEmptyForUnknownCommand(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	assert.Empty(t, ValidModesForCommand(reg, "next"))
	assert.Empty(t, ValidModesForCommand(reg, ""))
	assert.Empty(t, ValidModesForCommand(nil, "review"))
}

func TestBindingMatchesCommand_PrefixedTargetsAreCommandScoped(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		target  string
		command string
		want    bool
	}{
		{name: "bare command target", target: "review", command: "review", want: true},
		{name: "command prefix exact match", target: "command:validate", command: "validate", want: true},
		{name: "command prefix mismatch", target: "command:validate", command: "review", want: false},
		{name: "mode prefix with command and route", target: "mode:review:security-review", command: "review", want: true},
		{name: "mode prefix rejects other commands", target: "mode:review:security-review", command: "repair", want: false},
		{name: "view prefix with command and view", target: "view:status:incident-response", command: "status", want: true},
		{name: "view prefix rejects other commands", target: "view:status:incident-response", command: "health", want: false},
		{name: "mode prefix single segment treated as command scope", target: "mode:repair", command: "repair", want: true},
		{name: "empty target never matches", target: "", command: "review", want: false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := bindingMatchesCommand(Binding{Target: tc.target}, tc.command)
			assert.Equal(t, tc.want, got)
		})
	}
}
