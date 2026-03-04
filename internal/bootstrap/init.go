package bootstrap

import (
	"os"
	"path/filepath"

	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/toolgen"
)

func InitWorkspace(root string, tools []string, refresh bool) error {
	dirs := []string{
		filepath.Join(root, ".spln"),
		filepath.Join(root, ".spln", "runtime"),
		filepath.Join(root, ".spln", "runtime", "admissions"),
		filepath.Join(root, ".spln", "runtime", "changes"),
		filepath.Join(root, ".spln", "archive"),
		filepath.Join(root, ".spln", "archive", "admissions"),
		filepath.Join(root, ".spln", "archive", "changes"),
		filepath.Join(root, ".spln", "archive", "config"),
		filepath.Join(root, ".spln", "evidence"),
		filepath.Join(root, ".spln", "evidence", "skills"),
		filepath.Join(root, ".spln", "evidence", "tasks"),
		filepath.Join(root, ".spln", "evidence", "runs"),
		filepath.Join(root, "aircraft"),
		filepath.Join(root, "aircraft", "changes"),
		filepath.Join(root, "aircraft", "changes", "archived"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	configPath := filepath.Join(root, ".spln", "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := model.SaveConfig(configPath, model.DefaultConfig()); err != nil {
			return err
		}
	}

	if len(tools) == 0 {
		return nil
	}
	return toolgen.Generate(root, tools, refresh)
}
