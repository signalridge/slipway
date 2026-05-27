package governance

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/model"
	"gopkg.in/yaml.v3"
)

const (
	maxAdvisoryPolicyPackItems     = 20
	maxAdvisoryPolicyPackItemBytes = 240
)

// AdvisoryPolicyPack is the bounded, non-blocking projection of a project-local
// policy pack. Built-in controls remain the only fail-closed enforcement path.
type AdvisoryPolicyPack struct {
	Name                 string   `json:"name"`
	Path                 string   `json:"path"`
	SchemaVersion        string   `json:"schema_version,omitempty"`
	AdvisoryRules        []string `json:"advisory_rules,omitempty"`
	ArtifactRequirements []string `json:"artifact_requirements,omitempty"`
	RecommendedReviewers []string `json:"recommended_reviewers,omitempty"`
	Terminology          []string `json:"terminology,omitempty"`
}

// LoadAdvisoryPolicyPack parses a policy pack and rejects any blocking-policy
// declarations. The parsed result is intentionally bounded for handoff use.
func LoadAdvisoryPolicyPack(name, path string) (AdvisoryPolicyPack, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return AdvisoryPolicyPack{}, fmt.Errorf("policy pack %q unreadable: %v", name, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return AdvisoryPolicyPack{}, fmt.Errorf("policy pack %q parse error: %v", name, err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return AdvisoryPolicyPack{}, fmt.Errorf("policy pack %q root must be a YAML mapping", name)
	}

	pack := AdvisoryPolicyPack{Name: strings.TrimSpace(name), Path: strings.TrimSpace(path)}
	root := doc.Content[0]
	blockingKeys := []string{}
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := strings.TrimSpace(root.Content[i].Value)
		value := root.Content[i+1]
		switch key {
		case "version", "schema_version":
			pack.SchemaVersion = scalarNodeString(value)
		case "name":
			if pack.Name == "" {
				pack.Name = scalarNodeString(value)
			}
		case "advisory_rules", "checklist", "advisory_checklist":
			pack.AdvisoryRules = appendBoundedStrings(pack.AdvisoryRules, nodeStringItems(value)...)
		case "artifact_requirements", "extra_artifact_requirements":
			pack.ArtifactRequirements = appendBoundedStrings(pack.ArtifactRequirements, nodeStringItems(value)...)
		case "recommended_reviewers", "reviewers":
			pack.RecommendedReviewers = appendBoundedStrings(pack.RecommendedReviewers, nodeStringItems(value)...)
		case "terminology", "terms":
			pack.Terminology = appendBoundedStrings(pack.Terminology, nodeStringItems(value)...)
		case "blocking", "blocking_controls", "guardrail_domains", "fail_closed_domains":
			blockingKeys = append(blockingKeys, key)
		case "mode":
			if mode := scalarNodeString(value); mode != "" && mode != string(model.ControlModeAdvisory) {
				blockingKeys = append(blockingKeys, "mode="+mode)
			}
		}
	}
	if pack.SchemaVersion == "" {
		return AdvisoryPolicyPack{}, fmt.Errorf("policy pack %q is missing version/schema_version", name)
	}
	if len(blockingKeys) > 0 {
		slices.Sort(blockingKeys)
		return AdvisoryPolicyPack{}, fmt.Errorf("policy pack %q declares unsupported blocking fields: %s", name, strings.Join(blockingKeys, ", "))
	}
	if pack.Name == "" {
		pack.Name = strings.TrimSpace(name)
	}
	return pack, nil
}

func scalarNodeString(node *yaml.Node) string {
	if node == nil {
		return ""
	}
	return strings.TrimSpace(node.Value)
}

func nodeStringItems(node *yaml.Node) []string {
	if node == nil {
		return nil
	}
	switch node.Kind {
	case yaml.SequenceNode:
		items := make([]string, 0, len(node.Content))
		for _, child := range node.Content {
			items = append(items, nodeStringItems(child)...)
		}
		return items
	case yaml.MappingNode:
		items := make([]string, 0, len(node.Content)/2)
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := scalarNodeString(node.Content[i])
			value := scalarNodeString(node.Content[i+1])
			if key == "" && value == "" {
				continue
			}
			if value == "" {
				items = append(items, key)
				continue
			}
			items = append(items, key+"="+value)
		}
		return items
	default:
		if value := scalarNodeString(node); value != "" {
			return []string{value}
		}
		return nil
	}
}

func appendBoundedStrings(dst []string, values ...string) []string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if len(value) > maxAdvisoryPolicyPackItemBytes {
			value = value[:maxAdvisoryPolicyPackItemBytes]
		}
		if slices.Contains(dst, value) {
			continue
		}
		dst = append(dst, value)
		if len(dst) >= maxAdvisoryPolicyPackItems {
			return dst
		}
	}
	return dst
}
