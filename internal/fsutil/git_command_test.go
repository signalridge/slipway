package fsutil

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestGitCommandContextIsolatesRepositoryLocalEnvironment(t *testing.T) {
	blocked := []string{
		"GIT_DIR",
		"GIT_WORK_TREE",
		"GIT_IMPLICIT_WORK_TREE",
		"GIT_COMMON_DIR",
		"GIT_INDEX_FILE",
		"GIT_INDEX_VERSION",
		"GIT_OBJECT_DIRECTORY",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES",
		"GIT_QUARANTINE_PATH",
		"GIT_NAMESPACE",
		"GIT_REPLACE_REF_BASE",
		"GIT_NO_REPLACE_OBJECTS",
		"GIT_SHALLOW_FILE",
		"GIT_GRAFT_FILE",
		"GIT_CEILING_DIRECTORIES",
		"GIT_DISCOVERY_ACROSS_FILESYSTEM",
		"GIT_CONFIG",
		"GIT_CONFIG_PARAMETERS",
		"GIT_CONFIG_COUNT",
		"GIT_CONFIG_KEY_0",
		"GIT_CONFIG_VALUE_0",
		"GIT_PREFIX",
	}
	for _, name := range blocked {
		t.Setenv(name, "poisoned")
	}
	t.Setenv("GIT_OPTIONAL_LOCKS", "1")
	t.Setenv("SLIPWAY_GIT_ENV_TEST", "preserved")

	command := GitCommandContext(context.Background(), "git", "-C", "/repository", "status", "--short")
	wantArgs := []string{"git", "-c", "core.fsmonitor=false", "-C", "/repository", "status", "--short"}
	if !reflect.DeepEqual(command.Args, wantArgs) {
		t.Fatalf("command args = %#v, want %#v", command.Args, wantArgs)
	}

	environment := make(map[string]string, len(command.Env))
	for _, entry := range command.Env {
		name, value, found := strings.Cut(entry, "=")
		if found {
			environment[strings.ToUpper(name)] = value
		}
	}
	for _, name := range blocked {
		if _, found := environment[name]; found {
			t.Errorf("%s unexpectedly survived Git environment isolation", name)
		}
	}
	if got := environment["GIT_OPTIONAL_LOCKS"]; got != "0" {
		t.Errorf("GIT_OPTIONAL_LOCKS = %q, want 0", got)
	}
	if got := environment["SLIPWAY_GIT_ENV_TEST"]; got != "preserved" {
		t.Errorf("ordinary environment = %q, want preserved", got)
	}
}
