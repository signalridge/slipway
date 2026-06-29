package model

import (
	"reflect"
	"strings"
	"testing"
)

func TestConfigSubagentsResolveFallsBackToDefault(t *testing.T) {
	sa := ConfigSubagents{
		Default: SubagentProfile{
			Model:             "default-model",
			AllowedSkills:     []string{"skill-a"},
			AllowedMCPServers: []string{"mcp-a"},
		},
		Review: SubagentProfile{
			Model: "review-model",
		},
		Fix: SubagentProfile{
			AllowedSkills: []string{"fix-skill"},
		},
	}

	// Review overrides only Model; skills/MCP fall back to Default.
	review := sa.Resolve(SubagentStageReview)
	if review.Model != "review-model" {
		t.Errorf("review model = %q, want review-model", review.Model)
	}
	if !reflect.DeepEqual(review.AllowedSkills, []string{"skill-a"}) {
		t.Errorf("review skills = %v, want default [skill-a]", review.AllowedSkills)
	}
	if !reflect.DeepEqual(review.AllowedMCPServers, []string{"mcp-a"}) {
		t.Errorf("review mcp = %v, want default [mcp-a]", review.AllowedMCPServers)
	}

	// Fix overrides only skills; model/MCP fall back to Default.
	fix := sa.Resolve(SubagentStageFix)
	if fix.Model != "default-model" {
		t.Errorf("fix model = %q, want default-model", fix.Model)
	}
	if !reflect.DeepEqual(fix.AllowedSkills, []string{"fix-skill"}) {
		t.Errorf("fix skills = %v, want [fix-skill]", fix.AllowedSkills)
	}

	// Verify has no override; entirely Default.
	verify := sa.Resolve(SubagentStageVerify)
	if verify.Model != "default-model" || !reflect.DeepEqual(verify.AllowedSkills, []string{"skill-a"}) {
		t.Errorf("verify profile = %+v, want full default", verify)
	}
}

func TestConfigSubagentsResolveEmptyInheritsNothing(t *testing.T) {
	var sa ConfigSubagents
	got := sa.Resolve(SubagentStageReview)
	if !got.IsZero() {
		t.Errorf("zero subagents resolve = %+v, want zero (host inherits)", got)
	}
}

func TestConfigSubagentsIsZero(t *testing.T) {
	if !(ConfigSubagents{}).IsZero() {
		t.Error("empty ConfigSubagents should be zero")
	}
	if (ConfigSubagents{Verify: SubagentProfile{Model: "x"}}).IsZero() {
		t.Error("ConfigSubagents with a verify model must not be zero")
	}
}

func TestConfigSubagentsYAMLRoundTrip(t *testing.T) {
	in := []byte("subagents:\n  default:\n    model: fast\n  review:\n    allowed_skills:\n      - code-quality-review\n    allowed_mcp_servers:\n      - serena\n")
	cfg, err := ParseConfigYAML(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Subagents.Default.Model != "fast" {
		t.Errorf("default.model = %q, want fast", cfg.Subagents.Default.Model)
	}
	if len(cfg.UnknownTopLevel) != 0 {
		t.Errorf("subagents must not land in UnknownTopLevel: %v", cfg.UnknownTopLevel)
	}

	out, err := cfg.ToYAML()
	if err != nil {
		t.Fatalf("toYAML: %v", err)
	}
	if !strings.Contains(string(out), "subagents:") {
		t.Errorf("ToYAML did not emit subagents section:\n%s", out)
	}
	back, err := ParseConfigYAML(out)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if !reflect.DeepEqual(cfg.Subagents, back.Subagents) {
		t.Errorf("subagents round-trip drift: %+v vs %+v", cfg.Subagents, back.Subagents)
	}
}

func TestConfigSubagentsAbsentWhenZero(t *testing.T) {
	cfg := DefaultConfig()
	out, err := cfg.ToYAML()
	if err != nil {
		t.Fatalf("toYAML: %v", err)
	}
	if strings.Contains(string(out), "subagents:") {
		t.Errorf("zero subagents must not be emitted:\n%s", out)
	}
}

func TestConfigGitHubIsZeroAndRoundTrip(t *testing.T) {
	if !(ConfigGitHub{}).IsZero() {
		t.Error("empty ConfigGitHub should be zero")
	}
	in := []byte("github:\n  api_url: https://ghe.example.com/api/v3\n  api_allowed_base_urls:\n    - https://ghe.example.com/api/v3\n")
	cfg, err := ParseConfigYAML(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.GitHub.APIURL != "https://ghe.example.com/api/v3" {
		t.Errorf("api_url = %q", cfg.GitHub.APIURL)
	}
	if len(cfg.UnknownTopLevel) != 0 {
		t.Errorf("github must not land in UnknownTopLevel: %v", cfg.UnknownTopLevel)
	}
	out, err := cfg.ToYAML()
	if err != nil {
		t.Fatalf("toYAML: %v", err)
	}
	back, err := ParseConfigYAML(out)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if !reflect.DeepEqual(cfg.GitHub, back.GitHub) {
		t.Errorf("github round-trip drift: %+v vs %+v", cfg.GitHub, back.GitHub)
	}
}

func TestConfigGitHubValidateRejectsNonHTTPS(t *testing.T) {
	cfg := DefaultConfig()
	cfg.GitHub.APIURL = "http://insecure.example.com"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected non-https github.api_url to be rejected")
	} else if !strings.Contains(err.Error(), "github.api_url") {
		t.Errorf("error should name github.api_url, got: %v", err)
	}

	// Empty api_url is valid (env/default precedence applies at the call site).
	cfg.GitHub.APIURL = ""
	if err := cfg.Validate(); err != nil {
		t.Errorf("empty github.api_url should be valid, got: %v", err)
	}
}

// TestConfigSetSubagentModelScalar asserts the scalar `set` path reaches the new
// nested string leaf (model) but rejects the []string leaves as file-only, matching
// the existing context.languages precedent.
func TestConfigSetSubagentModelScalar(t *testing.T) {
	cfg := DefaultConfig()
	updated, err := ConfigSetValue(cfg, "subagents.review.model", "fast-review")
	if err != nil {
		t.Fatalf("set subagents.review.model: %v", err)
	}
	if updated.Subagents.Review.Model != "fast-review" {
		t.Errorf("subagents.review.model = %q, want fast-review", updated.Subagents.Review.Model)
	}
	if _, err := ConfigSetValue(cfg, "subagents.default.allowed_skills", "a,b"); err == nil {
		t.Error("expected []string leaf to be rejected by scalar set path")
	}
}
