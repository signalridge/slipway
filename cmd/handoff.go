package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// handoffWriteMaxBodyBytes bounds the narrative read from stdin so a hostile or
// runaway producer cannot exhaust memory through the handoff writer.
const handoffWriteMaxBodyBytes = 1 << 20

const (
	handoffFullBodyExample    = "`printf '## Current Position\\n...' | slipway handoff write`"
	handoffSectionBodyExample = "`printf '...' | slipway handoff write --section %q`"
)

// handoffCommandStdin and handoffCommandIsTerminal isolate the real process
// stdin and its terminal probe so the non-interactive decision is made against
// the actual os.Stdin fd while tests can drive narrative through the injected
// command reader (cmd.SetIn) and simulate an interactive host.
var (
	handoffCommandStdin      = os.Stdin
	handoffCommandIsTerminal = term.IsTerminal
)

type handoffShowView struct {
	Path      string              `json:"path"`
	Header    state.HandoffHeader `json:"header"`
	Narrative string              `json:"narrative,omitempty"`
	Brief     string              `json:"brief,omitempty"`
	Empty     bool                `json:"empty,omitempty"`
	Notice    string              `json:"notice,omitempty"`
}

func makeHandoffCmd() *cobra.Command {
	var changeSlug string
	cmd := &cobra.Command{
		Use:   "handoff",
		Short: desc("handoff"),
		Long: desc("handoff") + `

Environment variables:
  SLIPWAY_SESSION_OWNER  Set the session-owner label recorded in the handoff
                         header. When unset, the owner falls back to USER, then
                         USERNAME, then the machine hostname, then "unknown".`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runHandoffWrite(cmd, changeSlug, "")
		},
	}
	addChangeSelectorFlags(cmd, &changeSlug, "Change slug to target")
	cmd.AddCommand(makeHandoffWriteCmd(&changeSlug))
	cmd.AddCommand(makeHandoffShowCmd(&changeSlug))
	return cmd
}

func makeHandoffWriteCmd(parentChangeSlug *string) *cobra.Command {
	var changeSlug string
	var section string
	cmd := &cobra.Command{
		Use:   "write",
		Short: "Create or refresh the per-change advisory handoff",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			targetSlug := strings.TrimSpace(changeSlug)
			if targetSlug == "" && parentChangeSlug != nil {
				targetSlug = *parentChangeSlug
			}
			return runHandoffWrite(cmd, targetSlug, section)
		},
	}
	addChangeSelectorFlags(cmd, &changeSlug, "Change slug to target")
	cmd.Flags().StringVar(&section, "section", "", "Narrative section to update from stdin")
	return cmd
}

func makeHandoffShowCmd(parentChangeSlug *string) *cobra.Command {
	var changeSlug string
	var jsonOut bool
	var brief bool
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the per-change advisory handoff",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			targetSlug := strings.TrimSpace(changeSlug)
			if targetSlug == "" && parentChangeSlug != nil {
				targetSlug = *parentChangeSlug
			}
			return runHandoffShow(cmd, targetSlug, jsonOut, brief)
		},
	}
	addChangeSelectorFlags(cmd, &changeSlug, "Change slug to target")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit structured JSON")
	cmd.Flags().BoolVar(&brief, "brief", false, "Emit a bounded machine descriptor only")
	return cmd
}

func runHandoffWrite(cmd *cobra.Command, changeSlug, section string) error {
	root, err := projectRootFromCommand(cmd)
	if err != nil {
		return err
	}
	ref, ok, err := resolveHandoffChangeRef(root, changeSlug)
	if err != nil || !ok {
		return err
	}
	change, err := state.LoadChange(root, ref.Slug)
	if err != nil {
		return err
	}

	section = strings.TrimSpace(section)
	if section != "" {
		canonical, known := state.CanonicalHandoffSection(section)
		if !known {
			return unknownHandoffSectionError(section)
		}
		section = canonical
	}

	// The non-interactive decision is made against the real os.Stdin fd, but the
	// narrative is read from the injected command reader so tests can exercise the
	// piped-body path. Interactive writes must not block on stdin waiting for EOF;
	// they guide instead unless the caller pipes content.
	interactive := handoffCommandIsTerminal(int(handoffCommandStdin.Fd()))
	body := ""
	if !interactive {
		raw, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), handoffWriteMaxBodyBytes))
		if err != nil {
			return err
		}
		body = string(raw)
	}

	if strings.TrimSpace(body) == "" {
		if interactive {
			// Interactive host with nothing to persist: guide instead of claiming a
			// false `handoff_written` success.
			return emitHandoffWriteGuidance(cmd, section)
		}
		// Non-interactive host that supplied no narrative: fail loudly so a headless
		// agent never receives a false success with silent data loss (#364).
		return emptyHandoffBodyError(section)
	}

	opts := state.HandoffWriteOptions{}
	if section != "" {
		opts.Section = section
		opts.SectionBody = body
	} else {
		opts.Body = body
	}
	doc, err := state.WriteHandoff(root, change, opts)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "handoff_written: %s\n", doc.Path)
	return err
}

