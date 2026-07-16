package adapter

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/signalridge/slipway/internal/fsutil"
)

const (
	currentManifestVersion    = 2
	manifestFileName          = "ownership-manifest.json"
	sentinelFileName          = ".adapter-generated"
	generatedSentinelContent  = "generated\n"
	maxOwnershipManifestBytes = 256 << 10
	maxManagedFileBytes       = 1 << 20
)

type manifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type ownershipManifest struct {
	Version      int               `json:"version"`
	ToolID       string            `json:"tool_id"`
	Surface      map[string]string `json:"surface,omitempty"`
	Files        []manifestFile    `json:"files"`
	sourceSHA256 string
}

type InstallOptions struct {
	Root    string
	Tools   []string
	Surface string
	Refresh bool
}

// SurfaceSelectionError reports an invalid host surface selection.
type SurfaceSelectionError struct {
	HostID  string
	Surface string
	Reason  string
}

func (err *SurfaceSelectionError) Error() string {
	return err.Reason
}

type UninstallOptions struct {
	Root  string
	Tools []string
}

type TransactionOutcome string

const (
	TransactionOutcomeCommitted    TransactionOutcome = "committed"
	TransactionOutcomeRolledBack   TransactionOutcome = "rolled_back"
	TransactionOutcomeNotCommitted TransactionOutcome = "not_committed"
	TransactionOutcomeAmbiguous    TransactionOutcome = "ambiguous"
)

type ChangeReport struct {
	Hosts              []string           `json:"hosts"`
	TransactionOutcome TransactionOutcome `json:"transaction_outcome"`
	Written            []string           `json:"written,omitempty"`
	Removed            []string           `json:"removed,omitempty"`
	Preserved          []string           `json:"preserved,omitempty"`
	RecoveryArtifacts  []string           `json:"recovery_artifacts,omitempty"`
	Warnings           []string           `json:"warnings,omitempty"`
}

type HostStatus struct {
	ID           string   `json:"id"`
	Detected     bool     `json:"detected"`
	Installed    bool     `json:"installed"`
	NeedsRefresh bool     `json:"needs_refresh"`
	Capabilities []string `json:"capabilities"`
	// Warning carries an advisory note when a read-only listing could not
	// fully inspect a host (for example an unreadable or malformed manifest).
	// Listing is non-mutating, so one host's inspection problem must never
	// abort the whole report; instead the host degrades to an advisory and
	// the other hosts continue to be reported. See issue #434 §13.
	Warning string `json:"warning,omitempty"`
}

type DoctorDurability struct {
	Level         string `json:"level"`
	FileSync      bool   `json:"file_sync"`
	DirectorySync bool   `json:"directory_sync"`
	Limitation    string `json:"limitation,omitempty"`
}

type DoctorCheck struct {
	Code       string            `json:"code"`
	Status     string            `json:"status"`
	HostID     string            `json:"host_id"`
	Name       string            `json:"name"`
	Detail     string            `json:"detail"`
	Durability *DoctorDurability `json:"durability,omitempty"`
}

type DoctorReport struct {
	Checks []DoctorCheck `json:"checks"`
}

type hostPlan struct {
	ops       []fsutil.FileTransactionOp
	written   []string
	removed   []string
	preserved []string
	warnings  []string
}

func selectedSurfaceKind(surface string) (string, bool) {
	switch surface {
	case "ide", "cli":
		return "kiro_" + surface, true
	default:
		return "", false
	}
}

func hostWithManifestSurface(host Host, manifest ownershipManifest) (Host, error) {
	if host.ID != "kiro" {
		return host, nil
	}
	surface := manifest.Surface[host.ID]
	kind, valid := selectedSurfaceKind(surface)
	if !valid {
		return host, &SurfaceSelectionError{
			HostID:  host.ID,
			Surface: surface,
			Reason:  "ownership manifest for kiro does not record a valid surface (ide or cli)",
		}
	}
	host.SurfaceKind = kind
	return host, nil
}

func manifestSurface(host Host) map[string]string {
	if host.ID != "kiro" {
		return nil
	}
	return map[string]string{host.ID: strings.TrimPrefix(host.SurfaceKind, "kiro_")}
}

