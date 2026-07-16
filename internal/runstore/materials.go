package runstore

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/signalridge/slipway/internal/fsutil"
)

const (
	materialsDirectoryName = "materials"
	maxMaterialBytes       = 256 << 10
	materialDigestDomain   = "slipway-material/v1"
)

// Material is an immutable, content-addressed Run input. Digest uses the
// lowercase sha256:<hex> machine-protocol representation.
type Material struct {
	Digest string
	Data   []byte
}

// MaterialReader reads immutable Run material while the authoritative per-Run
// journal lock remains held. It is valid only during the callback that receives
// it.
type MaterialReader func(digest string) ([]byte, error)

type materialGuard struct {
	run      *runHandle
	root     *os.Root
	identity os.FileInfo
}

func openMaterialGuard(run *runHandle, create bool) (*materialGuard, bool, error) {
	if err := run.validate(); err != nil {
		return nil, false, err
	}
	if create && run.store.readOnly {
		return nil, false, ErrReadOnly
	}
	var root *os.Root
	var identity os.FileInfo
	var created bool
	var err error
	if run.store.readOnly {
		root, identity, err = openChildReadOnly(run.root, materialsDirectoryName)
	} else {
		root, identity, created, err = openPrivateChild(run.root, materialsDirectoryName, create)
	}
	if err != nil {
		return nil, false, err
	}
	guard := &materialGuard{run: run, root: root, identity: identity}
	if err := guard.validate(); err != nil {
		_ = root.Close()
		return nil, false, err
	}
	return guard, created, nil
}

func (guard *materialGuard) validate() error {
	if guard == nil {
		return nil
	}
	if err := guard.run.validate(); err != nil {
		return err
	}
	if err := validateDirectoryIdentity(guard.run.root, materialsDirectoryName, guard.identity); err != nil {
		return fmt.Errorf("run materials directory changed after opening: %w", err)
	}
	if err := validateOpenedDirectoryRoot(guard.root, guard.identity); err != nil {
		return fmt.Errorf("opened run materials directory changed: %w", err)
	}
	return nil
}

func (guard *materialGuard) close() error {
	if guard == nil || guard.root == nil {
		return nil
	}
	err := guard.root.Close()
	guard.root = nil
	return err
}

// PutMaterials makes all supplied materials durable before returning. Existing
// matching blobs are accepted idempotently; mismatched blobs fail closed.
func (store *Store) PutMaterials(runID string, materials []Material) error {
	if err := store.requireWritable(); err != nil {
		return err
	}
	if len(materials) == 0 {
		return nil
	}
	run, err := store.openRunRoot(runID)
	if err != nil {
		return err
	}
	defer run.Close()
	return withRunLock(run, nil, func(_ *runTransaction) error {
		guard, err := store.putMaterials(run, materials)
		if err != nil {
			return err
		}
		return errors.Join(guard.validate(), guard.close())
	})
}

func (store *Store) putMaterials(run *runHandle, materials []Material) (*materialGuard, error) {
	if len(materials) == 0 {
		return nil, nil
	}

	unique := make([]Material, 0, len(materials))
	seen := make(map[string]struct{}, len(materials))
	for _, material := range materials {
		filename, err := materialFilename(material.Digest)
		if err != nil {
			return nil, err
		}
		if len(material.Data) == 0 || len(material.Data) > maxMaterialBytes {
			return nil, fmt.Errorf("material %s must contain 1..%d bytes", material.Digest, maxMaterialBytes)
		}
		if materialDigest(material.Data) != material.Digest {
			return nil, fmt.Errorf("material %s digest does not match content", material.Digest)
		}
		if _, duplicate := seen[filename]; duplicate {
			continue
		}
		seen[filename] = struct{}{}
		unique = append(unique, material)
	}

	guard, created, err := openMaterialGuard(run, true)
	if err != nil {
		return nil, fmt.Errorf("open run materials: %w", err)
	}
	fail := func(err error) (*materialGuard, error) {
		return nil, errors.Join(err, guard.close())
	}
	if created {
		if err := syncNewDirectory(
			guard.root,
			guard.identity,
			run.root,
			run.identity,
			materialsDirectoryName,
			store.hooks,
			faultSyncRunDirectory,
			faultSyncRunDirectory,
		); err != nil {
			return fail(fmt.Errorf("sync run materials directory: %w", err))
		}
	}

	for _, material := range unique {
		filename, _ := materialFilename(material.Digest)
		if err := putMaterial(guard, filename, material); err != nil {
			return fail(err)
		}
	}
	if err := syncAnchoredDirectory(guard.root, guard.identity, store.hooks, faultSyncRunDirectory); err != nil {
		return fail(fmt.Errorf("sync run materials: %w", err))
	}
	if err := guard.validate(); err != nil {
		return fail(err)
	}
	return guard, nil
}

