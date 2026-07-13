// Package runstore provides durable, repository-local journals for autopilot recovery.
package runstore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/signalridge/slipway/internal/fsutil"
)

const (
	journalFileName    = "journal.jsonl"
	projectionFileName = "run.json"
	lockFileName       = "run.lock"

	// WorkspaceIdentityVersion is the serialized workspace identity contract.
	WorkspaceIdentityVersion = 1
	// GitObservationVersion is the serialized Git observation contract.
	GitObservationVersion = 1
	// MaxObservedFileBytes is the threshold for a full regular-file digest.
	// Larger files use a bounded, domain-separated sample fingerprint.
	MaxObservedFileBytes int64 = 16 << 20

	// maxGitObservationDetailBytes bounds the persisted prefix of dirty-path
	// details. The complete set still contributes to PathFingerprint.
	maxGitObservationDetailBytes = 512 << 10

	oversizeFileSampleBytes int64 = 64 << 10
)

type Paths struct {
	Directory   string
	JournalFile string
	RunFile     string
	LockFile    string
}

// WorkspaceIdentity pins a Run to one canonical Git worktree and its metadata.
// GitDir intentionally makes linked worktrees distinct even when CommonDir is shared.
type WorkspaceIdentity struct {
	Version      int    `json:"version"`
	WorktreeRoot string `json:"worktree_root"`
	GitDir       string `json:"git_dir"`
	GitCommonDir string `json:"git_common_dir"`
	ID           string `json:"id"`
}

// PathObservation is a metadata-only record for one dirty or untracked path.
// Content is never retained; regular content and symlink targets are represented
// by a full digest or, for oversize files, a bounded sample fingerprint.
type PathObservation struct {
	Path          string `json:"path"`
	Category      string `json:"category"`
	State         string `json:"state"`
	Observation   string `json:"observation"`
	Size          *int64 `json:"size,omitempty"`
	ContentSHA256 string `json:"content_sha256,omitempty"`
}

// GitObservation records a bounded, exact identity for the observed worktree
// state. DirtyFiles and PathObservations are a sorted, non-nil prefix of the
// complete dirty-path set. PathCount and PathFingerprint cover the complete set;
// DetailsTruncated states explicitly when the prefix omits details.
type GitObservation struct {
	Version           int               `json:"version"`
	Head              string            `json:"head"`
	IndexFingerprint  string            `json:"index_fingerprint"`
	StatusFingerprint string            `json:"status_fingerprint"`
	SnapshotHash      string            `json:"snapshot_hash"`
	PathCount         int               `json:"path_count"`
	PathFingerprint   string            `json:"path_fingerprint"`
	DetailsTruncated  bool              `json:"details_truncated"`
	DirtyFiles        []string          `json:"dirty_files"`
	PathObservations  []PathObservation `json:"path_observations"`
}

type statusPath struct {
	path     string
	category string
	state    string
}

func pathsFor(commonDir, runID string) (Paths, error) {
	if err := validateRunID(runID); err != nil {
		return Paths{}, err
	}
	directory := filepath.Join(commonDir, "slipway", "runs", runID)
	return Paths{
		Directory:   directory,
		JournalFile: filepath.Join(directory, journalFileName),
		RunFile:     filepath.Join(directory, projectionFileName),
		LockFile:    filepath.Join(directory, lockFileName),
	}, nil
}

func validateRunID(runID string) error {
	if runID == "" || len(runID) > 128 {
		return fmt.Errorf("invalid run id")
	}
	for _, char := range runID {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '-' || char == '_' {
			continue
		}
		return fmt.Errorf("invalid run id %q", runID)
	}
	return nil
}

