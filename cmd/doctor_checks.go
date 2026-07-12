package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/adapter"
)

const (
	doctorCommandTimeout = 5 * time.Second
	doctorOutputLimit    = 64 << 10
)

type doctorOutput struct {
	ContractVersion int                   `json:"contract_version"`
	Checks          []adapter.DoctorCheck `json:"checks"`
}

type doctorCommandRunner interface {
	LookPath(string) (string, error)
	Run(context.Context, string, ...string) ([]byte, error)
}

type systemDoctorRunner struct{}

type boundedDoctorBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (systemDoctorRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func (systemDoctorRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	base := strings.ToLower(filepath.Base(name))
	if base != "gh" && base != "gh.exe" && base != "git" && base != "git.exe" {
		return nil, fmt.Errorf("doctor executable %q is not allowed", name)
	}
	command := exec.CommandContext(ctx, name, args...) // #nosec G204 -- executable is restricted above to gh or git and arguments are passed without a shell.
	stdout := &boundedDoctorBuffer{limit: doctorOutputLimit}
	command.Stdout = stdout
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		return stdout.buffer.Bytes(), err
	}
	return stdout.buffer.Bytes(), nil
}

func (writer *boundedDoctorBuffer) Write(data []byte) (int, error) {
	remaining := writer.limit - writer.buffer.Len()
	if remaining > len(data) {
		remaining = len(data)
	}
	if remaining > 0 {
		_, _ = writer.buffer.Write(data[:remaining])
	}
	return len(data), nil
}

func collectDoctorReport(ctx context.Context, root string, runner doctorCommandRunner) (adapter.DoctorReport, error) {
	report, err := adapter.Doctor(root)
	if err != nil {
		return adapter.DoctorReport{}, err
	}
	report.Checks = append(report.Checks, githubDoctorChecks(ctx, root, runner)...)
	return report, nil
}

func makeDoctorOutput(report adapter.DoctorReport) doctorOutput {
	return doctorOutput{
		ContractVersion: machineContractVersion,
		Checks:          append([]adapter.DoctorCheck{}, report.Checks...),
	}
}

func runDoctorCommand(ctx context.Context, runner doctorCommandRunner, name string, args ...string) ([]byte, error) {
	bounded, cancel := context.WithTimeout(ctx, doctorCommandTimeout)
	defer cancel()
	return runner.Run(bounded, name, args...)
}

func githubDoctorChecks(ctx context.Context, root string, runner doctorCommandRunner) []adapter.DoctorCheck {
	executable, err := runner.LookPath("gh")
	if err != nil {
		return []adapter.DoctorCheck{doctorCheck(
			"github_cli_unavailable", "warning", "github_cli",
			"GitHub CLI is unavailable; issue-backed workflows require gh or an official REST client.",
		)}
	}

	checks := make([]adapter.DoctorCheck, 0, 3)
	versionOutput, versionErr := runDoctorCommand(ctx, runner, executable, "--version")
	version, versionOK := parseGHVersion(versionOutput)
	switch {
	case versionErr != nil || !versionOK:
		checks = append(checks, doctorCheck(
			"github_cli_version_unknown", "warning", "github_cli",
			"GitHub CLI version could not be verified; compatibility with issue relationship operations is unknown.",
		))
	case version.less(ghVersion{major: 2, minor: 94, patch: 0}):
		checks = append(checks, doctorCheck(
			"github_cli_rest_fallback_required", "warning", "github_cli",
			fmt.Sprintf("GitHub CLI %s is older than 2.94.0; the official REST fallback is required for parent/sub-issue/dependency operations.", version),
		))
	default:
		checks = append(checks, doctorCheck(
			"github_cli_compatible", "ok", "github_cli",
			fmt.Sprintf("GitHub CLI %s supports required issue relationship operations.", version),
		))
	}

	_, authErr := runDoctorCommand(ctx, runner, executable, "auth", "status", "--hostname", "github.com")
	if authErr != nil {
		checks = append(checks, doctorCheck(
			"github_auth_unavailable", "warning", "github_auth",
			"GitHub authentication for github.com is unavailable; authenticate before issue-backed operations.",
		))
		return checks
	}
	checks = append(checks, doctorCheck(
		"github_auth_available", "ok", "github_auth",
		"GitHub authentication for github.com is available.",
	))

	remoteOutput, remoteErr := runDoctorCommand(ctx, runner, "git", "-C", root, "remote", "get-url", "origin")
	repository, identified := githubRepositoryFromRemote(remoteOutput)
	if remoteErr != nil || !identified {
		return checks
	}
	permissionsOutput, permissionsErr := runDoctorCommand(
		ctx,
		runner,
		executable,
		"api",
		"--hostname",
		"github.com",
		"repos/"+repository,
		"--jq",
		".permissions",
	)
	if permissionsErr != nil {
		checks = append(checks, githubPermissionsUnknownCheck())
		return checks
	}

	var permissions map[string]bool
	if err := json.Unmarshal(permissionsOutput, &permissions); err != nil || permissions == nil {
		checks = append(checks, githubPermissionsUnknownCheck())
		return checks
	}
	read := permissions["pull"]
	write := permissions["push"] || permissions["maintain"] || permissions["admin"]
	triage := permissions["triage"] || write
	detail := fmt.Sprintf("GitHub issue permissions for %s: read=%t triage=%t write=%t.", repository, read, triage, write)
	if read && triage && write {
		checks = append(checks, doctorCheck("github_issue_permissions_ok", "ok", "github_permissions", detail))
	} else {
		checks = append(checks, doctorCheck("github_issue_permissions_limited", "warning", "github_permissions", detail))
	}
	return checks
}

