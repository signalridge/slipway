package capability

import (
	"slices"
)

// ValidModesForCommand returns the sorted list of skill IDs that may be
// selected via `--mode` for the given command. It includes any skill whose
// bindings target the command with a routable binding type (command-auto,
// command-manual). Used by review / validate / repair CLI layers to
// validate user input.
func ValidModesForCommand(reg *Registry, command string) []string {
	if reg == nil || command == "" {
		return nil
	}
	out := make([]string, 0, reg.Len())
	for _, sk := range reg.All() {
		if skillHasRoutableBinding(sk, command) {
			out = append(out, sk.ID)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// ValidViewsForCommand returns the sorted list of skill IDs that may be
// selected via `--view` for the given status/health command. It includes any
// skill whose bindings target the command with a command-view or
// command-manual binding (status/health views are read-only surfaces).
func ValidViewsForCommand(reg *Registry, command string) []string {
	if reg == nil || command == "" {
		return nil
	}
	out := make([]string, 0, reg.Len())
	for _, sk := range reg.All() {
		if skillHasViewBinding(sk, command) {
			out = append(out, sk.ID)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func skillHasRoutableBinding(sk Skill, command string) bool {
	for _, b := range sk.Bindings {
		switch b.Type {
		case BindingCommandAuto, BindingCommandManual:
			if bindingMatchesCommand(b, command) {
				return true
			}
		}
	}
	return false
}

func skillHasViewBinding(sk Skill, command string) bool {
	for _, b := range sk.Bindings {
		switch b.Type {
		case BindingCommandView, BindingCommandManual:
			if bindingMatchesCommand(b, command) {
				return true
			}
		}
	}
	return false
}