// unknownHandoffSectionError fails a `--section` write whose name does not match
// a canonical advisory section, listing the valid names instead of silently
// writing a non-canonical section.
func unknownHandoffSectionError(section string) error {
	return newInvalidUsageError(
		"handoff_section_unknown",
		fmt.Sprintf("unknown handoff section %q", section),
		fmt.Sprintf("Pass one of the valid sections: %s.", handoffValidSectionsText()),
		map[string]any{"section": section, "valid_sections": state.HandoffSectionNames()},
	)
}

// emptyHandoffBodyError fails a non-interactive write that supplied no narrative,
// telling the agent exactly how to provide content.
func emptyHandoffBodyError(section string) error {
	if section != "" {
		return newInvalidUsageError(
			"handoff_body_empty",
			fmt.Sprintf("no narrative was supplied on stdin for section %q", section),
			fmt.Sprintf("Pipe the section body on stdin, e.g. %s. Valid sections: %s.", fmt.Sprintf(handoffSectionBodyExample, section), handoffValidSectionsText()),
			map[string]any{"section": section, "non_interactive": true},
		)
	}
	return newInvalidUsageError(
		"handoff_body_empty",
		"no handoff narrative was supplied on stdin",
		fmt.Sprintf("Pipe the narrative on stdin, e.g. %s, or update one section with `slipway handoff write --section \"<name>\"`. Valid sections: %s.", handoffFullBodyExample, handoffValidSectionsText()),
		map[string]any{"non_interactive": true},
	)
}

// emitHandoffWriteGuidance prints how to supply narrative on an interactive host
// without emitting a false `handoff_written` success line.
func emitHandoffWriteGuidance(cmd *cobra.Command, section string) error {
	if section != "" {
		_, err := fmt.Fprintf(
			cmd.ErrOrStderr(),
			"handoff not written: section %q needs a piped narrative on stdin, e.g. %s.\n",
			section, fmt.Sprintf(handoffSectionBodyExample, section),
		)
		return err
	}
	_, err := fmt.Fprintln(
		cmd.ErrOrStderr(),
		"handoff not written: pipe a narrative on stdin, e.g. "+handoffFullBodyExample+", or target a section with `slipway handoff write --section \"<name>\"`.",
	)
	return err
}

func runHandoffShow(cmd *cobra.Command, changeSlug string, jsonOut, brief bool) error {
	root, err := projectRootFromCommand(cmd)
	if err != nil {
		return err
	}
	ref, hasChange, err := resolveHandoffChangeRef(root, changeSlug)
	if err != nil {
		return err
	}

	var doc state.HandoffDocument
	hasDoc := false
	if hasChange {
		change, err := state.LoadChange(root, ref.Slug)
		if err != nil {
			return err
		}
		loaded, readErr := state.ReadHandoff(root, change)
		if readErr != nil {
			if !errors.Is(readErr, os.ErrNotExist) {
				return readErr
			}
		} else {
			doc = loaded
			hasDoc = true
		}
	}

	// Never produce silent empty output: a missing or all-pending handoff gets a
	// clear notice so the read side is honest about there being nothing recorded.
	if !hasDoc || state.HandoffIsEmpty(doc) {
		return emitHandoffEmptyNotice(cmd, doc, hasChange, jsonOut)
	}

	if brief {
		briefText := state.HandoffBrief(doc)
		if jsonOut {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(handoffShowView{
				Path:   doc.Path,
				Header: doc.Header,
				Brief:  briefText,
			})
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), briefText)
		return err
	}
	if jsonOut {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(handoffShowView{
			Path:      doc.Path,
			Header:    doc.Header,
			Narrative: doc.Narrative,
		})
	}
	_, err = io.WriteString(cmd.OutOrStdout(), state.RenderHandoff(doc))
	return err
}

// emitHandoffEmptyNotice reports an empty/missing handoff instead of printing
// nothing. JSON consumers receive a structured `empty` flag with the notice;
// human callers receive a one-line notice plus how to record a narrative.
func emitHandoffEmptyNotice(cmd *cobra.Command, doc state.HandoffDocument, hasChange, jsonOut bool) error {
	notice := "handoff is empty / all sections pending"
	guidance := notice + "; record one with " + handoffFullBodyExample + "."
	if !hasChange {
		notice = "no active change in this context; nothing to show"
		guidance = notice + "; run `slipway status` to choose an active change."
	}
	if jsonOut {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(handoffShowView{
			Path:      doc.Path,
			Header:    doc.Header,
			Narrative: doc.Narrative,
			Empty:     true,
			Notice:    notice,
		})
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), guidance)
	return err
}

func handoffValidSectionsText() string {
	return strings.Join(state.HandoffSectionNames(), ", ")
}

func resolveHandoffChangeRef(root, changeSlug string) (changeRef, bool, error) {
	ref, err := resolveActiveChangeRef(root, changeSlug)
	if err == nil {
		return ref, true, nil
	}
	if strings.TrimSpace(changeSlug) != "" {
		return changeRef{}, false, err
	}
	cliErr := asCLIError(err)
	if cliErr != nil && (cliErr.ErrorCode == "no_active_change" || cliErr.ErrorCode == "change_bound_to_other_worktree") {
		return changeRef{}, false, nil
	}
	return changeRef{}, false, err
}