func Install(options InstallOptions) (ChangeReport, error) {
	report := ChangeReport{TransactionOutcome: TransactionOutcomeNotCommitted}
	root, err := validatedRoot(options.Root)
	if err != nil {
		return report, err
	}
	pinnedRoot, err := fsutil.OpenPinnedRoot(root)
	if err != nil {
		return report, err
	}
	defer func() { _ = pinnedRoot.Close() }()
	selected, err := resolveHosts(root, options.Tools, true)
	if err != nil {
		return report, err
	}

	var kiroSurfaceKind string
	if options.Surface != "" {
		hasKiro := false
		for _, host := range selected {
			if host.ID == "kiro" {
				hasKiro = true
				break
			}
		}
		if !hasKiro {
			return report, &SurfaceSelectionError{
				HostID:  "kiro",
				Surface: options.Surface,
				Reason:  "--surface requires kiro in the selected hosts",
			}
		}
		var valid bool
		kiroSurfaceKind, valid = selectedSurfaceKind(options.Surface)
		if !valid {
			return report, &SurfaceSelectionError{
				HostID:  "kiro",
				Surface: options.Surface,
				Reason:  "surface for kiro must be ide or cli",
			}
		}
	}

	var operations []fsutil.FileTransactionOp
	for _, host := range selected {
		if host.ID == "kiro" && kiroSurfaceKind != "" {
			host.SurfaceKind = kiroSurfaceKind
		}
		plan, err := planInstall(root, host, options.Refresh)
		if err != nil {
			var surfaceErr *SurfaceSelectionError
			if len(options.Tools) == 0 && options.Surface == "" && errors.As(err, &surfaceErr) && surfaceErr.HostID == "kiro" {
				report.Warnings = append(report.Warnings, "adapter kiro was not installed: first install needs --surface ide or --surface cli; other detected adapters were still planned")
				continue
			}
			return transactionFailureReport(root, report, err), err
		}
		report.Hosts = append(report.Hosts, host.ID)
		report.Written = append(report.Written, plan.written...)
		report.Removed = append(report.Removed, plan.removed...)
		report.Preserved = append(report.Preserved, plan.preserved...)
		report.Warnings = append(report.Warnings, plan.warnings...)
		operations = append(operations, plan.ops...)
	}
	if err := pinnedRoot.Apply(operations); err != nil {
		return transactionFailureReport(root, report, err), fmt.Errorf("install adapters transactionally: %w", err)
	}
	report.TransactionOutcome = TransactionOutcomeCommitted
	return normalizeReport(report), nil
}

func Uninstall(options UninstallOptions) (ChangeReport, error) {
	report := ChangeReport{TransactionOutcome: TransactionOutcomeNotCommitted}
	root, err := validatedRoot(options.Root)
	if err != nil {
		return report, err
	}
	pinnedRoot, err := fsutil.OpenPinnedRoot(root)
	if err != nil {
		return report, err
	}
	defer func() { _ = pinnedRoot.Close() }()
	selected, err := uninstallHosts(root, options.Tools)
	if err != nil {
		return report, err
	}
	var operations []fsutil.FileTransactionOp
	for _, host := range selected {
		plan, err := planUninstall(root, host)
		if err != nil {
			return transactionFailureReport(root, report, err), err
		}
		report.Hosts = append(report.Hosts, host.ID)
		report.Removed = append(report.Removed, plan.removed...)
		report.Preserved = append(report.Preserved, plan.preserved...)
		report.Warnings = append(report.Warnings, plan.warnings...)
		operations = append(operations, plan.ops...)
	}
	if err := pinnedRoot.Apply(operations); err != nil {
		return transactionFailureReport(root, report, err), fmt.Errorf("uninstall adapters transactionally: %w", err)
	}
	report.TransactionOutcome = TransactionOutcomeCommitted
	return normalizeReport(report), nil
}

func transactionFailureReport(root string, report ChangeReport, transactionErr error) ChangeReport {
	report.TransactionOutcome = classifyTransactionOutcome(transactionErr)
	if report.TransactionOutcome != TransactionOutcomeCommitted {
		report.Written = nil
		report.Removed = nil
	}
	recoveries := transactionRecoveryErrors(transactionErr)
	for _, recovery := range recoveries {
		artifactPath := recovery.OriginalPath
		if recovery.RecoveryPath != "" && !recovery.Reattached {
			artifactPath = recovery.RecoveryPath
		}
		report.RecoveryArtifacts = append(report.RecoveryArtifacts, relativeToRoot(root, artifactPath))
		report.Warnings = append(report.Warnings, recovery.Error())
	}
	var cleanupErr *fsutil.FileTransactionCleanupError
	if errors.As(transactionErr, &cleanupErr) && len(recoveries) == 0 {
		report.Warnings = append(report.Warnings, cleanupErr.Error())
	}
	return normalizeReport(report)
}