// DiscoverWorkspaceIdentity resolves start through Git without a shell and
// canonicalizes the worktree, per-worktree Git directory, and common Git directory.
func DiscoverWorkspaceIdentity(start string) (WorkspaceIdentity, error) {
	root, err := gitStartDirectory(start)
	if err != nil {
		return WorkspaceIdentity{}, err
	}
	worktree, err := gitOutput(root, "rev-parse", "--path-format=absolute", "--show-toplevel")
	if err != nil {
		return WorkspaceIdentity{}, fmt.Errorf("discover worktree root: %w", err)
	}
	gitDir, err := gitOutput(root, "rev-parse", "--path-format=absolute", "--git-dir")
	if err != nil {
		return WorkspaceIdentity{}, fmt.Errorf("discover git directory: %w", err)
	}
	commonDir, err := gitOutput(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return WorkspaceIdentity{}, fmt.Errorf("discover git common directory: %w", err)
	}

	worktree, err = canonicalGitPath(worktree)
	if err != nil {
		return WorkspaceIdentity{}, fmt.Errorf("canonicalize worktree root: %w", err)
	}
	gitDir, err = canonicalGitPath(gitDir)
	if err != nil {
		return WorkspaceIdentity{}, fmt.Errorf("canonicalize git directory: %w", err)
	}
	commonDir, err = canonicalGitPath(commonDir)
	if err != nil {
		return WorkspaceIdentity{}, fmt.Errorf("canonicalize git common directory: %w", err)
	}
	identity := WorkspaceIdentity{
		Version:      WorkspaceIdentityVersion,
		WorktreeRoot: worktree,
		GitDir:       gitDir,
		GitCommonDir: commonDir,
	}
	identity.ID = workspaceIdentityID(identity)
	return identity, nil
}

// Validate checks the serialized identity without consulting the filesystem.
func (identity WorkspaceIdentity) Validate() error {
	if identity.Version != WorkspaceIdentityVersion {
		return fmt.Errorf("workspace identity version must be %d", WorkspaceIdentityVersion)
	}
	for name, value := range map[string]string{
		"worktree_root":  identity.WorktreeRoot,
		"git_dir":        identity.GitDir,
		"git_common_dir": identity.GitCommonDir,
	} {
		if value == "" || !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 || !filepath.IsAbs(value) {
			return fmt.Errorf("workspace identity %s must be an absolute valid utf-8 path without NUL", name)
		}
		if filepath.Clean(value) != value {
			return fmt.Errorf("workspace identity %s must be clean", name)
		}
	}
	if !validSHA256(identity.ID) {
		return errors.New("workspace identity id must be a lowercase sha256 digest")
	}
	if expected := workspaceIdentityID(identity); identity.ID != expected {
		return errors.New("workspace identity id does not match its canonical paths")
	}
	return nil
}

// Equal reports exact equality across the versioned identity, not merely ID equality.
func (identity WorkspaceIdentity) Equal(other WorkspaceIdentity) bool {
	return identity.Version == other.Version &&
		identity.WorktreeRoot == other.WorktreeRoot &&
		identity.GitDir == other.GitDir &&
		identity.GitCommonDir == other.GitCommonDir &&
		identity.ID == other.ID
}

func workspaceIdentityID(identity WorkspaceIdentity) string {
	hasher := sha256.New()
	writeHashSection(hasher, "version", []byte(strconv.Itoa(identity.Version)))
	writeHashSection(hasher, "worktree_root", []byte(identity.WorktreeRoot))
	writeHashSection(hasher, "git_dir", []byte(identity.GitDir))
	writeHashSection(hasher, "git_common_dir", []byte(identity.GitCommonDir))
	return hashDigest(hasher)
}

func gitStartDirectory(start string) (string, error) {
	if strings.TrimSpace(start) == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
	}
	absolute, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("absolute repository path: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("inspect repository path: %w", err)
	}
	if !info.IsDir() {
		absolute = filepath.Dir(absolute)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve repository path: %w", err)
	}
	return filepath.Clean(resolved), nil
}

func canonicalGitPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("git returned an empty path")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("git path is not a directory")
	}
	return filepath.Clean(resolved), nil
}

