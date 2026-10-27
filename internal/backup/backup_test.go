package backup

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var timeZero = time.Unix(0, 0)

type tarEntry struct {
	name string
	typ  byte
	mode int64
	body string
	link string
}

func writeArchive(t *testing.T, dir string, entries []tarEntry) string {
	t.Helper()
	p := filepath.Join(dir, "backup-20260101T000000Z.tar.gz")
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		mode := e.mode
		if mode == 0 {
			mode = 0o644
		}
		hdr := &tar.Header{
			Name:     e.name,
			Typeflag: e.typ,
			Mode:     mode,
			Size:     int64(len(e.body)),
			Linkname: e.link,
		}
		if e.typ == tar.TypeDir {
			hdr.Size = 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header %q: %v", e.name, err)
		}
		if len(e.body) > 0 {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatalf("write body %q: %v", e.name, err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
	return p
}

func TestCreateRestoreRoundTrip(t *testing.T) {
	src := t.TempDir()
	stackDir := filepath.Join(src, "stack")
	configDir := filepath.Join(src, "config")
	backupDir := filepath.Join(src, "backups")

	writeFile(t, filepath.Join(stackDir, "docker-compose.yml"), "services: {}\n", 0o644)
	writeFile(t, filepath.Join(stackDir, "prometheus", "prometheus.yml"), "global: {}\n", 0o644)
	writeFile(t, filepath.Join(configDir, "config.yaml"), "stack: here\n", 0o600)

	archive, err := Create(backupDir, Root{Name: "stack", Path: stackDir}, Root{Name: "config", Path: configDir})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	dstStack := filepath.Join(t.TempDir(), "stack")
	dstConfig := filepath.Join(t.TempDir(), "config")
	restored, err := Restore(archive, map[string]string{"stack": dstStack, "config": dstConfig})
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if len(restored) != 3 {
		t.Fatalf("restored %d files, want 3: %v", len(restored), restored)
	}

	cases := map[string]string{
		filepath.Join(dstStack, "docker-compose.yml"):           "services: {}\n",
		filepath.Join(dstStack, "prometheus", "prometheus.yml"): "global: {}\n",
		filepath.Join(dstConfig, "config.yaml"):                 "stack: here\n",
	}
	for p, want := range cases {
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", p, got, want)
		}
	}
}

func TestCreateReturnsErrEmpty(t *testing.T) {
	src := t.TempDir()
	empty := filepath.Join(src, "empty")
	if err := os.MkdirAll(empty, 0o750); err != nil {
		t.Fatal(err)
	}
	backupDir := filepath.Join(src, "backups")

	if _, err := Create(backupDir, Root{Name: "stack", Path: empty}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("Create on empty tree = %v, want ErrEmpty", err)
	}
	infos, err := List(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 0 {
		t.Errorf("left %d archives behind after ErrEmpty", len(infos))
	}
}

func TestCreateSkipsMissingRoot(t *testing.T) {
	src := t.TempDir()
	stackDir := filepath.Join(src, "stack")
	writeFile(t, filepath.Join(stackDir, "docker-compose.yml"), "x", 0o644)

	archive, err := Create(filepath.Join(src, "backups"),
		Root{Name: "config", Path: filepath.Join(src, "does-not-exist")},
		Root{Name: "stack", Path: stackDir},
	)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := os.Stat(archive); err != nil {
		t.Fatalf("archive missing: %v", err)
	}
}

func TestRestoreRejectsTraversal(t *testing.T) {
	src := t.TempDir()
	archive := writeArchive(t, src, []tarEntry{
		{name: "stack/../../evil.txt", typ: tar.TypeReg, body: "pwned"},
	})

	dst := filepath.Join(src, "dst", "stack")
	_, err := Restore(archive, map[string]string{"stack": dst})
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("Restore = %v, want ErrUnsafePath", err)
	}
	if _, err := os.Stat(filepath.Join(src, "evil.txt")); !os.IsNotExist(err) {
		t.Fatal("traversal wrote a file outside the destination")
	}
}

func TestRestoreRejectsAbsolutePath(t *testing.T) {
	src := t.TempDir()
	archive := writeArchive(t, src, []tarEntry{
		{name: "/etc/passwd", typ: tar.TypeReg, body: "pwned"},
	})

	dst := filepath.Join(src, "dst", "stack")
	if _, err := Restore(archive, map[string]string{"stack": dst}); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("Restore = %v, want ErrUnsafePath", err)
	}
}

func TestRestoreRejectsUnsupportedEntries(t *testing.T) {
	src := t.TempDir()
	archive := writeArchive(t, src, []tarEntry{
		{name: "stack/link", typ: tar.TypeSymlink, link: "/etc/passwd"},
	})

	dst := filepath.Join(src, "dst", "stack")
	if _, err := Restore(archive, map[string]string{"stack": dst}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Restore = %v, want ErrUnsupported", err)
	}
}

func TestRestoreRejectsUnknownRoot(t *testing.T) {
	src := t.TempDir()
	archive := writeArchive(t, src, []tarEntry{
		{name: "other/file.txt", typ: tar.TypeReg, body: "x"},
	})

	dst := filepath.Join(src, "dst", "stack")
	if _, err := Restore(archive, map[string]string{"stack": dst}); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("Restore = %v, want ErrUnsafePath", err)
	}
	if _, err := os.Stat(filepath.Join(src, "dst", "other")); !os.IsNotExist(err) {
		t.Fatal("unknown root was extracted")
	}
}

