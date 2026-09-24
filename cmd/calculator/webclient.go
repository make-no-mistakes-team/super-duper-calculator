package main

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// withClient serves a built web client alongside the unchanged API and health routes.
// An empty assetRoot selects the web directory next to the running executable.
func withClient(apiHandler http.Handler, assetRoot string) (http.Handler, error) {
	if assetRoot == "" {
		executable, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("locate executable for web assets: %w", err)
		}
		assetRoot = filepath.Join(filepath.Dir(executable), "web")
	}
	root, err := filepath.Abs(assetRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve web assets directory: %w", err)
	}
	if linkedAsset(root, "index.html") {
		return nil, fmt.Errorf("web assets require a regular index.html in %q", root)
	}
	index, err := os.OpenInRoot(root, "index.html")
	if err != nil {
		return nil, fmt.Errorf("web assets require index.html in %q: %w", root, err)
	}
	info, err := index.Stat()
	_ = index.Close()
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return nil, fmt.Errorf("web assets require a nonempty regular index.html in %q", root)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlPath := r.URL.Path
		if urlPath == "/api" || strings.HasPrefix(urlPath, "/api/") ||
			urlPath == "/health" || strings.HasPrefix(urlPath, "/health/") {
			apiHandler.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		rel, ok := clientPath(urlPath)
		if !ok {
			http.NotFound(w, r)
			return
		}
		if rel == "" {
			rel = "index.html"
		}
		if linkedAsset(root, rel) {
			http.NotFound(w, r)
			return
		}
		file, openErr := os.OpenInRoot(root, filepath.FromSlash(rel))
		if openErr != nil {
			// A missing URL without an extension may be a client-side route.
			// Never turn a missing file within a real asset directory into HTML.
			if !errors.Is(openErr, fs.ErrNotExist) || path.Ext(rel) != "" ||
				rel == "assets" || strings.HasPrefix(rel, "assets/") ||
				underAssetDirectory(root, rel) {
				http.NotFound(w, r)
				return
			}
			file, openErr = os.OpenInRoot(root, "index.html")
			if openErr != nil {
				http.NotFound(w, r)
				return
			}
			rel = "index.html"
		}
		defer file.Close()
		info, statErr := file.Stat()
		if statErr != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, path.Base(rel), info.ModTime(), file)
	}), nil
}

func clientPath(urlPath string) (string, bool) {
	if !strings.HasPrefix(urlPath, "/") {
		return "", false
	}
	segments := strings.Split(strings.Trim(urlPath, "/"), "/")
	for _, segment := range segments {
		lower := strings.ToLower(segment)
		if segment == "." || segment == ".." || strings.HasPrefix(segment, ".") ||
			strings.ContainsAny(segment, "\\%\x00:") ||
			lower == "private" || lower == "data" || lower == "state" ||
			databaseAsset(lower) {
			return "", false
		}
	}
	if strings.Contains(urlPath, "//") {
		return "", false
	}
	return strings.Trim(urlPath, "/"), true
}

func databaseAsset(name string) bool {
	if strings.Contains(name, ".sqlite-") || strings.Contains(name, ".db-") {
		return true
	}
	for _, sidecar := range [...]string{"-wal", "-shm", "-journal"} {
		if mainfile, ok := strings.CutSuffix(name, sidecar); ok {
			name = mainfile
			break
		}
	}
	switch path.Ext(name) {
	case ".sqlite", ".sqlite3", ".db", ".db3":
		return true
	default:
		return false
	}
}

func linkedAsset(root, rel string) bool {
	var prefix string
	for _, segment := range strings.Split(rel, "/") {
		prefix = filepath.Join(prefix, segment)
		info, err := os.Lstat(filepath.Join(root, prefix))
		if err != nil {
			return false
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return true
		}
	}
	return false
}

func underAssetDirectory(root, rel string) bool {
	for parent := path.Dir(rel); parent != "."; parent = path.Dir(parent) {
		file, err := os.OpenInRoot(root, filepath.FromSlash(parent))
		if err != nil {
			continue
		}
		info, err := file.Stat()
		_ = file.Close()
		if err == nil && info.IsDir() {
			return true
		}
	}
	return false
}