func classifyTransactionOutcome(transactionErr error) TransactionOutcome {
	var transaction *fsutil.FileTransactionError
	if errors.As(transactionErr, &transaction) {
		if len(transaction.RollbackErrs) == 0 {
			return TransactionOutcomeRolledBack
		}
		return TransactionOutcomeAmbiguous
	}
	var cleanup *fsutil.FileTransactionCleanupError
	if errors.As(transactionErr, &cleanup) {
		return TransactionOutcomeCommitted
	}
	var rootIdentity *fsutil.RootNamespaceIdentityError
	if errors.As(transactionErr, &rootIdentity) {
		if rootIdentity.Committed {
			return TransactionOutcomeAmbiguous
		}
		return TransactionOutcomeNotCommitted
	}
	return TransactionOutcomeNotCommitted
}

func transactionRecoveryErrors(err error) []*fsutil.FileTransactionRecoveryError {
	var recoveries []*fsutil.FileTransactionRecoveryError
	seen := map[*fsutil.FileTransactionRecoveryError]struct{}{}
	var visit func(error)
	visit = func(current error) {
		if current == nil {
			return
		}
		if recovery, ok := current.(*fsutil.FileTransactionRecoveryError); ok {
			if _, exists := seen[recovery]; !exists {
				seen[recovery] = struct{}{}
				recoveries = append(recoveries, recovery)
			}
		}
		switch wrapped := current.(type) {
		case interface{ Unwrap() []error }:
			for _, child := range wrapped.Unwrap() {
				visit(child)
			}
		case interface{ Unwrap() error }:
			visit(wrapped.Unwrap())
		}
	}
	visit(err)
	return recoveries
}

