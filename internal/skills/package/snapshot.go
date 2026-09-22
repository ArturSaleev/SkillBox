package skillpackage

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Snapshot copies an entire validated package to an immutable directory. If an
// identical snapshot already exists the operation is idempotent.
func Snapshot(packageRoot, snapshotRoot string) (*Package, error) {
	source, err := Load(packageRoot)
	if err != nil {
		return nil, err
	}
	if existing, loadErr := Load(snapshotRoot); loadErr == nil {
		if existing.Hash != source.Hash {
			return nil, fmt.Errorf("immutable snapshot already exists with different hash: %s", snapshotRoot)
		}
		return existing, nil
	} else if !os.IsNotExist(loadErr) {
		return nil, loadErr
	}
	parent := filepath.Dir(snapshotRoot)
	if err = os.MkdirAll(parent, 0o750); err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp(parent, ".snapshot-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if err = copyPackage(source.Root, tmp); err != nil {
		return nil, err
	}
	if err = makeReadOnly(tmp); err != nil {
		return nil, err
	}
	if err = os.Rename(tmp, snapshotRoot); err != nil {
		return nil, fmt.Errorf("commit package snapshot: %w", err)
	}
	return Load(snapshotRoot)
}

// Restore atomically replaces a package with all files from one snapshot.
func Restore(snapshotRoot, packageRoot string) (*Package, error) {
	snapshot, err := Load(snapshotRoot)
	if err != nil {
		return nil, err
	}
	parent := filepath.Dir(packageRoot)
	if err = os.MkdirAll(parent, 0o750); err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp(parent, ".restore-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if err = copyPackage(snapshot.Root, tmp); err != nil {
		return nil, err
	}
	backup, err := os.MkdirTemp(parent, ".backup-*")
	if err != nil {
		return nil, err
	}
	if err = os.Remove(backup); err != nil {
		return nil, err
	}
	if err = os.Rename(packageRoot, backup); err != nil {
		return nil, fmt.Errorf("backup current package: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.Rename(backup, packageRoot)
		}
	}()
	if err = os.Rename(tmp, packageRoot); err != nil {
		return nil, fmt.Errorf("restore package snapshot: %w", err)
	}
	committed = true
	if err = os.RemoveAll(backup); err != nil {
		return nil, fmt.Errorf("remove package backup: %w", err)
	}
	return Load(packageRoot)
}

func copyPackage(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if err = ValidateRelativePath(filepath.ToSlash(rel)); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symlinks are not allowed: %s", ErrUnsafePath, rel)
		}
		target := filepath.Join(destination, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%w: special files are not allowed: %s", ErrUnsafePath, rel)
		}
		if err = os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, err = io.Copy(out, in)
		}
		closeOut := error(nil)
		if out != nil {
			closeOut = out.Close()
		}
		closeIn := in.Close()
		if err != nil {
			return err
		}
		if closeOut != nil {
			return closeOut
		}
		return closeIn
	})
}

func makeReadOnly(root string) error {
	var directories []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			directories = append(directories, path)
			return nil
		}
		return os.Chmod(path, 0o400)
	})
	if err != nil {
		return err
	}
	for i := len(directories) - 1; i >= 0; i-- {
		// Directories stay owner-writable so snapshots can be removed during an
		// aborted transaction; files are read-only and Snapshot never overwrites
		// an existing version.
		if err = os.Chmod(directories[i], 0o700); err != nil {
			return err
		}
	}
	return nil
}
