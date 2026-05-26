package state

import (
	"os"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNeedsDiscoveryPersistsAcrossSaveLoad(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	change := model.NewChange("discovery-change")
	change.NeedsDiscovery = true
	require.NoError(t, SaveChange(root, change))

	loaded, err := LoadChange(root, "discovery-change")
	require.NoError(t, err)
	assert.True(t, loaded.NeedsDiscovery)
}

func TestRepairCorruptConfigBacksUpAndRewritesDefaults(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	configPath := ConfigPath(root)
	require.NoError(t, os.WriteFile(configPath, []byte("defaults: ["), 0o644))

	now := time.Date(2026, 3, 4, 1, 2, 3, 0, time.UTC)
	backupPath, err := RepairCorruptConfig(root, now)
	require.NoError(t, err)

	_, err = os.Stat(backupPath)
	require.NoError(t, err)

	cfg, err := model.LoadConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, model.ArtifactSchemaExpanded, cfg.Defaults.ArtifactSchema)
}
