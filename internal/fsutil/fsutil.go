package fsutil

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var ErrPathTraversal = errors.New("path traversal detected")
var ErrOutsideBase = errors.New("path outside allowed base")

// SafeJoin ensures the resulting path is under base and does not contain traversal
func SafeJoin(base, path string) (string, error) {
	if strings.Contains(path, "..") {
		return "", ErrPathTraversal
	}
	cleaned := filepath.Clean(path)
	if filepath.IsAbs(cleaned) {
		rel, err := filepath.Rel(base, cleaned)
		if err != nil {
			return "", err
		}
		if strings.HasPrefix(rel, "..") {
			return "", ErrOutsideBase
		}
		return cleaned, nil
	}
	joined := filepath.Join(base, cleaned)
	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil {
		if os.IsNotExist(err) {
			rel, err2 := filepath.Rel(base, joined)
			if err2 != nil {
				return "", err2
			}
			if strings.HasPrefix(rel, "..") {
				return "", ErrOutsideBase
			}
			return joined, nil
		}
		return "", err
	}
	baseResolved, err := filepath.EvalSymlinks(base)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(baseResolved, resolved)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") {
		return "", ErrOutsideBase
	}
	return resolved, nil
}

// AtomicWrite writes data to a file atomically
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
