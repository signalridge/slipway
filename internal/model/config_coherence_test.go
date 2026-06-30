package model

import (
	"reflect"
	"strings"
	"testing"
)

func stringList(values ...string) *[]string {
	out := append([]string(nil), values...)
	return &out
}

func stringListValue(in *[]string) []string {
	if in == nil {
		return nil
	}
	return *in
}

func TestConfigSubagentsResolveFallsBackToDefault(t *testing.T) {
	sa := ConfigSubagents{
		Default: SubagentProfile{
			Model:             "default-model",
			AllowedSkills:     stringList("skill-a"),
			AllowedMCPServers: stringList("mcp-a"),
		},
		Review: SubagentProfile{
			Model: "review-model",
		},
		Executor: SubagentProfile{
			AllowedMCPServers: stringList("executor-mcp"),
		},
		Fix: SubagentProfile{
			AllowedSkills: stringList("fix-skill"),
		},
	}

	// Review overrides only Model; skills/MCP fall back to Default.
	review := sa.Resolve(SubagentStageReview)
	if review.Model != "review-model" {
		t.Errorf("review model = %q, want review-model", review.Model)
	}
	if !reflect.DeepEqual(stringListValue(review.AllowedSkills), []string{"skill-a"}) {
		t.Errorf("review skills = %v, want default [skill-a]", review.AllowedSkills)
	}
	if !reflect.DeepEqual(stringListValue(review.AllowedMCPServers), []string{"mcp-a"}) {
		t.Errorf("review mcp = %v, want default [mcp-a]", review.AllowedMCPServers)
	}

	// Executor overrides only MCP; model/skills fall back to Default.
	executor := sa.Resolve(SubagentStageExecutor)
	if executor.Model != "default-model" {
		t.Errorf("executor model = %q, want default-model", executor.Model)
	}
	if !reflect.DeepEqual(stringListValue(executor.AllowedSkills), []string{"skill-a"}) {
		t.Errorf("executor skills = %v, want default [skill-a]", executor.AllowedSkills)
	}
	if !reflect.DeepEqual(stringListValue(executor.AllowedMCPServers), []string{"executor-mcp"}) {
		t.Errorf("executor mcp = %v, want [executor-mcp]", executor.AllowedMCPServers)
	}

	// Fix overrides only skills; model/MCP fall back to Default.
	fix := sa.Resolve(SubagentStageFix)
	if fix.Model != "default-model" {
		t.Errorf("fix model = %q, want default-model", fix.Model)
	}
	if !reflect.DeepEqual(stringListValue(fix.AllowedSkills), []string{"fix-skill"}) {
		t.Errorf("fix skills = %v, want [fix-skill]", fix.AllowedSkills)
	}

	// Verify has no override; entirely Default.
	verify := sa.Resolve(SubagentStageVerify)
	if verify.Model != "default-model" || !reflect.DeepEqual(stringListValue(verify.AllowedSkills), []string{"skill-a"}) {
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

func TestConfigSubagentsResolveExplicitEmptyListsOverrideDefault(t *testing.T) {
	sa := ConfigSubagents{
		Default: SubagentProfile{
			AllowedSkills:     stringList("default-skill"),
			AllowedMCPServers: stringList("default-mcp"),
		},
		Review: SubagentProfile{
			AllowedSkills:     stringList(),
			AllowedMCPServers: stringList(),
		},
	}

	got := sa.Resolve(SubagentStageReview)
	if got.AllowedSkills == nil || len(*got.AllowedSkills) != 0 {
		t.Errorf("review skills = %v, want explicit empty list", got.AllowedSkills)
	}
	if got.AllowedMCPServers == nil || len(*got.AllowedMCPServers) != 0 {
		t.Errorf("review MCP servers = %v, want explicit empty list", got.AllowedMCPServers)
	}
}

func TestConfigSubagentsResolvePerReviewerFallsBackThroughReview(t *testing.T) {
	sa := ConfigSubagents{
		Default: SubagentProfile{
			Model:             "default-model",
			AllowedSkills:     stringList("default-skill"),
			AllowedMCPServers: stringList("default-mcp"),
		},
		Review: SubagentProfile{
			Model:             "review-model",
			AllowedSkills:     stringList("review-skill"),
			AllowedMCPServers: stringList("review-mcp"),
		},
		SecurityReview: SubagentProfile{
			Model:             "security-model",
			AllowedMCPServers: stringList("security-mcp"),
		},
		CodeQualityReview: SubagentProfile{
			AllowedSkills: stringList(),
		},
	}

	security := sa.Resolve(SubagentStageSecurityReview)
	if security.Model != "security-model" {
		t.Errorf("security model = %q, want security-model", security.Model)
	}
	if !reflect.DeepEqual(stringListValue(security.AllowedSkills), []string{"review-skill"}) {
		t.Errorf("security skills = %v, want review fallback", security.AllowedSkills)
	}
	if !reflect.DeepEqual(stringListValue(security.AllowedMCPServers), []string{"security-mcp"}) {
		t.Errorf("security MCP servers = %v, want per-reviewer override", security.AllowedMCPServers)
	}

	codeQuality := sa.Resolve(SubagentStageCodeQualityReview)
	if codeQuality.Model != "review-model" {
		t.Errorf("code-quality model = %q, want review fallback", codeQuality.Model)
	}
	if codeQuality.AllowedSkills == nil || len(*codeQuality.AllowedSkills) != 0 {
		t.Errorf("code-quality skills = %v, want explicit empty list", codeQuality.AllowedSkills)
	}
	if !reflect.DeepEqual(stringListValue(codeQuality.AllowedMCPServers), []string{"review-mcp"}) {
		t.Errorf("code-quality MCP servers = %v, want review fallback", codeQuality.AllowedMCPServers)
	}
}

func TestConfigSubagentsIsZero(t *testing.T) {
	if !(ConfigSubagents{}).IsZero() {
		t.Error("empty ConfigSubagents should be zero")
	}
	if (ConfigSubagents{Verify: SubagentProfile{Model: "x"}}).IsZero() {
		t.Error("ConfigSubagents with a verify model must not be zero")
	}
	if (ConfigSubagents{Executor: SubagentProfile{Model: "x"}}).IsZero() {
		t.Error("ConfigSubagents with an executor model must not be zero")
	}
}

func TestConfigSubagentsYAMLRoundTrip(t *testing.T) {
	in := []byte("subagents:\n  default:\n    model: fast\n  executor:\n    allowed_skills:\n      - coding-discipline\n  review:\n    allowed_skills:\n      - code-quality-review\n    allowed_mcp_servers:\n      - serena\n")
	cfg, err := ParseConfigYAML(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Subagents.Default.Model != "fast" {
		t.Errorf("default.model = %q, want fast", cfg.Subagents.Default.Model)
	}
	if !reflect.DeepEqual(stringListValue(cfg.Subagents.Executor.AllowedSkills), []string{"coding-discipline"}) {
		t.Errorf("executor.allowed_skills = %v, want [coding-discipline]", cfg.Subagents.Executor.AllowedSkills)
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

func TestConfigSubagentsResolveCoversEveryStageProfileField(t *testing.T) {
	typ := reflect.TypeOf(ConfigSubagents{})
	val := reflect.ValueOf(&ConfigSubagents{}).Elem()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Name == "Default" {
			continue
		}
		stage := SubagentStage(strings.TrimSuffix(field.Tag.Get("yaml"), ",omitempty"))
		if stage == "" {
			t.Fatalf("subagent field %s has no yaml stage tag", field.Name)
		}
		wantModel := string(stage) + "-model"
		val.Field(i).Set(reflect.ValueOf(SubagentProfile{Model: wantModel}))
		cfg := val.Interface().(ConfigSubagents)
		got := cfg.Resolve(stage)
		if got.Model != wantModel {
			t.Errorf("Resolve(%q).Model = %q, want %q (field %s is not covered)", stage, got.Model, wantModel, field.Name)
		}
		val.Field(i).Set(reflect.Zero(field.Type))
	}
}

func TestConfigSubagentsExplicitEmptyListRoundTripStaysBare(t *testing.T) {
	in := []byte("subagents:\n  default:\n    allowed_skills:\n      - default-skill\n  review:\n    allowed_skills: []\n")
	cfg, err := ParseConfigYAML(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := cfg.Subagents.Resolve(SubagentStageReview).AllowedSkills; got == nil || len(*got) != 0 {
		t.Fatalf("pre-round-trip review skills = %v, want explicit empty list", got)
	}

	out, err := cfg.ToYAML()
	if err != nil {
		t.Fatalf("toYAML: %v", err)
	}
	back, err := ParseConfigYAML(out)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if got := back.Subagents.Resolve(SubagentStageReview).AllowedSkills; got == nil || len(*got) != 0 {
		t.Errorf("post-round-trip review skills = %v, want explicit empty list", got)
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

func TestConfigGitHubValidateRejectsRuntimeUnsafeURLs(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "userinfo", url: "https://token@ghe.example.com/api/v3"},
		{name: "query", url: "https://ghe.example.com/api/v3?token=secret"},
		{name: "fragment", url: "https://ghe.example.com/api/v3#token"},
		{name: "noncanonical path", url: "https://ghe.example.com/api//v3"},
		{name: "encoded path", url: "https://ghe.example.com/api%2Fv3"},
		{name: "public host path", url: "https://api.github.com/api/v3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.GitHub.APIURL = tt.url
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("expected github.api_url %q to be rejected", tt.url)
			}
			if !strings.Contains(err.Error(), "github.api_url") {
				t.Errorf("error should name github.api_url, got: %v", err)
			}
		})
	}
}

func TestConfigGitHubValidateRejectsInvalidAllowedBaseURLs(t *testing.T) {
	cfg := DefaultConfig()
	cfg.GitHub.APIURL = "https://ghe.example.com/api/v3"
	cfg.GitHub.APIAllowedBaseURLs = []string{
		"https://ghe.example.com/api/v3",
		"httpss://typo.example.com",
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected invalid github.api_allowed_base_urls entry to be rejected")
	}
	if !strings.Contains(err.Error(), "github.api_allowed_base_urls[1]") {
		t.Errorf("error should name invalid allowlist index, got: %v", err)
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