func List(root string) ([]HostStatus, error) {
	root, err := validatedRoot(root)
	if err != nil {
		return nil, err
	}
	statuses := make([]HostStatus, 0, len(hosts))
	for _, host := range hosts {
		status := HostStatus{
			ID:           host.ID,
			Detected:     hostDetected(root, host),
			Capabilities: []string{},
		}
		manifest, found, err := loadManifest(root, host)
		if err != nil {
			// Listing is read-only. A malformed or stale manifest for one host
			// is advisory, not a mutation-blocking failure, so degrade this
			// host instead of aborting the entire multi-host report.
			status.Warning = "ownership manifest could not be read: " + err.Error()
			statuses = append(statuses, status)
			continue
		}
		status.Installed = found && manifest.Version == currentManifestVersion
		if found {
			host, err = hostWithManifestSurface(host, manifest)
			if err != nil {
				status.Warning = "ownership manifest surface could not be read: " + err.Error()
				statuses = append(statuses, status)
				continue
			}
			inspection, err := inspectManagedSurface(root, host, manifest)
			if err != nil {
				status.Warning = "managed surface could not be inspected: " + err.Error()
				statuses = append(statuses, status)
				continue
			}
			status.NeedsRefresh = !inspection.Complete || inspection.Modified > 0
			if manifest.Version == currentManifestVersion {
				status.Capabilities = healthyCapabilities(host, manifest, inspection.HealthyFiles)
			}
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func Doctor(root string) (DoctorReport, error) {
	return doctorWithInspector(root, inspectManagedSurface)
}

func doctorWithInspector(
	root string,
	inspect func(string, Host, ownershipManifest) (managedSurfaceInspection, error),
) (DoctorReport, error) {
	root, err := validatedRoot(root)
	if err != nil {
		return DoctorReport{}, err
	}
	report := DoctorReport{Checks: []DoctorCheck{{
		Code: "repository_ok", Status: "ok", HostID: "-", Name: "repository", Detail: root,
	}}}
	for _, host := range hosts {
		manifest, found, err := loadManifest(root, host)
		if err != nil {
			report.Checks = append(report.Checks, DoctorCheck{
				Code: "adapter_manifest_unreadable", Status: "error", HostID: host.ID, Name: "manifest", Detail: err.Error(),
			})
			continue
		}
		if !hostDetected(root, host) && !found {
			report.Checks = append(report.Checks, DoctorCheck{
				Code: "adapter_not_detected", Status: "ok", HostID: host.ID, Name: "adapter", Detail: "not detected",
			})
			continue
		}
		if !found {
			detail := "detected, current ownership manifest is missing"
			if markerOnlyOwnershipState(root, host) {
				detail = currentOwnershipMissingWarning(host)
			}
			report.Checks = append(report.Checks, DoctorCheck{
				Code: "adapter_not_installed", Status: "warning", HostID: host.ID, Name: "adapter", Detail: detail,
			})
			continue
		}
		host, err = hostWithManifestSurface(host, manifest)
		if err != nil {
			report.Checks = append(report.Checks, DoctorCheck{
				Code: "adapter_manifest_unreadable", Status: "error", HostID: host.ID, Name: "manifest", Detail: err.Error(),
			})
			continue
		}
		inspection, err := inspect(root, host, manifest)
		if err != nil {
			report.Checks = append(report.Checks, DoctorCheck{
				Code: "adapter_inspection_unavailable", Status: "error", HostID: host.ID, Name: "adapter inspection", Detail: err.Error(),
			})
			continue
		}
		switch {
		case inspection.SentinelModified:
			detail := "generated sentinel was modified and is preserved as user content"
			if inspection.Modified > 1 {
				detail += fmt.Sprintf("; %d additional managed files changed or missing", inspection.Modified-1)
			}
			detail += "; inspect it and remove it manually before slipway install --refresh if regeneration is desired"
			report.Checks = append(report.Checks, DoctorCheck{
				Code: "adapter_modified", Status: "warning", HostID: host.ID, Name: "adapter", Detail: detail,
			})
		case !inspection.Complete || inspection.SentinelMissing:
			report.Checks = append(report.Checks, DoctorCheck{
				Code: "adapter_refresh_required", Status: "warning", HostID: host.ID, Name: "adapter", Detail: "managed capability set or generated sentinel is incomplete or outdated; run slipway install --refresh",
			})
		case inspection.Modified > 0:
			report.Checks = append(report.Checks, DoctorCheck{
				Code: "adapter_modified", Status: "warning", HostID: host.ID, Name: "adapter", Detail: fmt.Sprintf("%d managed files changed or missing", inspection.Modified),
			})
		default:
			report.Checks = append(report.Checks, DoctorCheck{
				Code: "adapter_healthy", Status: "ok", HostID: host.ID, Name: "adapter", Detail: fmt.Sprintf("%d managed files", len(manifest.Files)),
			})
		}
	}
	report.Checks = append(report.Checks, legacyRunstoreChecks(root)...)
	return report, nil
}

func planInstall(root string, host Host, refresh bool) (hostPlan, error) {
	var plan hostPlan
	manifest, found, err := loadManifest(root, host)
	if err != nil {
		return plan, err
	}
	if found {
		manifestHost, err := hostWithManifestSurface(host, manifest)
		if err != nil {
			return plan, err
		}
		if host.ID == "kiro" && host.SurfaceKind != "" && host.SurfaceKind != manifestHost.SurfaceKind {
			return plan, &SurfaceSelectionError{
				HostID:  host.ID,
				Surface: strings.TrimPrefix(host.SurfaceKind, "kiro_"),
				Reason:  "surface does not match the installed kiro surface",
			}
		}
		host = manifestHost
	} else if host.ID == "kiro" && host.SurfaceKind == "" {
		return plan, &SurfaceSelectionError{
			HostID: host.ID,
			Reason: "--surface is required for first kiro install; choose ide or cli",
		}
	}
	desired, err := generateHostFiles(host)
	if err != nil {
		return plan, err
	}
	if found && !refresh {
		plan.warnings = append(plan.warnings, "adapter "+host.ID+" is already installed; use slipway install --refresh to update managed files")
		return plan, nil
	}
	manifestPath, sentinelPath, err := ownershipPaths(root, host)
	if err != nil {
		return plan, err
	}
	sentinelClassification, err := classifyFile(sentinelPath, hashBytes([]byte(generatedSentinelContent)))
	if err != nil {
		return plan, err
	}
	if !found && sentinelClassification != "missing" {
		plan.warnings = append(plan.warnings, currentOwnershipMissingWarning(host))
		return plan, nil
	}
	sentinelWritable := sentinelClassification != "modified"
	sentinelExpectation := plannedFileExpectation{missing: sentinelClassification == "missing"}
	if sentinelClassification == "pristine" {
		sentinelExpectation.sha256 = hashBytes([]byte(generatedSentinelContent))
	}
	if found && !sentinelWritable {
		plan.preserved = append(plan.preserved, relativeToRoot(root, sentinelPath))
	}
	manifestExpectation := plannedFileExpectation{missing: true}
	if found {
		manifestExpectation = plannedFileExpectation{sha256: manifest.sourceSHA256}
	}

	previous := manifestIndex(manifest)
	claimed := map[string]manifestFile{}

	for _, file := range desired {
		absolute, err := safeManifestPath(root, host, file.Relative)
		if err != nil {
			return plan, err
		}
		desiredHash := hashBytes(file.Data)
		allowWrite := false
		var writeExpectation plannedFileExpectation
		if found {
			if record, managed := previous[file.Relative]; managed {
				classification, err := classifyFile(absolute, record.SHA256)
				if err != nil {
					return plan, err
				}
				if classification == "modified" {
					plan.preserved = append(plan.preserved, file.Relative)
					continue
				}
				allowWrite = true
				writeExpectation = plannedFileExpectation{missing: classification == "missing", sha256: record.SHA256}
			}
		}
		if !allowWrite {
			info, err := os.Lstat(absolute)
			if errors.Is(err, os.ErrNotExist) {
				allowWrite = true
				writeExpectation = plannedFileExpectation{missing: true}
			} else if err != nil {
				return plan, err
			} else if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				plan.preserved = append(plan.preserved, file.Relative)
				continue
			} else {
				plan.preserved = append(plan.preserved, file.Relative)
				continue
			}
		}
		if allowWrite {
			op := fsutil.WriteFileTransactionOp(absolute, file.Data, 0o644)
			plan.ops = append(plan.ops, writeExpectation.guard(op))
			plan.written = append(plan.written, file.Relative)
			claimed[file.Relative] = manifestFile{Path: file.Relative, SHA256: desiredHash}
		}
	}

	if len(claimed) == 0 {
		if found {
			plan.ops = append(plan.ops, manifestExpectation.guard(fsutil.RemoveFileTransactionOp(manifestPath)))
			plan.removed = append(plan.removed, relativeToRoot(root, manifestPath))
			if sentinelWritable && sentinelClassification != "missing" {
				plan.ops = append(plan.ops, sentinelExpectation.guard(fsutil.RemoveFileTransactionOp(sentinelPath)))
				plan.removed = append(plan.removed, relativeToRoot(root, sentinelPath))
			}
		}
		return plan, nil
	}
	encoded, err := encodeManifest(ownershipManifest{Version: currentManifestVersion, ToolID: host.ID, Surface: manifestSurface(host), Files: manifestValues(claimed)})
	if err != nil {
		return plan, err
	}
	plan.ops = append(plan.ops, manifestExpectation.guard(fsutil.WriteFileTransactionOp(manifestPath, encoded, 0o600)))
	plan.written = append(plan.written, relativeToRoot(root, manifestPath))
	if sentinelWritable {
		plan.ops = append(plan.ops, sentinelExpectation.guard(fsutil.WriteFileTransactionOp(sentinelPath, []byte(generatedSentinelContent), 0o600)))
		plan.written = append(plan.written, relativeToRoot(root, sentinelPath))
	}
	return plan, nil
}

func planUninstall(root string, host Host) (hostPlan, error) {
	var plan hostPlan
	manifest, found, err := loadManifest(root, host)
	if err != nil {
		return plan, err
	}
	if found {
		host, err = hostWithManifestSurface(host, manifest)
		if err != nil {
			return plan, err
		}
		for _, record := range manifest.Files {
			absolute, err := safeManifestPath(root, host, record.Path)
			if err != nil {
				return plan, err
			}
			classification, err := classifyFile(absolute, record.SHA256)
			if err != nil {
				return plan, err
			}
			if classification == "pristine" {
				op := fsutil.RemoveFileTransactionOp(absolute).WithExpectedSHA256(record.SHA256)
				plan.ops = append(plan.ops, op)
				plan.removed = append(plan.removed, record.Path)
			} else if classification == "modified" {
				plan.preserved = append(plan.preserved, record.Path)
			}
		}
		manifestPath, sentinelPath, err := ownershipPaths(root, host)
		if err != nil {
			return plan, err
		}
		plan.ops = append(plan.ops, fsutil.RemoveFileTransactionOp(manifestPath).WithExpectedSHA256(manifest.sourceSHA256))
		plan.removed = append(plan.removed, relativeToRoot(root, manifestPath))
		sentinelClassification, err := classifyFile(sentinelPath, hashBytes([]byte(generatedSentinelContent)))
		if err != nil {
			return plan, err
		}
		switch sentinelClassification {
		case "pristine":
			plan.ops = append(plan.ops, fsutil.RemoveFileTransactionOp(sentinelPath).WithExpectedSHA256(hashBytes([]byte(generatedSentinelContent))))
			plan.removed = append(plan.removed, relativeToRoot(root, sentinelPath))
		case "modified":
			plan.preserved = append(plan.preserved, relativeToRoot(root, sentinelPath))
		}
	} else {
		_, sentinelPath, err := ownershipPaths(root, host)
		if err != nil {
			return plan, err
		}
		if _, err := os.Lstat(sentinelPath); err == nil {
			plan.warnings = append(plan.warnings, currentOwnershipMissingWarning(host))
		}
	}
	return plan, nil
}

func uninstallHosts(root string, requested []string) ([]Host, error) {
	if len(requested) > 0 {
		return resolveHosts(root, requested, false)
	}
	var selected []Host
	for _, host := range hosts {
		_, found, err := loadManifest(root, host)
		if err != nil {
			return nil, err
		}
		if found {
			selected = append(selected, host)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no installed adapters found; select one with --tool")
	}
	return selected, nil
}

func loadManifest(root string, host Host) (ownershipManifest, bool, error) {
	manifestPath, _, err := ownershipPaths(root, host)
	if err != nil {
		return ownershipManifest{}, false, err
	}
	info, err := os.Lstat(manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		return ownershipManifest{}, false, nil
	}
	if err != nil {
		return ownershipManifest{}, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return ownershipManifest{}, false, fmt.Errorf("ownership manifest for %s is not a regular file", host.ID)
	}
	if info.Size() > maxOwnershipManifestBytes {
		return ownershipManifest{}, false, fmt.Errorf("ownership manifest for %s exceeds %d bytes", host.ID, maxOwnershipManifestBytes)
	}
	raw, err := fsutil.ReadFileNoSymlink(manifestPath, maxOwnershipManifestBytes)
	if err != nil {
		return ownershipManifest{}, false, err
	}
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return ownershipManifest{}, false, fmt.Errorf("parse ownership manifest for %s: %w", host.ID, err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var manifest ownershipManifest
	if err := decoder.Decode(&manifest); err != nil {
		return ownershipManifest{}, false, fmt.Errorf("parse ownership manifest for %s: %w", host.ID, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return ownershipManifest{}, false, fmt.Errorf("parse ownership manifest for %s: multiple JSON values", host.ID)
		}
		return ownershipManifest{}, false, fmt.Errorf("parse ownership manifest for %s: trailing data: %w", host.ID, err)
	}
	if manifest.Version != currentManifestVersion {
		return ownershipManifest{}, false, fmt.Errorf("unsupported ownership manifest version %d for %s", manifest.Version, host.ID)
	}
	if manifest.ToolID != host.ID {
		return ownershipManifest{}, false, fmt.Errorf("ownership manifest for %s belongs to %s", host.ID, manifest.ToolID)
	}
	host, err = hostWithManifestSurface(host, manifest)
	if err != nil {
		return ownershipManifest{}, false, err
	}
	if manifest.Files == nil {
		return ownershipManifest{}, false, fmt.Errorf("ownership manifest for %s requires a non-null files array", host.ID)
	}
	seen := map[string]struct{}{}
	for index := range manifest.Files {
		relative, err := normalizeRelative(manifest.Files[index].Path)
		if err != nil {
			return ownershipManifest{}, false, err
		}
		if _, duplicate := seen[relative]; duplicate {
			return ownershipManifest{}, false, fmt.Errorf("duplicate ownership path %q", relative)
		}
		seen[relative] = struct{}{}
		hash := strings.ToLower(strings.TrimSpace(manifest.Files[index].SHA256))
		decoded, err := hex.DecodeString(hash)
		if err != nil || len(decoded) != sha256.Size {
			return ownershipManifest{}, false, fmt.Errorf("invalid sha256 for %s", relative)
		}
		manifest.Files[index] = manifestFile{Path: relative, SHA256: hash}
	}
	// The manifest is untrusted repository JSON. A claim authorizes mutation
	// only when both its normalized path and content hash exactly match bytes
	// generated by this binary; a path-only check would let a forged manifest
	// claim arbitrary user content at a real managed path.
	currentClaims, err := currentGeneratedClaims(host)
	if err != nil {
		return ownershipManifest{}, false, err
	}
	for _, record := range manifest.Files {
		if currentClaims[record.Path] != record.SHA256 {
			return ownershipManifest{}, false, fmt.Errorf("ownership manifest for %s claims path %q that is not a current pristine managed file (unknown path or content not generated by this version)", host.ID, record.Path)
		}
	}
	manifest.sourceSHA256 = hashBytes(raw)
	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Path < manifest.Files[j].Path })
	return manifest, true, nil
}

func ownershipPaths(root string, host Host) (string, string, error) {
	manifestRelative := filepath.ToSlash(filepath.Join(host.OwnershipRoot, "slipway", manifestFileName))
	sentinelRelative := filepath.ToSlash(filepath.Join(host.OwnershipRoot, "slipway", sentinelFileName))
	manifestPath, err := safePath(root, manifestRelative, host.OwnershipRoot)
	if err != nil {
		return "", "", err
	}
	sentinelPath, err := safePath(root, sentinelRelative, host.OwnershipRoot)
	return manifestPath, sentinelPath, err
}

func safeManifestPath(root string, host Host, relative string) (string, error) {
	relative, err := normalizeRelative(relative)
	if err != nil {
		return "", err
	}
	if !claimAllowed(host, relative) {
		return "", fmt.Errorf("ownership path %q is outside adapter %s", relative, host.ID)
	}
	return safePath(root, relative, "")
}

func safePath(root, relative, requiredPrefix string) (string, error) {
	relative, err := normalizeRelative(relative)
	if err != nil {
		return "", err
	}
	if requiredPrefix != "" {
		prefix := filepath.ToSlash(filepath.Clean(filepath.FromSlash(requiredPrefix)))
		if relative != prefix && !strings.HasPrefix(relative, prefix+"/") {
			return "", fmt.Errorf("path %q is outside %s", relative, prefix)
		}
	}
	absolute := filepath.Join(root, filepath.FromSlash(relative))
	if !fsutil.PathWithin(root, absolute) {
		return "", fmt.Errorf("path %q escapes repository", relative)
	}
	current := root
	parts := strings.Split(filepath.FromSlash(relative), string(os.PathSeparator))
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("path %q crosses symlink component %q", relative, strings.Join(parts[:index+1], "/"))
		}
		if index < len(parts)-1 && !info.IsDir() {
			return "", fmt.Errorf("path %q crosses non-directory component", relative)
		}
	}
	return absolute, nil
}

