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
)

type handoffShowView struct {
	Path      string              `json:"path"`
	Header    state.HandoffHeader `json:"header"`
	Narrative string              `json:"narrative,omitempty"`
	Brief     string              `json:"brief,omitempty"`
}

func makeHandoffCmd() *cobra.Command {
	var changeSlug string
	cmd := &cobra.Command{
		Use:   "handoff",
		Short: desc("handoff"),
		Args:  cobra.NoArgs,
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
	body := ""
	if strings.TrimSpace(section) != "" {
		raw, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 1<<20))
		if err != nil {
			return err
		}
		body = string(raw)
	}
	doc, err := state.WriteHandoff(root, change, state.HandoffWriteOptions{
		Section:     section,
		SectionBody: body,
	})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "handoff_written: %s\n", doc.Path)
	return err
}

func runHandoffShow(cmd *cobra.Command, changeSlug string, jsonOut, brief bool) error {
	root, err := projectRootFromCommand(cmd)
	if err != nil {
		return err
	}
	doc, ok, err := readHandoffDocument(root, changeSlug)
	if err != nil || !ok {
		return err
	}
	if brief {
		briefText := state.HandoffBrief(doc)
		if briefText == "" {
			return nil
		}
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

func readHandoffDocument(root, changeSlug string) (state.HandoffDocument, bool, error) {
	ref, ok, err := resolveHandoffChangeRef(root, changeSlug)
	if err != nil || !ok {
		return state.HandoffDocument{}, false, err
	}
	change, err := state.LoadChange(root, ref.Slug)
	if err != nil {
		return state.HandoffDocument{}, false, err
	}
	doc, err := state.ReadHandoff(root, change)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state.HandoffDocument{}, false, nil
		}
		return state.HandoffDocument{}, false, err
	}
	return doc, true, nil
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
