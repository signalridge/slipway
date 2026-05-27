package governance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func writeVerificationForTest(t *testing.T, root, slug, skillName string, rec model.VerificationRecord) {
	t.Helper()

	rec.Normalize()
	require.NoError(t, rec.Validate())

	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	paths, err := state.ResolveChangePaths(root, change)
	require.NoError(t, err)

	dir := filepath.Join(paths.GovernedBundleDir, "verification")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	raw, err := yaml.Marshal(rec)
	require.NoError(t, err)
	require.NoError(t, fsutil.WriteFileAtomic(filepath.Join(dir, skillName+".yaml"), raw, 0o644))
}
