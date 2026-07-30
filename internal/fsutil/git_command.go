package fsutil

import (
	"context"
	"os/exec"
	"strings"
)

// GitCommandContext constructs a Git subprocess whose repository identity
// comes from its arguments and on-disk metadata, not inherited repository-local
// environment. Ordinary environment variables and Git configuration files are
// intentionally preserved.
func GitCommandContext(ctx context.Context, executable string, args ...string) *exec.Cmd {
	commandArgs := make([]string, 0, len(args)+2)
	commandArgs = append(commandArgs, "-c", "core.fsmonitor=false")
	commandArgs = append(commandArgs, args...)
	command := exec.CommandContext(ctx, executable, commandArgs...) // #nosec G204 -- internal callers use the fixed Git executable or validate its base name before calling.
	command.Env = gitCommandEnvironment(command.Environ())
	return command
}

func gitCommandEnvironment(environment []string) []string {
	sanitized := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		name, _, found := strings.Cut(entry, "=")
		if !found || gitEnvironmentVariableIsRepositoryLocal(name) {
			continue
		}
		sanitized = append(sanitized, entry)
	}
	return append(sanitized, "GIT_OPTIONAL_LOCKS=0")
}

func gitEnvironmentVariableIsRepositoryLocal(name string) bool {
	name = strings.ToUpper(name)
	switch name {
	case "GIT_DIR",
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
		"GIT_PREFIX",
		"GIT_OPTIONAL_LOCKS":
		return true
	default:
		return strings.HasPrefix(name, "GIT_CONFIG_KEY_") ||
			strings.HasPrefix(name, "GIT_CONFIG_VALUE_")
	}
}
