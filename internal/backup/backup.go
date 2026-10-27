// Package backup creates and restores timestamped archives of Orvyn's stack
// and configuration directories.
//
// Archives are plain gzip-compressed tar files so they can be inspected with
// standard tools. Restore is fail-closed: the archive is fully validated and
// unpacked into a temporary directory before anything is written to the final
// destination, and entries that could escape it are rejected.
package backup

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	// MaxArchiveBytes bounds both the archive size accepted for restore and the
	// total number of bytes a restore is allowed to write.
	MaxArchiveBytes int64 = 256 << 20

	maxEntries = 5000

	archivePrefix = "backup-"
	archiveSuffix = ".tar.gz"
)

var (
	ErrEmpty       = errors.New("nothing to back up")
	ErrUnsafePath  = errors.New("archive contains an unsafe path")
	ErrUnsupported = errors.New("archive contains an unsupported entry")
	ErrTooLarge    = errors.New("archive exceeds the size limit")
)

// Root is a directory (or file) to archive under a stable name.
type Root struct {
	Name string
	Path string
}

// Info describes a backup file on disk.
type Info struct {
	Path    string
	Size    int64
	ModTime time.Time
}

// Create archives every root into dir and returns the path of the new archive.
// It returns ErrEmpty when there is nothing worth archiving, in which case no
// file is left behind.
func Create(dir string, roots ...Root) (string, error) {
	if len(roots) == 0 {
		return "", ErrEmpty
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}

	base := archivePrefix + time.Now().UTC().Format("20060102T150405Z")

	// Two backups requested inside the same second would otherwise collide on
	// the timestamp. Pick the first free name instead of overwriting.
	var (
		f     *os.File
		final string
		tmp   string
	)
	const maxAttempts = 10
	for attempt := 0; attempt < maxAttempts; attempt++ {
		name := base + archiveSuffix
		if attempt > 0 {
			name = fmt.Sprintf("%s-%d%s", base, attempt, archiveSuffix)
		}
		final = filepath.Join(dir, name)
		tmp = final + ".tmp"

		if _, err := os.Lstat(final); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return "", err
		}

		var err error
		f, err = os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			break
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", err
		}
	}
	if f == nil {
		return "", fmt.Errorf("backup: %d archives already exist for %s", maxAttempts, base)
	}
	discard := func() {
		f.Close()
		os.Remove(tmp)
	}

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	written := 0
	for _, root := range roots {
		if root.Name == "" || root.Name == "." || strings.ContainsAny(root.Name, `/\`) {
			discard()
			return "", fmt.Errorf("%w: invalid root name %q", ErrUnsafePath, root.Name)
		}
		if err := addRoot(tw, root, &written); err != nil {
			discard()
			return "", err
		}
	}

	if written == 0 {
		discard()
		return "", ErrEmpty
	}
	if err := tw.Close(); err != nil {
		discard()
		return "", err
	}
	if err := gz.Close(); err != nil {
		discard()
		return "", err
	}
	if err := f.Sync(); err != nil {
		discard()
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return final, nil
}

func addRoot(tw *tar.Writer, root Root, written *int) error {
	info, err := os.Lstat(root.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return nil
		}
		return addFile(tw, root.Path, root.Name, written)
	}

	return filepath.WalkDir(root.Path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root.Path {
			return nil
		}
		rel, err := filepath.Rel(root.Path, p)
		if err != nil {
			return err
		}
		entry := path.Join(root.Name, filepath.ToSlash(rel))

		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return tw.WriteHeader(&tar.Header{
				Typeflag: tar.TypeDir,
				Name:     entry + "/",
				Mode:     int64(0o750),
				ModTime:  info.ModTime(),
			})
		}
		if !d.Type().IsRegular() {
			// Symlinks, sockets and devices have no place in a config backup.
			return nil
		}
		return addFile(tw, p, entry, written)
	})
}

func addFile(tw *tar.Writer, src, name string, written *int) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}

	hdr := &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     name,
		Mode:     int64(exportMode(info.Mode())),
		Size:     info.Size(),
		ModTime:  info.ModTime(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	// Lstat above already skipped symlinks, but the path could be swapped for one
	// between that call and Open. Only copy when the opened handle is the same
	// regular file that was sized, so a race cannot pull in an outside file.
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() || !os.SameFile(info, fi) {
		return fmt.Errorf("%w: %q changed while being archived", ErrUnsafePath, src)
	}

	if _, err := io.Copy(tw, f); err != nil {
		return err
	}
	*written++
	return nil
}

// List returns the backups found in dir, newest first. A missing directory is
// not an error: it simply means there are no backups yet.
func List(dir string) ([]Info, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	out := make([]Info, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, archivePrefix) || !strings.HasSuffix(name, archiveSuffix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, Info{
			Path:    filepath.Join(dir, name),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].ModTime.Equal(out[j].ModTime) {
			return out[i].ModTime.After(out[j].ModTime)
		}
		// Same-second archives (the -1/-2 suffix) are ordered by that suffix so
		// "newest" stays meaningful.
		return archiveSeq(out[i].Path) > archiveSeq(out[j].Path)
	})
	return out, nil
}

// archiveSeq extracts the numeric disambiguation suffix from a backup name,
// returning 0 for the unsuffixed archive.
func archiveSeq(p string) int {
	name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(p), archivePrefix), archiveSuffix)
	i := strings.LastIndexByte(name, '-')
	if i < 0 {
		return 0
	}
	n, err := strconv.Atoi(name[i+1:])
	if err != nil {
		return 0
	}
	return n
}

// Restore extracts archivePath, mapping each top-level archive entry to the
// destination given in dests. Unknown top-level entries, absolute paths,
// traversal, symlinks and oversized files are rejected.
//
// Files are unpacked into a temporary directory first and only moved into place
// once the whole archive has been validated, so a bad archive cannot leave a
// half-restored tree behind.
func Restore(archivePath string, dests map[string]string) ([]string, error) {
	if len(dests) == 0 {
		return nil, fmt.Errorf("%w: no destinations configured", ErrUnsafePath)
	}

	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if st.Size() > MaxArchiveBytes {
		return nil, ErrTooLarge
	}

	tmpDir, err := os.MkdirTemp("", "orvyn-restore-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	gz, err := gzip.NewReader(io.LimitReader(f, MaxArchiveBytes))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	var (
		files []string
		dirs  []string
		total int64
		count int
	)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		count++
		if count > maxEntries {
			return nil, ErrTooLarge
		}

		rel, err := safeRel(hdr.Name)
		if err != nil {
			return nil, err
		}
		if rel == "" {
			continue
		}
		if _, ok := dests[topLevel(rel)]; !ok {
			return nil, fmt.Errorf("%w: unexpected top-level entry %q", ErrUnsafePath, hdr.Name)
		}
		if hdr.Typeflag == tar.TypeReg && rest(rel) == "" {
			return nil, fmt.Errorf("%w: file entry %q has no parent directory", ErrUnsupported, hdr.Name)
		}

		target, err := withinRoot(tmpDir, rel)
		if err != nil {
			return nil, err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o750); err != nil {
				return nil, err
			}
			dirs = append(dirs, rel)
		case tar.TypeReg:
			if hdr.Size < 0 || hdr.Size > MaxArchiveBytes-total {
				return nil, ErrTooLarge
			}
			total += hdr.Size
			if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
				return nil, err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, importMode(hdr.Mode))
			if err != nil {
				return nil, err
			}
			if _, err := io.CopyN(out, tr, hdr.Size); err != nil {
				out.Close()
				return nil, err
			}
			if err := out.Close(); err != nil {
				return nil, err
			}
			files = append(files, rel)
		default:
			return nil, fmt.Errorf("%w: %q (type %q)", ErrUnsupported, hdr.Name, hdr.Typeflag)
		}
	}

	if len(files) == 0 {
		return nil, ErrEmpty
	}

	for _, rel := range dirs {
		dst, err := withinRoot(dests[topLevel(rel)], rest(rel))
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(dst, 0o750); err != nil {
			return nil, err
		}
	}

	for _, rel := range files {
		src, err := withinRoot(tmpDir, rel)
		if err != nil {
			return nil, err
		}
		dst, err := withinRoot(dests[topLevel(rel)], rest(rel))
		if err != nil {
			return nil, err
		}
		if err := moveFile(src, dst); err != nil {
			return nil, err
		}
	}

	return files, nil
}

// moveFile renames src to dst, falling back to a copy when the temporary
// directory and the destination live on different filesystems.
func moveFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	if !errors.Is(err, syscall.EXDEV) {
		return err
	}

	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(src)
}

// safeRel validates an archive entry name and returns it as a local relative
// path. An empty result means the entry describes the archive root itself.
func safeRel(name string) (string, error) {
	name = strings.ReplaceAll(name, `\`, "/")
	clean := path.Clean(name)

	switch {
	case clean == "." || clean == "/":
		return "", nil
	case strings.HasPrefix(clean, "/"):
		return "", fmt.Errorf("%w: absolute path %q", ErrUnsafePath, name)
	case clean == ".." || strings.HasPrefix(clean, "../"):
		return "", fmt.Errorf("%w: traversal in %q", ErrUnsafePath, name)
	}
	return filepath.FromSlash(clean), nil
}

func withinRoot(root, rel string) (string, error) {
	target := filepath.Join(root, rel)
	if target == root {
		return target, nil
	}
	prefix := root + string(os.PathSeparator)
	if !strings.HasPrefix(target, prefix) {
		return "", fmt.Errorf("%w: %q escapes %q", ErrUnsafePath, rel, root)
	}
	return target, nil
}

func topLevel(rel string) string {
	if i := strings.IndexRune(rel, os.PathSeparator); i >= 0 {
		return rel[:i]
	}
	return rel
}

func rest(rel string) string {
	if i := strings.IndexRune(rel, os.PathSeparator); i >= 0 {
		return rel[i+1:]
	}
	return ""
}

// exportMode records the permission bits for the archive header. Group/other
// write and setuid/setgid are dropped at creation time so a tampered archive
// has nothing to escalate to.
func exportMode(m os.FileMode) os.FileMode {
	return m.Perm() &^ os.FileMode(0o022)
}

// importMode derives the on-disk mode for a restored file. It never widens the
// archived permissions and never grants group/other write, but it does keep the
// owner able to read and write so a later apply can rewrite the file. Explicit
// executable bits survive, since generated scripts rely on them.
func importMode(mode int64) os.FileMode {
	perm := os.FileMode(mode).Perm() &^ os.FileMode(0o022)
	perm |= 0o600
	if os.FileMode(mode).Perm()&0o111 != 0 {
		perm |= 0o111
	}
	return perm
}