func TestRestoreRejectsOversizedEntry(t *testing.T) {
	src := t.TempDir()
	p := filepath.Join(src, "backup-20260101T000000Z.tar.gz")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Name:     "stack/huge.bin",
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     MaxArchiveBytes + 1,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	// Deliberately skip the body and close straight away: Restore must reject
	// the header before it tries to read the payload.
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(src, "dst", "stack")
	if _, err := Restore(p, map[string]string{"stack": dst}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Restore = %v, want ErrTooLarge", err)
	}
}

func TestRestoreRejectsCorruptArchiveWithoutPartialWrite(t *testing.T) {
	src := t.TempDir()
	good := writeArchive(t, src, []tarEntry{
		{name: "stack/a.txt", typ: tar.TypeReg, body: "aaaa"},
		{name: "stack/b.txt", typ: tar.TypeReg, body: "bbbb"},
	})

	data, err := os.ReadFile(good)
	if err != nil {
		t.Fatal(err)
	}
	truncated := filepath.Join(src, "backup-20260101T000001Z.tar.gz")
	if err := os.WriteFile(truncated, data[:len(data)/2], 0o600); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(src, "dst", "stack")
	if _, err := Restore(truncated, map[string]string{"stack": dst}); err == nil {
		t.Fatal("Restore accepted a truncated archive")
	}
	if _, err := os.Stat(filepath.Join(dst, "a.txt")); !os.IsNotExist(err) {
		t.Error("a truncated archive left a partially restored file behind")
	}
}

func TestRestoreAppliesSafePermissions(t *testing.T) {
	src := t.TempDir()
	archive := writeArchive(t, src, []tarEntry{
		{name: "stack/world-writable.txt", typ: tar.TypeReg, mode: 0o666, body: "x"},
		{name: "stack/script.sh", typ: tar.TypeReg, mode: 0o777, body: "#!/bin/sh\n"},
		{name: "stack/private.txt", typ: tar.TypeReg, mode: 0o600, body: "secret-ish\n"},
		{name: "stack/setuid", typ: tar.TypeReg, mode: 0o4755, body: "x"},
	})

	dst := filepath.Join(src, "dst", "stack")
	if _, err := Restore(archive, map[string]string{"stack": dst}); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	for name, want := range map[string]os.FileMode{
		"world-writable.txt": 0o644,
		"script.sh":          0o755,
		"private.txt":        0o600,
		"setuid":             0o755,
	} {
		info, err := os.Stat(filepath.Join(dst, name))
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s mode = %o, want %o", name, got, want)
		}
	}
}

func TestCreateRestorePreservesOwnerOnlyPermissions(t *testing.T) {
	src := t.TempDir()
	stackDir := filepath.Join(src, "stack")
	writeFile(t, filepath.Join(stackDir, "private.env"), "TOKEN=placeholder\n", 0o600)

	archive, err := Create(filepath.Join(src, "backups"), Root{Name: "stack", Path: stackDir})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	dst := filepath.Join(src, "dst", "stack")
	if _, err := Restore(archive, map[string]string{"stack": dst}); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	info, err := os.Stat(filepath.Join(dst, "private.env"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("restored mode = %o, want 0600 (must not widen owner-only files)", got)
	}
}

func TestCreateTwiceInSameSecondKeepsBoth(t *testing.T) {
	src := t.TempDir()
	stackDir := filepath.Join(src, "stack")
	writeFile(t, filepath.Join(stackDir, "docker-compose.yml"), "x", 0o644)
	dir := filepath.Join(src, "backups")

	first, err := Create(dir, Root{Name: "stack", Path: stackDir})
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}
	second, err := Create(dir, Root{Name: "stack", Path: stackDir})
	if err != nil {
		t.Fatalf("second Create: %v", err)
	}
	if first == second {
		t.Fatal("a second backup in the same second overwrote the first")
	}

	infos, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 {
		t.Fatalf("List returned %d archives, want 2", len(infos))
	}
	if infos[0].Path != second {
		t.Errorf("newest archive = %s, want %s", infos[0].Path, second)
	}
}

func TestListSortsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "backup-20260101T000000Z.tar.gz"), "old", 0o600)
	writeFile(t, filepath.Join(dir, "backup-20260301T000000Z.tar.gz"), "new", 0o600)
	writeFile(t, filepath.Join(dir, "not-a-backup.txt"), "x", 0o600)

	if err := os.Chtimes(filepath.Join(dir, "backup-20260101T000000Z.tar.gz"), timeZero, timeZero); err != nil {
		t.Fatal(err)
	}

	infos, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("List returned %d entries, want 2", len(infos))
	}
	if filepath.Base(infos[0].Path) != "backup-20260301T000000Z.tar.gz" {
		t.Errorf("newest entry = %s", filepath.Base(infos[0].Path))
	}
}

func TestListMissingDir(t *testing.T) {
	infos, err := List(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("List on missing dir: %v", err)
	}
	if len(infos) != 0 {
		t.Errorf("got %d entries, want 0", len(infos))
	}
}

func writeFile(t *testing.T, p, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}