// ObserveGit captures exact index and porcelain-v2 fingerprints plus bounded
// per-path metadata for every currently dirty or untracked path.
func ObserveGit(root string) (GitObservation, error) {
	identity, err := DiscoverWorkspaceIdentity(root)
	if err != nil {
		return GitObservation{}, err
	}
	root = identity.WorktreeRoot

	head, err := gitOutput(root, "rev-parse", "--verify", "HEAD")
	if err != nil {
		head = "unborn"
	}
	index, err := gitBytes(root, "ls-files", "--stage", "-z")
	if err != nil {
		return GitObservation{}, err
	}
	status, err := gitBytes(root, "status", "--porcelain=v2", "-z", "--untracked-files=all")
	if err != nil {
		return GitObservation{}, err
	}
	paths, err := parsePorcelainV2(status)
	if err != nil {
		return GitObservation{}, err
	}
	paths = deduplicateStatusPaths(paths)

	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return GitObservation{}, fmt.Errorf("open repository root: %w", err)
	}
	defer func() { _ = rootHandle.Close() }()

	observation := GitObservation{
		Version:           GitObservationVersion,
		Head:              head,
		IndexFingerprint:  digestBytes(index),
		StatusFingerprint: digestBytes(status),
		PathCount:         len(paths),
		DirtyFiles:        make([]string, 0, min(len(paths), 128)),
		PathObservations:  make([]PathObservation, 0, min(len(paths), 128)),
	}
	pathHasher := sha256.New()
	writeHashSection(pathHasher, "path_count", []byte(strconv.Itoa(len(paths))))
	detailBytes := 0
	for _, path := range paths {
		item := observePath(rootHandle, path)
		writePathObservationHash(pathHasher, item)
		cost := estimatedPathObservationJSONBytes(item)
		if !observation.DetailsTruncated && detailBytes+cost <= maxGitObservationDetailBytes {
			observation.DirtyFiles = append(observation.DirtyFiles, path.path)
			observation.PathObservations = append(observation.PathObservations, item)
			detailBytes += cost
		} else {
			observation.DetailsTruncated = true
		}
	}
	observation.PathFingerprint = hashDigest(pathHasher)
	observation.SnapshotHash = gitSnapshotHash(observation)
	if err := observation.Validate(); err != nil {
		return GitObservation{}, fmt.Errorf("validate git observation: %w", err)
	}
	return observation, nil
}

// Validate checks deterministic ordering, bounded path metadata, and the snapshot digest.
func (observation GitObservation) Validate() error {
	if observation.Version != GitObservationVersion {
		return fmt.Errorf("git observation version must be %d", GitObservationVersion)
	}
	if observation.Head == "" || !utf8.ValidString(observation.Head) || strings.IndexByte(observation.Head, 0) >= 0 {
		return errors.New("git observation head must be nonempty valid utf-8 without NUL")
	}
	if !validSHA256(observation.IndexFingerprint) {
		return errors.New("git observation index_fingerprint must be a lowercase sha256 digest")
	}
	if !validSHA256(observation.StatusFingerprint) {
		return errors.New("git observation status_fingerprint must be a lowercase sha256 digest")
	}
	if observation.PathCount < 0 {
		return errors.New("git observation path_count cannot be negative")
	}
	if !validSHA256(observation.PathFingerprint) {
		return errors.New("git observation path_fingerprint must be a lowercase sha256 digest")
	}
	if observation.DirtyFiles == nil || observation.PathObservations == nil {
		return errors.New("git observation arrays must be non-null")
	}
	if len(observation.DirtyFiles) != len(observation.PathObservations) {
		return errors.New("git observation dirty files and path observations must correspond")
	}
	if observation.PathCount < len(observation.PathObservations) {
		return errors.New("git observation path_count cannot be smaller than retained path details")
	}
	if observation.DetailsTruncated != (observation.PathCount > len(observation.PathObservations)) {
		return errors.New("git observation details_truncated must state whether path details were omitted")
	}
	for index, path := range observation.DirtyFiles {
		if err := validateObservedPath(path); err != nil {
			return fmt.Errorf("dirty_files[%d]: %w", index, err)
		}
		if index > 0 && observation.DirtyFiles[index-1] >= path {
			return errors.New("git observation dirty files must be sorted and unique")
		}
		item := observation.PathObservations[index]
		if item.Path != path {
			return errors.New("git observation path observations must match dirty files in order")
		}
		if item.Category == "" || item.State == "" || !utf8.ValidString(item.Category) || !utf8.ValidString(item.State) {
			return fmt.Errorf("path observation %q requires valid category and state", path)
		}
		switch item.Observation {
		case "regular", "symlink", "oversize":
			if !validSHA256(item.ContentSHA256) {
				return fmt.Errorf("path observation %q requires a content digest", path)
			}
		case "missing", "non_regular", "unreadable":
			if item.ContentSHA256 != "" {
				return fmt.Errorf("path observation %q cannot retain a content digest for %s", path, item.Observation)
			}
		default:
			return fmt.Errorf("path observation %q has unsupported observation %q", path, item.Observation)
		}
		if item.Size != nil && *item.Size < 0 {
			return fmt.Errorf("path observation %q has negative size", path)
		}
	}
	if !observation.DetailsTruncated {
		expectedFingerprint := pathObservationFingerprint(observation.PathObservations)
		if observation.PathFingerprint != expectedFingerprint {
			return errors.New("git observation path_fingerprint does not match path details")
		}
	}
	if !validSHA256(observation.SnapshotHash) {
		return errors.New("git observation snapshot_hash must be a lowercase sha256 digest")
	}
	if expected := gitSnapshotHash(observation); observation.SnapshotHash != expected {
		return errors.New("git observation snapshot_hash does not match structured fields")
	}
	return nil
}

