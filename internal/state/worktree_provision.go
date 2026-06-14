package state

// WorktreeProvisioner materializes host-adapter surfaces (skills, hooks,
// settings, references) into a worktree so an agent working there — including an
// isolated subagent — has the same host surface as the main checkout.
//
// The concrete implementation lives in the surface-renderer layer
// (internal/toolgen.ProvisionWorktreeHostSurfaces) and is injected by the
// composition root (cmd) so that this authority package never imports a surface
// renderer (enforced by internal/architecture). A nil provisioner is a no-op,
// which keeps pure state unit tests free of the rendering dependency.
type WorktreeProvisioner func(repoRoot, worktreeRoot string) error

// provision invokes the provisioner, treating a nil provisioner as a no-op.
func (p WorktreeProvisioner) provision(repoRoot, worktreeRoot string) error {
	if p == nil {
		return nil
	}
	return p(repoRoot, worktreeRoot)
}