func claimAllowed(host Host, relative string) bool {
	claims, err := currentGeneratedClaims(host)
	if err != nil {
		return false
	}
	_, allowed := claims[relative]
	return allowed
}

// currentGeneratedClaims computes the exact current ownership surface once per
// manifest load. The map is both a path allowlist and a content-provenance
// anchor; callers must compare the claimed hash, not merely path membership.
func currentGeneratedClaims(host Host) (map[string]string, error) {
	files, err := generateHostFiles(host)
	if err != nil {
		return nil, fmt.Errorf("enumerate managed files for %s: %w", host.ID, err)
	}
	claims := make(map[string]string, len(files))
	for _, file := range files {
		claims[file.Relative] = hashBytes(file.Data)
	}
	return claims, nil
}

func markerOnlyOwnershipState(root string, host Host) bool {
	_, sentinelPath, err := ownershipPaths(root, host)
	if err != nil {
		return false
	}
	_, err = os.Lstat(sentinelPath)
	return err == nil
}

func currentOwnershipMissingWarning(host Host) string {
	return "current ownership manifest is missing for " + host.ID + "; marker-only state does not establish file ownership"
}

func normalizeRelative(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("ownership path is required")
	}
	relative := filepath.Clean(filepath.FromSlash(name))
	if relative == "." || relative == ".." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path must be repository-relative: %q", name)
	}
	return filepath.ToSlash(relative), nil
}