// ChangedFrom compares the digest over all structured observation fields.
func (observation GitObservation) ChangedFrom(initial GitObservation) bool {
	return observation.SnapshotHash != initial.SnapshotHash
}

func parsePorcelainV2(raw []byte) ([]statusPath, error) {
	records := bytes.Split(raw, []byte{0})
	paths := make([]statusPath, 0, len(records))
	for index := 0; index < len(records); index++ {
		record := records[index]
		if len(record) == 0 {
			continue
		}
		switch record[0] {
		case '1':
			fields := bytes.SplitN(record, []byte(" "), 9)
			if len(fields) != 9 {
				return nil, errors.New("parse git porcelain v2 ordinary record")
			}
			path, err := porcelainPath(fields[8])
			if err != nil {
				return nil, err
			}
			paths = append(paths, statusPath{path: path, category: "ordinary", state: string(fields[1])})
		case '2':
			fields := bytes.SplitN(record, []byte(" "), 10)
			if len(fields) != 10 || len(fields[8]) < 2 {
				return nil, errors.New("parse git porcelain v2 rename/copy record")
			}
			if index+1 >= len(records) || len(records[index+1]) == 0 {
				return nil, errors.New("parse git porcelain v2 rename/copy origin")
			}
			destination, err := porcelainPath(fields[9])
			if err != nil {
				return nil, err
			}
			index++
			origin, err := porcelainPath(records[index])
			if err != nil {
				return nil, err
			}
			category := "rename"
			if fields[8][0] == 'C' {
				category = "copy"
			}
			state := string(fields[1]) + " " + string(fields[8])
			paths = append(paths,
				statusPath{path: destination, category: category, state: state},
				statusPath{path: origin, category: category + "_origin", state: state},
			)
		case 'u':
			fields := bytes.SplitN(record, []byte(" "), 11)
			if len(fields) != 11 {
				return nil, errors.New("parse git porcelain v2 unmerged record")
			}
			path, err := porcelainPath(fields[10])
			if err != nil {
				return nil, err
			}
			paths = append(paths, statusPath{path: path, category: "unmerged", state: string(fields[1])})
		case '?':
			if len(record) < 3 || record[1] != ' ' {
				return nil, errors.New("parse git porcelain v2 untracked record")
			}
			path, err := porcelainPath(record[2:])
			if err != nil {
				return nil, err
			}
			paths = append(paths, statusPath{path: path, category: "untracked", state: "??"})
		case '!':
			// Ignored records are not requested, but tolerate them defensively.
			continue
		default:
			return nil, fmt.Errorf("parse git porcelain v2 record type %q", record[0])
		}
	}
	return paths, nil
}

func porcelainPath(raw []byte) (string, error) {
	if len(raw) == 0 || !utf8.Valid(raw) {
		return "", errors.New("git porcelain v2 path must be nonempty valid utf-8")
	}
	path := filepath.ToSlash(string(raw))
	if err := validateObservedPath(path); err != nil {
		return "", err
	}
	return path, nil
}

