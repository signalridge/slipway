package model

import (
	"reflect"
	"strings"
	"testing"
)

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
