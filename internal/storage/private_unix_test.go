//go:build unix

package storage_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/make-no-mistakes-team/super-duper-calculator/internal/storage"
)

func TestOpenRejectsInsecureDirectoryBeforeCreatingDatabase(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "shared")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0o750); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "records.sqlite")
	db, err := storage.Open(t.Context(), path)
	if db != nil {
		t.Cleanup(func() { _ = db.Close() })
	}
	if err == nil {
		t.Fatal("group-accessible directory unexpectedly accepted")
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rejected path created a database: %v", err)
	}
	info, err := os.Lstat(parent)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o750 {
		t.Fatalf("directory permissions changed to %04o", info.Mode().Perm())
	}
}

func TestOpenRejectsInsecureDatabaseAndSidecarWithoutModification(t *testing.T) {
	for _, tc := range []struct {
		name   string
		unsafe string
	}{
		{"database", ""},
		{"WAL", "-wal"},
		{"shared memory", "-shm"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := privateDatabasePath(t, "records.sqlite")
			unsafePath := path + tc.unsafe
			original := []byte("leave these bytes untouched")
			if tc.unsafe == "" {
				// An empty file is a valid new SQLite database. Without the
				// privacy check, Open would initialize it successfully.
				original = nil
			}
			if err := os.WriteFile(unsafePath, original, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(unsafePath, 0o640); err != nil {
				t.Fatal(err)
			}
			db, err := storage.Open(t.Context(), path)
			if db != nil {
				t.Cleanup(func() { _ = db.Close() })
			}
			if err == nil {
				t.Fatal("group-readable file unexpectedly accepted")
			}
			if strings.Contains(err.Error(), path) {
				t.Fatalf("file error exposes database path: %v", err)
			}
			contents, err := os.ReadFile(unsafePath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(contents, original) {
				t.Fatal("rejected file was modified")
			}
			info, err := os.Lstat(unsafePath)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o640 {
				t.Fatalf("file permissions changed to %04o", info.Mode().Perm())
			}
			if tc.unsafe != "" {
				if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("sidecar rejection created a database: %v", err)
				}
			}
		})
	}
}

func TestOpenRejectsSymlinkedDatabaseWithoutFollowingIt(t *testing.T) {
	path := privateDatabasePath(t, "records.sqlite")
	target := filepath.Join(filepath.Dir(path), "target.sqlite")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	db, err := storage.Open(t.Context(), path)
	if db != nil {
		t.Cleanup(func() { _ = db.Close() })
	}
	if err == nil {
		t.Fatal("symlinked database unexpectedly accepted")
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("rejected symlink was changed: %v", err)
	}
	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 0 {
		t.Fatal("symlink target was modified")
	}
}