func validateObservedPath(path string) error {
	if path == "" || !utf8.ValidString(path) || strings.IndexByte(path, 0) >= 0 {
		return errors.New("observed path must be nonempty valid utf-8 without NUL")
	}
	converted := filepath.FromSlash(path)
	if filepath.IsAbs(converted) || converted == ".." || strings.HasPrefix(converted, ".."+string(filepath.Separator)) {
		return errors.New("observed path must remain relative to the worktree")
	}
	return nil
}

func deduplicateStatusPaths(paths []statusPath) []statusPath {
	byPath := make(map[string]statusPath, len(paths))
	for _, item := range paths {
		if existing, ok := byPath[item.path]; ok {
			existing.category = mergeTokenList(existing.category, item.category)
			existing.state = mergeTokenList(existing.state, item.state)
			byPath[item.path] = existing
			continue
		}
		byPath[item.path] = item
	}
	result := make([]statusPath, 0, len(byPath))
	for _, item := range byPath {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].path < result[j].path })
	return result
}

func mergeTokenList(left, right string) string {
	set := map[string]struct{}{left: {}, right: {}}
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return strings.Join(values, ",")
}

func hashOversizeFileSamples(file *os.File, size int64) (string, error) {
	if size <= MaxObservedFileBytes {
		return "", errors.New("oversize sample requires an oversize file")
	}
	hasher := sha256.New()
	writeHashSection(hasher, "mode", []byte("oversize_samples_v1"))
	writeHashSection(hasher, "size", []byte(strconv.FormatInt(size, 10)))
	offsets := []int64{0, max(0, size/2-oversizeFileSampleBytes/2), max(0, size-oversizeFileSampleBytes)}
	lastOffset := int64(-1)
	for _, offset := range offsets {
		if offset == lastOffset {
			continue
		}
		lastOffset = offset
		length := min(oversizeFileSampleBytes, size-offset)
		buffer := make([]byte, int(length))
		read, err := file.ReadAt(buffer, offset)
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		if int64(read) != length {
			return "", errors.New("oversize file size changed while sampling")
		}
		writeHashSection(hasher, "offset", []byte(strconv.FormatInt(offset, 10)))
		writeHashSection(hasher, "sample", buffer)
	}
	return hashDigest(hasher), nil
}

func observePath(root *os.Root, item statusPath) PathObservation {
	result := PathObservation{Path: item.path, Category: item.category, State: item.state}
	name := filepath.FromSlash(item.path)
	info, err := root.Lstat(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			result.Observation = "missing"
		} else {
			result.Observation = "unreadable"
		}
		return result
	}
	result.Size = int64Pointer(info.Size())

	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, readErr := root.Readlink(name)
		if readErr != nil {
			result.Observation = "unreadable"
			return result
		}
		result.Observation = "symlink"
		result.Size = int64Pointer(int64(len([]byte(target))))
		result.ContentSHA256 = digestBytes([]byte(target))
		return result
	case !info.Mode().IsRegular():
		result.Observation = "non_regular"
		return result
	}

	file, err := root.Open(name)
	if err != nil {
		result.Observation = "unreadable"
		return result
	}
	openedInfo, statErr := file.Stat()
	if statErr != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		_ = file.Close()
		result.Observation = "unreadable"
		return result
	}

	observation := "regular"
	var contentSHA256 string
	var readErr error
	if openedInfo.Size() > MaxObservedFileBytes {
		observation = "oversize"
		contentSHA256, readErr = hashOversizeFileSamples(file, openedInfo.Size())
	} else {
		hasher := sha256.New()
		var written int64
		written, readErr = io.Copy(hasher, io.LimitReader(file, openedInfo.Size()+1))
		if readErr == nil && written == openedInfo.Size() {
			contentSHA256 = hashDigest(hasher)
		} else if readErr == nil {
			readErr = errors.New("regular file size changed while hashing")
		}
	}

	finalInfo, finalStatErr := file.Stat()
	closeErr := file.Close()
	currentInfo, currentErr := root.Lstat(name)
	if readErr != nil || finalStatErr != nil || closeErr != nil || currentErr != nil ||
		!finalInfo.Mode().IsRegular() || !currentInfo.Mode().IsRegular() ||
		!os.SameFile(openedInfo, finalInfo) || !os.SameFile(openedInfo, currentInfo) ||
		openedInfo.Size() != finalInfo.Size() || currentInfo.Size() != finalInfo.Size() ||
		!openedInfo.ModTime().Equal(finalInfo.ModTime()) || !currentInfo.ModTime().Equal(finalInfo.ModTime()) {
		result.Observation = "unreadable"
		result.ContentSHA256 = ""
		return result
	}
	result.Observation = observation
	result.Size = int64Pointer(finalInfo.Size())
	result.ContentSHA256 = contentSHA256
	return result
}

