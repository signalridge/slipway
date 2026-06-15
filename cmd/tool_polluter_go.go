package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func makeFindPolluterGoCmd() *cobra.Command {
	var runFilter string
	cmd := &cobra.Command{
		Use:   "find-polluter-go <pollution-path> <package-glob>",
		Short: "Find the Go test package that creates a pollution path",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return newInvalidUsageError("find_polluter_go_usage", "Usage: slipway tool find-polluter-go <pollution-path> <package-glob> [-run <regex>]", "Pass a pollution path and a Go package selector.", nil)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFindPolluterGo(cmd, args[0], args[1], runFilter)
		},
	}
	cmd.Flags().StringVar(&runFilter, "run", "", "Optional go test -run regex")
	return cmd
}

func runFindPolluterGo(cmd *cobra.Command, pollutionPath, packageGlob, runFilter string) error {
	if _, err := exec.LookPath("go"); err != nil {
		return newPreconditionError("find_polluter_go_missing_go", "find-polluter-go: go toolchain not found on PATH", "Install Go or run this helper in an environment with go on PATH.", "", nil)
	}
	if _, err := os.Stat(pollutionPath); err == nil {
		return newPreconditionError("find_polluter_go_pollution_present", fmt.Sprintf("find-polluter-go: pollution already present at %s; clean before bisecting", pollutionPath), "Remove the pollution path and retry.", "", map[string]any{"path": pollutionPath})
	}

	pkgs, listOutput, listErr := listGoTestPackages(packageGlob)
	if listOutput != "" {
		fmt.Fprint(cmd.ErrOrStderr(), listOutput)
		if !strings.HasSuffix(listOutput, "\n") {
			fmt.Fprintln(cmd.ErrOrStderr())
		}
	}
	if listErr != nil {
		return newPreconditionError("find_polluter_go_list_failed", fmt.Sprintf("find-polluter-go: go list failed for %s", packageGlob), strings.TrimSpace(listErr.Error()), "", nil)
	}
	if len(pkgs) == 0 {
		return newPreconditionError("find_polluter_go_no_tests", fmt.Sprintf("find-polluter-go: no test packages found under %s", packageGlob), "Point the helper at a selector containing Go tests.", "", nil)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "find-polluter-go: checking %d package(s) for polluter %s\n", len(pkgs), pollutionPath)
	args := []string{"test", "-count=1"}
	if strings.TrimSpace(runFilter) != "" {
		args = append(args, "-run", runFilter)
	}
	for _, pkg := range pkgs {
		fmt.Fprintf(cmd.OutOrStdout(), "-- go %s %s\n", strings.Join(args, " "), pkg)
		testArgs := append(append([]string{}, args...), pkg)
		testCmd := exec.Command("go", testArgs...) // #nosec G204 -- package selector comes from explicit helper input; no shell interpolation.
		_ = testCmd.Run()
		if _, err := os.Stat(pollutionPath); err == nil {
			fmt.Fprintf(cmd.OutOrStdout(), "\nPOLLUTER: %s created %s\n", pkg, pollutionPath)
			repro := "go test -count=1 "
			if strings.TrimSpace(runFilter) != "" {
				repro += "-run " + runFilter + " "
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Reproduce with: %s-v %s\n", repro, pkg)
			return newPreconditionError("find_polluter_go_polluter_found", fmt.Sprintf("%s created %s", pkg, pollutionPath), "Inspect the package and remove the shared-state mutation.", "", map[string]any{"package": pkg})
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "find-polluter-go: no polluter found across %d package(s)\n", len(pkgs))
	return nil
}

func listGoTestPackages(packageGlob string) ([]string, string, error) {
	goList := exec.Command("go", "list", "-f", "{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}", packageGlob) // #nosec G204 -- package selector comes from explicit helper input; no shell interpolation.
	var stderr bytes.Buffer
	goList.Stderr = &stderr
	out, err := goList.Output()
	if err != nil {
		combined := strings.TrimSpace(stderr.String())
		if strings.TrimSpace(string(out)) != "" {
			combined += "\n" + strings.TrimSpace(string(out))
		}
		return nil, "", fmt.Errorf("%s", strings.TrimSpace(combined))
	}
	var pkgs []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			pkgs = append(pkgs, line)
		}
	}
	sort.Strings(pkgs)
	return pkgs, stderr.String(), nil
}
