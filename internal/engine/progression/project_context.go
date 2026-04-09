package progression

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

// InferProjectContext scans the repository at repoRoot to infer project metadata.
// Returns a best-effort model.ProjectContext; empty fields on detection failure.
// Config values from .slipway.yaml take precedence over inferred values.
func InferProjectContext(repoRoot string) model.ProjectContext {
	ctx := model.ProjectContext{}

	// Detect tech stack and languages from lockfiles
	detectTechStack(repoRoot, &ctx)

	// Detect test/build commands
	detectCommands(repoRoot, &ctx)

	// Read conventions from CLAUDE.md (first 50 lines)
	detectConventions(repoRoot, &ctx)

	// Git recent work
	detectRecentWork(repoRoot, &ctx)

	// Merge with config: config values override inferred values
	cfgPath := state.ConfigPath(repoRoot)
	if cfg, err := model.LoadConfig(cfgPath); err == nil {
		mergeConfigOverInferred(&ctx, cfg.Context)
	}

	return ctx
}

// mergeConfigOverInferred applies explicit config values over inferred ones.
func mergeConfigOverInferred(inferred *model.ProjectContext, configured model.ProjectContext) {
	if configured.TechStack != "" {
		inferred.TechStack = configured.TechStack
	}
	if configured.Conventions != "" {
		inferred.Conventions = configured.Conventions
	}
	if configured.TestCmd != "" {
		inferred.TestCmd = configured.TestCmd
	}
	if configured.BuildCmd != "" {
		inferred.BuildCmd = configured.BuildCmd
	}
	if len(configured.Languages) > 0 {
		inferred.Languages = configured.Languages
	}
	if configured.RecentWork != "" {
		inferred.RecentWork = configured.RecentWork
	}
}

func detectTechStack(root string, ctx *model.ProjectContext) {
	var stacks []string
	var langs []string

	if fileExists(filepath.Join(root, "go.mod")) {
		stacks = append(stacks, "Go")
		langs = append(langs, "Go")
	}
	if fileExists(filepath.Join(root, "package.json")) || fileExists(filepath.Join(root, "package-lock.json")) {
		stacks = append(stacks, "Node.js")
		// Check for TypeScript
		if fileExists(filepath.Join(root, "tsconfig.json")) {
			langs = append(langs, "TypeScript")
		} else {
			langs = append(langs, "JavaScript")
		}
	}
	if fileExists(filepath.Join(root, "Cargo.toml")) {
		stacks = append(stacks, "Rust")
		langs = append(langs, "Rust")
	}
	if fileExists(filepath.Join(root, "pyproject.toml")) || fileExists(filepath.Join(root, "requirements.txt")) {
		stacks = append(stacks, "Python")
		langs = append(langs, "Python")
	}

	ctx.TechStack = strings.Join(stacks, ", ")
	ctx.Languages = langs
}

func detectCommands(root string, ctx *model.ProjectContext) {
	// Check package.json scripts
	pkgPath := filepath.Join(root, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		var pkg struct {
			Scripts map[string]string `json:"scripts"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			if cmd, ok := pkg.Scripts["test"]; ok {
				ctx.TestCmd = cmd
			}
			if cmd, ok := pkg.Scripts["build"]; ok {
				ctx.BuildCmd = cmd
			}
		}
	}

	// Check Makefile targets
	makePath := filepath.Join(root, "Makefile")
	if data, err := os.ReadFile(makePath); err == nil {
		content := string(data)
		if ctx.TestCmd == "" && strings.Contains(content, "test:") {
			ctx.TestCmd = "make test"
		}
		if ctx.BuildCmd == "" && strings.Contains(content, "build:") {
			ctx.BuildCmd = "make build"
		}
	}

	// Go defaults
	if fileExists(filepath.Join(root, "go.mod")) {
		if ctx.TestCmd == "" {
			ctx.TestCmd = "go test ./..."
		}
		if ctx.BuildCmd == "" {
			ctx.BuildCmd = "go build ./..."
		}
	}
}

func detectConventions(root string, ctx *model.ProjectContext) {
	claudePath := filepath.Join(root, "CLAUDE.md")
	data, err := os.ReadFile(claudePath)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	limit := 50
	if len(lines) < limit {
		limit = len(lines)
	}
	excerpt := strings.TrimSpace(strings.Join(lines[:limit], "\n"))
	if excerpt != "" {
		// Truncate to a reasonable size for context
		if len(excerpt) > 500 {
			excerpt = excerpt[:500] + "..."
		}
		ctx.Conventions = excerpt
	}
}

func detectRecentWork(root string, ctx *model.ProjectContext) {
	cmd := exec.Command("git", "log", "--oneline", "-5")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return
	}
	result := strings.TrimSpace(string(out))
	if result != "" {
		ctx.RecentWork = result
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
