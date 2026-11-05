package fsutil

import (
	"os"
	"path/filepath"
)

// AtomicWrite replaces path with data atomically.
//
// The data is written to a uniquely named temporary file in the same
// directory, fsynced, renamed over path and the directory is fsynced again.
// A crash therefore leaves either the old content or the new content, never a
// truncated file, and two concurrent writers cannot share the temporary name.
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	f, err := os.CreateTemp(dir, ".orvyn-tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	renamed := false
	defer func() {
		if !renamed {
			_ = os.Remove(tmp)
		}
	}()

	if err := f.Chmod(perm); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	renamed = true

	return syncDir(dir)
}

// syncDir makes the rename itself durable. Without it a crash shortly after
// AtomicWrite returns could still lose the directory entry.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	if err := d.Sync(); err != nil {
		d.Close()
		return err
	}
	return d.Close()
}
