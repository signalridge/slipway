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
	currentManifestVersion = 2
	legacyManifestVersion  = 1
	manifestFileName       = "ownership-manifest.json"
	sentinelFileName       = ".adapter-generated"
)

type manifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type ownershipManifest struct {
	Version      int            `json:"version"`
	ToolID       string         `json:"tool_id"`
	Files        []manifestFile `json:"files"`
	sourceSHA256 string
}

type InstallOptions struct {
	Root    string
	Tools   []string
	Refresh bool
}

type UninstallOptions struct {
	Root  string
	Tools []string
}

type ChangeReport struct {
	Hosts     []string `json:"hosts"`
	Written   []string `json:"written,omitempty"`
	Removed   []string `json:"removed,omitempty"`
	Preserved []string `json:"preserved,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
}

type HostStatus struct {
	ID           string   `json:"id"`
	Detected     bool     `json:"detected"`
	Installed    bool     `json:"installed"`
	NeedsRefresh bool     `json:"needs_refresh"`
	Capabilities []string `json:"capabilities"`
}

type DoctorCheck struct {
	Code   string `json:"code"`
	Status string `json:"status"`
	HostID string `json:"host_id"`
	Name   string `json:"name"`
	Detail string `json:"detail"`
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

func Install(options InstallOptions) (ChangeReport, error) {
	root, err := validatedRoot(options.Root)
	if err != nil {
		return ChangeReport{}, err
	}
	selected, err := resolveHosts(root, options.Tools, true)
	if err != nil {
		return ChangeReport{}, err
	}
	report := ChangeReport{}
	var operations []fsutil.FileTransactionOp
	for _, host := range selected {
		plan, err := planInstall(root, host, options.Refresh)
		if err != nil {
			return ChangeReport{}, err
		}
		report.Hosts = append(report.Hosts, host.ID)
		report.Written = append(report.Written, plan.written...)
		report.Removed = append(report.Removed, plan.removed...)
		report.Preserved = append(report.Preserved, plan.preserved...)
		report.Warnings = append(report.Warnings, plan.warnings...)
		operations = append(operations, plan.ops...)
	}
	if err := fsutil.ApplyFileTransactionWithin(root, operations); err != nil {
		return transactionFailureReport(root, report, err), fmt.Errorf("install adapters transactionally: %w", err)
	}
	return normalizeReport(report), nil
}

func Uninstall(options UninstallOptions) (ChangeReport, error) {
	root, err := validatedRoot(options.Root)
	if err != nil {
		return ChangeReport{}, err
	}
	selected, err := uninstallHosts(root, options.Tools)
	if err != nil {
		return ChangeReport{}, err
	}
	report := ChangeReport{}
	var operations []fsutil.FileTransactionOp
	for _, host := range selected {
		plan, err := planUninstall(root, host)
		if err != nil {
			return ChangeReport{}, err
		}
		report.Hosts = append(report.Hosts, host.ID)
		report.Removed = append(report.Removed, plan.removed...)
		report.Preserved = append(report.Preserved, plan.preserved...)
		report.Warnings = append(report.Warnings, plan.warnings...)
		operations = append(operations, plan.ops...)
	}
	if err := fsutil.ApplyFileTransactionWithin(root, operations); err != nil {
		return transactionFailureReport(root, report, err), fmt.Errorf("uninstall adapters transactionally: %w", err)
	}
	return normalizeReport(report), nil
}

func transactionFailureReport(root string, report ChangeReport, transactionErr error) ChangeReport {
	var cleanupErr *fsutil.FileTransactionCleanupError
	if !errors.As(transactionErr, &cleanupErr) {
		report.Written = nil
		report.Removed = nil
	}
	recoveries := transactionRecoveryErrors(transactionErr)
	for _, recovery := range recoveries {
		report.Preserved = append(report.Preserved, relativeToRoot(root, recovery.OriginalPath))
		if recovery.RecoveryPath != "" && !recovery.Reattached {
			report.Preserved = append(report.Preserved, relativeToRoot(root, recovery.RecoveryPath))
		}
		report.Warnings = append(report.Warnings, recovery.Error())
	}
	if cleanupErr != nil && len(recoveries) == 0 {
		report.Warnings = append(report.Warnings, cleanupErr.Error())
	}
	return normalizeReport(report)
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
		manifest, found, err := loadManifest(root, host)
		if err != nil {
			return nil, err
		}
		status := HostStatus{
			ID:           host.ID,
			Detected:     hostDetected(root, host),
			Installed:    found && manifest.Version == currentManifestVersion,
			Capabilities: []string{},
		}
		if found {
			inspection, err := inspectManagedSurface(root, host, manifest)
			if err != nil {
				return nil, err
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
			report.Checks = append(report.Checks, DoctorCheck{
				Code: "adapter_not_installed", Status: "warning", HostID: host.ID, Name: "adapter", Detail: "detected, not installed",
			})
			continue
		}
		if manifest.Version == legacyManifestVersion {
			report.Checks = append(report.Checks, DoctorCheck{
				Code: "adapter_legacy_manifest", Status: "warning", HostID: host.ID, Name: "adapter", Detail: "legacy ownership manifest; run slipway install --refresh",
			})
			continue
		}
		inspection, err := inspectManagedSurface(root, host, manifest)
		if err != nil {
			return DoctorReport{}, err
		}
		switch {
		case !inspection.Complete:
			report.Checks = append(report.Checks, DoctorCheck{
				Code: "adapter_refresh_required", Status: "warning", HostID: host.ID, Name: "adapter", Detail: "managed capability set is incomplete or outdated; run slipway install --refresh",
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
	desired, err := generateHostFiles(host)
	if err != nil {
		return plan, err
	}
	manifest, found, err := loadManifest(root, host)
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
	sentinelExpectation, err := observeRegularFileExpectation(sentinelPath)
	if err != nil {
		return plan, err
	}
	manifestExpectation := plannedFileExpectation{missing: true}
	if found {
		manifestExpectation = plannedFileExpectation{sha256: manifest.sourceSHA256}
	}

	previous := manifestIndex(manifest)
	legacyPristine := map[string]string{}
	claimed := map[string]manifestFile{}
	desiredIndex := map[string]generatedFile{}
	for _, file := range desired {
		desiredIndex[file.Relative] = file
	}

	if found && manifest.Version == legacyManifestVersion {
		for _, record := range manifest.Files {
			if isLegacySentinelClaim(host, record.Path) {
				continue
			}
			absolute, err := safeManifestPath(root, host, record.Path)
			if err != nil {
				return plan, err
			}
			classification, err := classifyFile(absolute, record.SHA256)
			if err != nil {
				return plan, err
			}
			switch classification {
			case "pristine":
				legacyPristine[record.Path] = record.SHA256
				if _, reused := desiredIndex[record.Path]; !reused {
					op := fsutil.RemoveFileTransactionOp(absolute).WithExpectedSHA256(record.SHA256)
					plan.ops = append(plan.ops, op)
					plan.removed = append(plan.removed, record.Path)
				}
			case "modified":
				plan.preserved = append(plan.preserved, record.Path)
			}
		}
	}

	if found && manifest.Version == currentManifestVersion {
		for _, record := range manifest.Files {
			if _, stillDesired := desiredIndex[record.Path]; stillDesired {
				continue
			}
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
	}

	for _, file := range desired {
		absolute, err := safeManifestPath(root, host, file.Relative)
		if err != nil {
			return plan, err
		}
		desiredHash := hashBytes(file.Data)
		allowWrite := false
		var writeExpectation plannedFileExpectation
		if found && manifest.Version == currentManifestVersion {
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
		if legacyHash, pristine := legacyPristine[file.Relative]; pristine {
			allowWrite = true
			writeExpectation = plannedFileExpectation{sha256: legacyHash}
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

	if settings, warning := safeSettingsCleanup(root, host); settings != nil {
		absolute := filepath.Join(root, filepath.FromSlash(settings.Relative))
		op := fsutil.WriteFileTransactionOp(absolute, settings.Data, 0o600).WithExpectedSHA256(settings.sourceSHA256)
		plan.ops = append(plan.ops, op)
		plan.removed = append(plan.removed, settings.Relative+" (retired Slipway entries)")
	} else if warning != "" {
		plan.warnings = append(plan.warnings, warning)
	}

	if len(claimed) == 0 {
		if found {
			plan.ops = append(plan.ops,
				manifestExpectation.guard(fsutil.RemoveFileTransactionOp(manifestPath)),
				sentinelExpectation.guard(fsutil.RemoveFileTransactionOp(sentinelPath)),
			)
		}
		if !found && !sentinelExpectation.missing {
			plan.warnings = append(plan.warnings, "preserved marker-only legacy adapter for "+host.ID)
		}
		return plan, nil
	}
	encoded, err := encodeManifest(ownershipManifest{Version: currentManifestVersion, ToolID: host.ID, Files: manifestValues(claimed)})
	if err != nil {
		return plan, err
	}
	plan.ops = append(plan.ops,
		manifestExpectation.guard(fsutil.WriteFileTransactionOp(manifestPath, encoded, 0o600)),
		sentinelExpectation.guard(fsutil.WriteFileTransactionOp(sentinelPath, []byte("generated\n"), 0o600)),
	)
	plan.written = append(plan.written, relativeToRoot(root, manifestPath), relativeToRoot(root, sentinelPath))
	return plan, nil
}

func planUninstall(root string, host Host) (hostPlan, error) {
	var plan hostPlan
	manifest, found, err := loadManifest(root, host)
	if err != nil {
		return plan, err
	}
	if found {
		for _, record := range manifest.Files {
			if manifest.Version == legacyManifestVersion && isLegacySentinelClaim(host, record.Path) {
				continue
			}
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
		sentinelExpectation, err := observeRegularFileExpectation(sentinelPath)
		if err != nil {
			return plan, err
		}
		plan.ops = append(plan.ops,
			fsutil.RemoveFileTransactionOp(manifestPath).WithExpectedSHA256(manifest.sourceSHA256),
			sentinelExpectation.guard(fsutil.RemoveFileTransactionOp(sentinelPath)),
		)
		plan.removed = append(plan.removed, relativeToRoot(root, manifestPath), relativeToRoot(root, sentinelPath))
	} else {
		_, sentinelPath, err := ownershipPaths(root, host)
		if err != nil {
			return plan, err
		}
		if _, err := os.Lstat(sentinelPath); err == nil {
			plan.warnings = append(plan.warnings, "preserved marker-only legacy adapter for "+host.ID)
		}
	}
	if settings, warning := safeSettingsCleanup(root, host); settings != nil {
		absolute := filepath.Join(root, filepath.FromSlash(settings.Relative))
		op := fsutil.WriteFileTransactionOp(absolute, settings.Data, 0o600).WithExpectedSHA256(settings.sourceSHA256)
		plan.ops = append(plan.ops, op)
		plan.removed = append(plan.removed, settings.Relative+" (retired Slipway entries)")
	} else if warning != "" {
		plan.warnings = append(plan.warnings, warning)
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
	raw, err := fsutil.ReadFileNoSymlink(manifestPath)
	if err != nil {
		return ownershipManifest{}, false, err
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
	if manifest.Version != legacyManifestVersion && manifest.Version != currentManifestVersion {
		return ownershipManifest{}, false, fmt.Errorf("unsupported ownership manifest version %d for %s", manifest.Version, host.ID)
	}
	if manifest.ToolID != host.ID {
		return ownershipManifest{}, false, fmt.Errorf("ownership manifest for %s belongs to %s", host.ID, manifest.ToolID)
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
		if _, err := safeManifestPath(root, host, relative); err != nil {
			return ownershipManifest{}, false, err
		}
		hash := strings.ToLower(strings.TrimSpace(manifest.Files[index].SHA256))
		decoded, err := hex.DecodeString(hash)
		if err != nil || len(decoded) != sha256.Size {
			return ownershipManifest{}, false, fmt.Errorf("invalid sha256 for %s", relative)
		}
		manifest.Files[index] = manifestFile{Path: relative, SHA256: hash}
	}
	for _, record := range manifest.Files {
		switch manifest.Version {
		case legacyManifestVersion:
			if !legacyV1ClaimAllowed(host, record.Path) {
				return ownershipManifest{}, false, fmt.Errorf("legacy ownership manifest for %s claims unknown path %q", host.ID, record.Path)
			}
		case currentManifestVersion:
			allowed, err := currentClaimAllowed(host, record.Path)
			if err != nil {
				return ownershipManifest{}, false, err
			}
			if !allowed {
				return ownershipManifest{}, false, fmt.Errorf("ownership manifest for %s claims unknown managed path %q", host.ID, record.Path)
			}
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

func safeSettingsCleanup(root string, host Host) (*settingsChange, string) {
	if host.SettingsPath == "" || host.SettingsKind == "preserve" {
		return nil, ""
	}
	if _, err := safePath(root, host.SettingsPath, host.OwnershipRoot); err != nil {
		return nil, "preserved settings for " + host.ID + ": " + err.Error()
	}
	return planSettingsCleanup(root, host)
}

func claimAllowed(host Host, relative string) bool {
	if host.ID != "copilot" {
		prefix := filepath.ToSlash(filepath.Clean(filepath.FromSlash(host.OwnershipRoot)))
		return relative == prefix || strings.HasPrefix(relative, prefix+"/")
	}
	if relative == ".github/copilot" || strings.HasPrefix(relative, ".github/copilot/") {
		return true
	}
	return strings.HasPrefix(relative, ".github/skills/slipway") || strings.HasPrefix(relative, ".github/prompts/slipway-")
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
	data, err := fsutil.ReadFileNoSymlink(path)
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

func observeRegularFileExpectation(path string) (plannedFileExpectation, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return plannedFileExpectation{missing: true}, nil
	}
	if err != nil {
		return plannedFileExpectation{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return plannedFileExpectation{}, fmt.Errorf("transaction input %s is not a regular file", path)
	}
	hash, err := hashRegularFile(path)
	if err != nil {
		return plannedFileExpectation{}, err
	}
	return plannedFileExpectation{sha256: hash}, nil
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
	report.Hosts = uniqueSorted(report.Hosts)
	report.Written = uniqueSorted(report.Written)
	report.Removed = uniqueSorted(report.Removed)
	report.Preserved = uniqueSorted(report.Preserved)
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
	Complete     bool
	Modified     int
	HealthyFiles map[string]bool
}

func inspectManagedSurface(root string, host Host, manifest ownershipManifest) (managedSurfaceInspection, error) {
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
	return inspection, nil
}

func healthyCapabilities(host Host, manifest ownershipManifest, healthyFiles map[string]bool) []string {
	capabilities := make([]string, 0, len(capabilityNames))
	for _, capability := range capabilityNames {
		prefix := filepath.ToSlash(filepath.Join(host.SkillsDir, capability)) + "/"
		found := false
		healthy := true
		for _, file := range manifest.Files {
			if !strings.HasPrefix(file.Path, prefix) {
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
