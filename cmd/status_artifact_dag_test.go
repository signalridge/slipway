package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/progression"
	enginestatus "github.com/signalridge/slipway/internal/engine/status"
	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildArtifactDAGWithSchema(change model.Change, schema []artifact.ArtifactSpec, preset ...model.WorkflowPreset) []artifactDAGNode {
	if len(schema) == 0 {
		return nil
	}

	required := map[string]bool{}
	presetArgs := append([]model.WorkflowPreset{change.WorkflowPreset}, preset...)
	for _, name := range artifact.RequiredArtifactsForChange(schema, change.NeedsDiscovery, presetArgs...) {
		required[name] = true
	}

	stateOf := func(name string) string {
		artifactID := strings.TrimSuffix(name, filepath.Ext(name))
		if as, ok := change.Artifacts[artifactID]; ok {
			return string(as.State)
		}
		return "pending"
	}

	doneStates := map[string]bool{
		string(model.ArtifactLifecycleApproved): true,
		string(model.ArtifactLifecycleFrozen):   true,
	}

	var nodes []artifactDAGNode
	for _, spec := range schema {
		if !required[spec.Name] {
			continue
		}
		st := stateOf(spec.Name)
		ready := true
		deps := make([]string, 0, len(spec.DependsOn))
		for _, dep := range spec.DependsOn {
			if !required[dep] {
				continue
			}
			deps = append(deps, dep)
			if !doneStates[stateOf(dep)] {
				ready = false
			}
		}
		nodes = append(nodes, artifactDAGNode{
			Name:      spec.Name,
			State:     st,
			DependsOn: deps,
			Ready:     ready,
		})
	}
	return nodes
}

func TestBuildArtifactDAGUsesArtifactIDsAndFiltersLevelScopedDependencies(t *testing.T) {
	t.Parallel()

	change := model.NewChange("status-dag")
	change.Artifacts["intent"] = model.ArtifactState{ID: "intent", State: model.ArtifactLifecycleApproved}
	change.Artifacts["requirements"] = model.ArtifactState{ID: "requirements", State: model.ArtifactLifecycleApproved}
	change.Artifacts["decision"] = model.ArtifactState{ID: "decision", State: model.ArtifactLifecycleDraft}

	dag := buildArtifactDAGWithSchema(change, artifact.ResolveSchema(model.ArtifactSchemaExpanded, change.CustomArtifacts))
	require.NotEmpty(t, dag)

	nodesByName := make(map[string]artifactDAGNode, len(dag))
	for _, node := range dag {
		nodesByName[node.Name] = node
	}

	decision, ok := nodesByName["decision.md"]
	require.True(t, ok)
	assert.Equal(t, string(model.ArtifactLifecycleDraft), decision.State)
	assert.Equal(t, []string{"intent.md", "requirements.md"}, decision.DependsOn)
}

func TestBuildArtifactDAGKeepsAllRequiredDependenciesWhenArtifactIsNotReady(t *testing.T) {
	t.Parallel()

	change := model.NewChange("status-dag")
	change.Artifacts["intent"] = model.ArtifactState{ID: "intent", State: model.ArtifactLifecycleDraft}
	change.Artifacts["requirements"] = model.ArtifactState{ID: "requirements", State: model.ArtifactLifecycleDraft}
	change.Artifacts["decision"] = model.ArtifactState{ID: "decision", State: model.ArtifactLifecycleDraft}
	change.Artifacts["tasks"] = model.ArtifactState{ID: "tasks", State: model.ArtifactLifecycleDraft}

	dag := buildArtifactDAGWithSchema(change, artifact.ResolveSchema(model.ArtifactSchemaExpanded, change.CustomArtifacts))
	require.NotEmpty(t, dag)

	nodesByName := make(map[string]artifactDAGNode, len(dag))
	for _, node := range dag {
		nodesByName[node.Name] = node
	}

	assurance, ok := nodesByName["assurance.md"]
	require.True(t, ok)
	assert.False(t, assurance.Ready)
	assert.Equal(t, []string{"tasks.md"}, assurance.DependsOn)
}

func TestBuildArtifactDAGUsesEffectivePresetForRequiredArtifacts(t *testing.T) {
	t.Parallel()

	change := model.NewChange("status-dag")
	change.WorkflowPreset = model.WorkflowPresetLight

	dag := buildArtifactDAGWithSchema(change, artifact.ResolveSchema(model.ArtifactSchemaCore, change.CustomArtifacts), model.WorkflowPresetStandard)
	require.NotEmpty(t, dag)

	nodesByName := make(map[string]artifactDAGNode, len(dag))
	for _, node := range dag {
		nodesByName[node.Name] = node
	}

	_, ok := nodesByName["assurance.md"]
	require.True(t, ok, "effective standard preset must keep assurance.md in the DAG even when confirmed preset is light")
}

func TestArtifactDAGFromProjectionSkipsNonRequiredNodes(t *testing.T) {
	t.Parallel()

	change := model.NewChange("projection-dag")
	change.CurrentState = model.StateS1Plan
	projection, err := enginestatus.BuildProjection(t.TempDir(), change, nil, nil, progression.GovernanceReadiness{
		ArtifactProjection: &progression.ArtifactProjection{
			Nodes: []progression.ArtifactProjectionNode{
				{
					Name:      "intent.md",
					State:     string(model.ArtifactLifecycleApproved),
					DependsOn: nil,
					Ready:     true,
					Required:  true,
				},
				{
					Name:      "notes.md",
					State:     string(model.ArtifactLifecycleStale),
					DependsOn: nil,
					Ready:     false,
					Required:  false,
				},
			},
		},
	}, workflowStateLabel)
	require.NoError(t, err)
	dag := mapArtifactDAGNodes(projection.ArtifactDAG)

	require.Len(t, dag, 1)
	assert.Equal(t, "intent.md", dag[0].Name)
}