func estimatedPathObservationJSONBytes(item PathObservation) int {
	// JSON may expand a UTF-8 path through escaping. This conservative estimate
	// covers both the dirty_files entry and the duplicate path in the structured
	// observation, plus fixed keys and metadata.
	return 12*len(item.Path) + 6*(len(item.Category)+len(item.State)+len(item.Observation)) +
		len(item.ContentSHA256) + 256
}

func writePathObservationHash(hasher hash.Hash, item PathObservation) {
	writeHashSection(hasher, "path", []byte(item.Path))
	writeHashSection(hasher, "category", []byte(item.Category))
	writeHashSection(hasher, "state", []byte(item.State))
	writeHashSection(hasher, "observation", []byte(item.Observation))
	size := "unknown"
	if item.Size != nil {
		size = strconv.FormatInt(*item.Size, 10)
	}
	writeHashSection(hasher, "size", []byte(size))
	writeHashSection(hasher, "content_sha256", []byte(item.ContentSHA256))
}

func pathObservationFingerprint(items []PathObservation) string {
	hasher := sha256.New()
	writeHashSection(hasher, "path_count", []byte(strconv.Itoa(len(items))))
	for _, item := range items {
		writePathObservationHash(hasher, item)
	}
	return hashDigest(hasher)
}

func gitSnapshotHash(observation GitObservation) string {
	hasher := sha256.New()
	writeHashSection(hasher, "version", []byte(strconv.Itoa(observation.Version)))
	writeHashSection(hasher, "head", []byte(observation.Head))
	writeHashSection(hasher, "index_fingerprint", []byte(observation.IndexFingerprint))
	writeHashSection(hasher, "status_fingerprint", []byte(observation.StatusFingerprint))
	writeHashSection(hasher, "path_count", []byte(strconv.Itoa(observation.PathCount)))
	writeHashSection(hasher, "path_fingerprint", []byte(observation.PathFingerprint))
	writeHashSection(hasher, "details_truncated", []byte(strconv.FormatBool(observation.DetailsTruncated)))
	writeHashSection(hasher, "retained_path_count", []byte(strconv.Itoa(len(observation.PathObservations))))
	for _, item := range observation.PathObservations {
		writePathObservationHash(hasher, item)
	}
	return hashDigest(hasher)
}

func writeHashSection(hasher hash.Hash, name string, content []byte) {
	_, _ = fmt.Fprintf(hasher, "\x00%s\x00%d\x00", name, len(content))
	_, _ = hasher.Write(content)
}

func digestBytes(content []byte) string {
	digest := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func hashDigest(hasher hash.Hash) string {
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}

func validSHA256(value string) bool {
	if len(value) != len("sha256:")+sha256.Size*2 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, character := range value[len("sha256:"):] {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func int64Pointer(value int64) *int64 {
	return &value
}

func gitOutput(root string, args ...string) (string, error) {
	output, err := gitBytes(root, args...)
	if err != nil {
		return "", err
	}
	if len(output) > 0 && output[len(output)-1] == '\n' {
		output = output[:len(output)-1]
		if len(output) > 0 && output[len(output)-1] == '\r' {
			output = output[:len(output)-1]
		}
	}
	return string(output), nil
}

func gitBytes(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...) // #nosec G204 -- fixed git executable with internal argument sets; no shell interpretation.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), detail)
	}
	return stdout.Bytes(), nil
}

func discover(start string) (fsutil.GitRepository, error) {
	return fsutil.DiscoverGit(start)
}