func githubPermissionsUnknownCheck() adapter.DoctorCheck {
	return doctorCheck(
		"github_issue_permissions_unknown", "warning", "github_permissions",
		"GitHub issue permissions could not be verified; retry when the network and API are available.",
	)
}

func doctorCheck(code, status, name, detail string) adapter.DoctorCheck {
	return adapter.DoctorCheck{Code: code, Status: status, HostID: "-", Name: name, Detail: detail}
}

type ghVersion struct {
	major int
	minor int
	patch int
}

func parseGHVersion(output []byte) (ghVersion, bool) {
	for _, field := range strings.Fields(string(output)) {
		candidate := strings.TrimPrefix(field, "v")
		candidate = strings.TrimRight(candidate, ",;)")
		parts := strings.Split(candidate, ".")
		if len(parts) != 3 {
			continue
		}
		values := [3]int{}
		valid := true
		for index, part := range parts {
			value, err := strconv.Atoi(part)
			if err != nil || value < 0 {
				valid = false
				break
			}
			values[index] = value
		}
		if valid {
			return ghVersion{major: values[0], minor: values[1], patch: values[2]}, true
		}
	}
	return ghVersion{}, false
}

func (version ghVersion) less(other ghVersion) bool {
	if version.major != other.major {
		return version.major < other.major
	}
	if version.minor != other.minor {
		return version.minor < other.minor
	}
	return version.patch < other.patch
}

func (version ghVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", version.major, version.minor, version.patch)
}

func githubRepositoryFromRemote(raw []byte) (string, bool) {
	remote := strings.TrimSpace(string(raw))
	if remote == "" || strings.ContainsAny(remote, "\r\n\x00") || strings.Contains(remote, "%") {
		return "", false
	}
	if strings.HasPrefix(remote, "git@github.com:") {
		return normalizeGitHubRepository(strings.TrimPrefix(remote, "git@github.com:"))
	}

	parsed, err := url.Parse(remote)
	if err != nil || parsed.RawQuery != "" || parsed.Fragment != "" || !strings.EqualFold(parsed.Hostname(), "github.com") {
		return "", false
	}
	switch parsed.Scheme {
	case "https":
		if parsed.User != nil {
			return "", false
		}
	case "ssh":
		if parsed.User != nil {
			_, hasPassword := parsed.User.Password()
			if parsed.User.Username() != "git" || hasPassword {
				return "", false
			}
		}
	default:
		return "", false
	}
	return normalizeGitHubRepository(strings.TrimPrefix(parsed.Path, "/"))
}

func normalizeGitHubRepository(repository string) (string, bool) {
	repository = strings.TrimSuffix(repository, ".git")
	parts := strings.Split(repository, "/")
	if len(parts) != 2 || !validGitHubSlug(parts[0]) || !validGitHubSlug(parts[1]) {
		return "", false
	}
	return parts[0] + "/" + parts[1], true
}

func validGitHubSlug(value string) bool {
	if value == "" || value == "." || value == ".." || path.Clean(value) != value {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}