func classifyFile(path, expectedHash string) (string, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "missing", nil
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "modified", nil
	}
	if info.Size() > maxManagedFileBytes {
		return "modified", nil
	}
	hash, err := hashRegularFile(path)
	if err != nil {
		return "", err
	}
	if hash == expectedHash {
		return "pristine", nil
	}
	return "modified", nil
}

func hashRegularFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", path)
	}
	data, err := fsutil.ReadFileNoSymlink(path, maxManagedFileBytes)
	if err != nil {
		return "", err
	}
	return hashBytes(data), nil
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type plannedFileExpectation struct {
	missing bool
	sha256  string
}

func (expectation plannedFileExpectation) guard(op fsutil.FileTransactionOp) fsutil.FileTransactionOp {
	if expectation.missing {
		return op.WithExpectedMissing()
	}
	return op.WithExpectedSHA256(expectation.sha256)
}

func encodeManifest(manifest ownershipManifest) ([]byte, error) {
	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Path < manifest.Files[j].Path })
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func manifestIndex(manifest ownershipManifest) map[string]manifestFile {
	index := make(map[string]manifestFile, len(manifest.Files))
	for _, file := range manifest.Files {
		index[file.Path] = file
	}
	return index
}

func manifestValues(index map[string]manifestFile) []manifestFile {
	values := make([]manifestFile, 0, len(index))
	for _, value := range index {
		values = append(values, value)
	}
	return values
}

func validatedRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("repository root is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("repository root is not a directory")
	}
	return filepath.Clean(absolute), nil
}

func relativeToRoot(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

func normalizeReport(report ChangeReport) ChangeReport {
	if report.TransactionOutcome == "" {
		report.TransactionOutcome = TransactionOutcomeNotCommitted
	}
	if report.TransactionOutcome != TransactionOutcomeCommitted {
		report.Written = nil
		report.Removed = nil
	}
	report.Hosts = uniqueSorted(report.Hosts)
	report.Written = uniqueSorted(report.Written)
	report.Removed = uniqueSorted(report.Removed)
	report.Preserved = uniqueSorted(report.Preserved)
	report.RecoveryArtifacts = uniqueSorted(report.RecoveryArtifacts)
	report.Warnings = uniqueSorted(report.Warnings)
	return report
}

func uniqueSorted(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

type managedSurfaceInspection struct {
	Complete         bool
	Modified         int
	HealthyFiles     map[string]bool
	SentinelMissing  bool
	SentinelModified bool
}

func inspectManagedSurface(root string, host Host, manifest ownershipManifest) (managedSurfaceInspection, error) {
	host, err := hostWithManifestSurface(host, manifest)
	if err != nil {
		return managedSurfaceInspection{}, err
	}
	complete, err := managedSurfaceComplete(host, manifest)
	if err != nil {
		return managedSurfaceInspection{}, err
	}
	inspection := managedSurfaceInspection{Complete: complete, HealthyFiles: make(map[string]bool, len(manifest.Files))}
	for _, record := range manifest.Files {
		path, err := safeManifestPath(root, host, record.Path)
		if err != nil {
			inspection.Modified++
			continue
		}
		hash, err := hashRegularFile(path)
		if err != nil || hash != record.SHA256 {
			inspection.Modified++
			continue
		}
		inspection.HealthyFiles[record.Path] = true
	}
	// The sentinel (.adapter-generated) is a generated health signal maintained
	// alongside the v2 ownership manifest, not ownership authority. A missing
	// sentinel can be refreshed; a modified sentinel is user content that requires
	// explicit inspection or removal rather than an overwrite.
	_, sentinelPath, err := ownershipPaths(root, host)
	if err != nil {
		return managedSurfaceInspection{}, err
	}
	sentinelClassification, err := classifyFile(sentinelPath, hashBytes([]byte(generatedSentinelContent)))
	if err != nil {
		return managedSurfaceInspection{}, err
	}
	switch sentinelClassification {
	case "missing":
		inspection.SentinelMissing = true
		inspection.Modified++
	case "modified":
		inspection.SentinelModified = true
		inspection.Modified++
	}
	return inspection, nil
}

func healthyCapabilities(_ Host, manifest ownershipManifest, healthyFiles map[string]bool) []string {
	capabilities := make([]string, 0, len(capabilityNames))
	for _, capability := range capabilityNames {
		found := false
		healthy := true
		for _, file := range manifest.Files {
			if !strings.Contains(file.Path, capability) {
				continue
			}
			found = true
			if !healthyFiles[file.Path] {
				healthy = false
			}
		}
		if found && healthy {
			capabilities = append(capabilities, capability)
		}
	}
	return capabilities
}

func managedSurfaceComplete(host Host, manifest ownershipManifest) (bool, error) {
	if manifest.Version != currentManifestVersion {
		return false, nil
	}
	desired, err := generateHostFiles(host)
	if err != nil {
		return false, err
	}
	if len(manifest.Files) != len(desired) {
		return false, nil
	}
	index := manifestIndex(manifest)
	for _, file := range desired {
		record, ok := index[file.Relative]
		if !ok || record.SHA256 != hashBytes(file.Data) {
			return false, nil
		}
	}
	return true, nil
}