func putMaterial(guard *materialGuard, filename string, material Material) error {
	if err := guard.validate(); err != nil {
		return err
	}
	root := guard.root
	existing, err := inspectRegularFileOrMissingInRoot(root, filename)
	if err != nil {
		return fmt.Errorf("inspect material %s: %w", material.Digest, err)
	}
	if existing.exists {
		data, readErr := readMaterialFile(root, filename, material.Digest)
		if readErr != nil {
			return readErr
		}
		if !bytes.Equal(data, material.Data) {
			return fmt.Errorf("material %s content conflicts with existing blob", material.Digest)
		}
		return guard.validate()
	}

	temporary, file, err := createTemporaryFileInRoot(root, filename, 0o600)
	if err != nil {
		return fmt.Errorf("create material temp: %w", err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = file.Close()
		}
		_ = root.Remove(temporary)
	}()
	if err := verifyOpenedRegularFileInRoot(root, temporary, file); err != nil {
		return fmt.Errorf("verify material temp: %w", err)
	}
	written, err := file.Write(material.Data)
	if err != nil {
		return fmt.Errorf("write material temp: %w", err)
	}
	if written != len(material.Data) {
		return io.ErrShortWrite
	}
	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("secure material temp: %w", err)
	}
	if err := verifyOpenedRegularFileInRoot(root, temporary, file); err != nil {
		return fmt.Errorf("verify written material temp: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync material temp: %w", err)
	}
	if err := verifyOpenedRegularFileInRoot(root, temporary, file); err != nil {
		return fmt.Errorf("verify synced material temp: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close material temp: %w", err)
	}
	closed = true

	if err := guard.validate(); err != nil {
		return err
	}
	if err := fsutil.RenameNoReplace(root, temporary, filename); err != nil {
		current, inspectErr := inspectRegularFileOrMissingInRoot(root, filename)
		if inspectErr == nil && current.exists {
			data, readErr := readMaterialFile(root, filename, material.Digest)
			if readErr == nil && bytes.Equal(data, material.Data) {
				return guard.validate()
			}
			if readErr != nil {
				return readErr
			}
			return fmt.Errorf("material %s content conflicts with concurrent blob", material.Digest)
		}
		return errors.Join(fmt.Errorf("commit material %s without replacement: %w", material.Digest, err), inspectErr)
	}
	if err := guard.validate(); err != nil {
		return err
	}
	_, err = readMaterialFile(root, filename, material.Digest)
	return err
}

// ReadMaterial returns a bounded blob after revalidating its content digest.
func (store *Store) ReadMaterial(runID, digest string) ([]byte, error) {
	run, err := store.openRunRoot(runID)
	if err != nil {
		return nil, err
	}
	defer run.Close()
	return store.readMaterial(run, digest)
}

// VisitWithMaterialReader replays the authoritative journal and performs a
// material read callback under the same per-Run lock. Concurrent mutations
// cannot make an authorization decision stale before the material bytes are
// read.
func (store *Store) VisitWithMaterialReader(
	runID string,
	consume func(Event) error,
	callback func(MaterialReader) error,
) error {
	if err := store.requireWritable(); err != nil {
		return err
	}
	if consume == nil || callback == nil {
		return errors.New("journal consumer and material callback are required")
	}
	run, err := store.openRunRoot(runID)
	if err != nil {
		return err
	}
	defer run.Close()
	return withRunLock(run, nil, func(transaction *runTransaction) error {
		if _, err := visitJournal(transaction.journalContext(), journalFileName, consume); err != nil {
			return err
		}
		return callback(func(digest string) ([]byte, error) {
			return store.readMaterial(run, digest)
		})
	})
}

func (store *Store) readMaterial(run *runHandle, digest string) ([]byte, error) {
	filename, err := materialFilename(digest)
	if err != nil {
		return nil, err
	}
	guard, _, err := openMaterialGuard(run, false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("material %s not found", digest)
	}
	if err != nil {
		return nil, fmt.Errorf("open run materials: %w", err)
	}
	defer guard.close()
	data, err := readMaterialFile(guard.root, filename, digest)
	if err != nil {
		return nil, err
	}
	if err := guard.validate(); err != nil {
		return nil, err
	}
	return data, nil
}

func readMaterialFile(root *os.Root, filename, digest string) ([]byte, error) {
	file, _, err := openRegularFileInRoot(root, filename, os.O_RDONLY, 0o600, false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("material %s not found", digest)
	}
	if err != nil {
		return nil, fmt.Errorf("open material %s: %w", digest, err)
	}
	defer file.Close()
	reader := io.LimitReader(file, maxMaterialBytes+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read material %s: %w", digest, err)
	}
	if len(data) == 0 || len(data) > maxMaterialBytes {
		return nil, fmt.Errorf("material %s has invalid size", digest)
	}
	if err := verifyOpenedRegularFileInRoot(root, filename, file); err != nil {
		return nil, fmt.Errorf("verify material %s: %w", digest, err)
	}
	if materialDigest(data) != digest {
		return nil, fmt.Errorf("material %s is corrupt", digest)
	}
	return data, nil
}

func materialFilename(digest string) (string, error) {
	if !strings.HasPrefix(digest, "sha256:") || len(digest) != len("sha256:")+sha256.Size*2 {
		return "", errors.New("material digest must use lowercase sha256:<64 hex> format")
	}
	hexDigest := strings.TrimPrefix(digest, "sha256:")
	decoded, err := hex.DecodeString(hexDigest)
	if err != nil || len(decoded) != sha256.Size || hex.EncodeToString(decoded) != hexDigest {
		return "", errors.New("material digest must use lowercase sha256:<64 hex> format")
	}
	return hexDigest, nil
}

func materialDigest(data []byte) string {
	hasher := sha256.New()
	writeFramedMaterialField(hasher, []byte(materialDigestDomain))
	writeFramedMaterialField(hasher, data)
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}

func writeFramedMaterialField(writer io.Writer, value []byte) {
	var prefix [8]byte
	binary.BigEndian.PutUint64(prefix[:], uint64(len(value)))
	_, _ = writer.Write(prefix[:])
	_, _ = writer.Write(value)
}
