package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/signalridge/slipway/internal/autopilot"
)

func commandNext(workspace string, operation autopilot.NextOperation, variantID string, argv []string, inputs []autopilot.NextInput) autopilot.Next {
	next, err := autopilot.NewCommandNext(operation, workspace, variantID, argv, inputs)
	if err != nil {
		return autopilot.NoneNext(workspace)
	}
	return next
}

func inputlessCommandNext(workspace, variantID string, argv ...string) autopilot.Next {
	return commandNext(workspace, autopilot.NextOperationCommand, variantID, argv, []autopilot.NextInput{})
}

func unavailableWorkspaceCommandNext(workspace, variantID string, argv ...string) autopilot.Next {
	next, err := autopilot.NewUnavailableWorkspaceCommandNext(workspace, variantID, argv, []autopilot.NextInput{})
	if err != nil {
		return autopilot.NoneNext(workspace)
	}
	return next
}

func writeHumanNext(writer io.Writer, next autopilot.Next) error {
	if err := next.Validate(); err != nil {
		return fmt.Errorf("render next: %w", err)
	}
	if next.Operation == autopilot.NextOperationNone {
		return nil
	}
	if _, err := fmt.Fprintln(writer, "Next choices:"); err != nil {
		return err
	}
	for _, variant := range next.Variants {
		missing := make([]string, 0, len(variant.Inputs))
		for _, input := range variant.Inputs {
			if input.Required {
				description := fmt.Sprintf("%s (%s via %s)", input.Name, input.Type, input.Flag)
				if input.Type == autopilot.NextInputEnum {
					description += " choices=" + strings.Join(input.Choices, "|")
				}
				missing = append(missing, description)
			}
		}
		if len(missing) > 0 {
			if _, err := fmt.Fprintf(writer, "- %s: requires %s\n", variant.ID, strings.Join(missing, ", ")); err != nil {
				return err
			}
			continue
		}
		argv, err := next.Resolve(variant.ID, map[string]autopilot.NextInputValue{})
		if err != nil {
			return fmt.Errorf("resolve next variant %s: %w", variant.ID, err)
		}
		if _, err := fmt.Fprintf(writer, "- %s: %s\n", variant.ID, recoveryCommand(argv...)); err != nil {
			return err
		}
	}
	return nil
}
