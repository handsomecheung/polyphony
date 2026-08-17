package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxListEntries = 10_000
	maxListDepth   = 20
	listSuffix     = "/__api__/list"
)

var (
	errListLimit = errors.New("list result limit reached")
	rootDir      string
	listEnabled  bool
	publicAddr   = envOrDefault("DOWNSERVER_PUBLIC_ADDR", ":80")
)

type listEntry struct {
	Path     string    `json:"path"`
	Type     string    `json:"type"`
	Size     int64     `json:"size,omitempty"`
	Modified time.Time `json:"modified"`
}

type listResponse struct {
	Path      string      `json:"path"`
	Entries   []listEntry `json:"entries"`
	Truncated bool        `json:"truncated"`
}

func main() {
	var err error
	rootDir, listEnabled, err = serverConfigFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	public := http.NewServeMux()
	public.HandleFunc("/ping", ok)
	public.HandleFunc("/", serveFile)

	log.Printf("serving files on %s (list enabled: %t)", publicAddr, listEnabled)
	log.Fatal(http.ListenAndServe(publicAddr, public))
}

func serverConfigFromEnv() (string, bool, error) {
	dirRoot, ok := os.LookupEnv("DOWNSERVER_DIR_ROOT")
	if !ok || strings.TrimSpace(dirRoot) == "" {
		return "", false, errors.New("DOWNSERVER_DIR_ROOT must be set")
	}

	listValue, ok := os.LookupEnv("DOWNSERVER_ENABLE_LIST")
	if !ok || strings.TrimSpace(listValue) == "" {
		return "", false, errors.New("DOWNSERVER_ENABLE_LIST must be set")
	}
	switch strings.ToLower(strings.TrimSpace(listValue)) {
	case "true":
		return dirRoot, true, nil
	case "false":
		return dirRoot, false, nil
	default:
		return "", false, errors.New("DOWNSERVER_ENABLE_LIST must be true or false")
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func ok(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK\n"))
}

func list(w http.ResponseWriter, r *http.Request, requestedPath string) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	requestedPath, directory, err := listDirectory(requestedPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := listResponse{Path: requestedPath, Entries: make([]listEntry, 0)}
	err = filepath.WalkDir(directory, func(fullPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fullPath == directory {
			return nil
		}

		relativePath, err := filepath.Rel(directory, fullPath)
		if err != nil {
			return err
		}
		depth := strings.Count(filepath.ToSlash(relativePath), "/") + 1
		if depth > maxListDepth {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Do not follow or expose symlinks, which could point outside the allowed root.
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if len(response.Entries) >= maxListEntries {
			response.Truncated = true
			return errListLimit
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		kind := "file"
		if entry.IsDir() {
			kind = "directory"
		}
		response.Entries = append(response.Entries, listEntry{
			Path:     filepath.ToSlash(relativePath),
			Type:     kind,
			Size:     info.Size(),
			Modified: info.ModTime().UTC(),
		})
		return nil
	})
	if err != nil && !errors.Is(err, errListLimit) {
		log.Printf("list %q: %v", requestedPath, err)
		http.Error(w, "unable to list directory", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("write list response: %v", err)
	}
}

func serveFile(w http.ResponseWriter, r *http.Request) {
	cleanedPath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if directoryPath, isListRequest := listPath(cleanedPath); isListRequest {
		if !listEnabled {
			http.NotFound(w, r)
			return
		}
		list(w, r, directoryPath)
		return
	}

	filePath := filepath.Join(rootDir, filepath.FromSlash(cleanedPath))
	relativePath, err := filepath.Rel(rootDir, filePath)
	if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		http.NotFound(w, r)
		return
	}

	info, err := os.Stat(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if info.IsDir() {
		http.Error(w, "directory listing is disabled", http.StatusForbidden)
		return
	}
	http.ServeFile(w, r, filePath)
}

func listPath(requestPath string) (string, bool) {
	if requestPath == strings.TrimPrefix(listSuffix, "/") {
		return "", true
	}
	if !strings.HasSuffix(requestPath, listSuffix) {
		return "", false
	}
	return strings.TrimSuffix(requestPath, listSuffix), true
}

func listDirectory(requested string) (string, string, error) {
	cleaned := path.Clean("/" + strings.Trim(strings.TrimSpace(requested), "/"))
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." {
		cleaned = ""
	}
	if cleaned != "" && (strings.HasPrefix(cleaned, "../") || cleaned == "..") {
		return "", "", errors.New("path must stay inside the public root")
	}
	directory := filepath.Join(rootDir, filepath.FromSlash(cleaned))
	relativePath, err := filepath.Rel(rootDir, directory)
	if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return "", "", errors.New("path must stay inside the public root")
	}
	info, err := os.Stat(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", errors.New("directory not found")
		}
		return "", "", errors.New("directory is unavailable")
	}
	if !info.IsDir() {
		return "", "", errors.New("path must be a directory")
	}
	if relativePath == "." {
		relativePath = ""
	}
	return filepath.ToSlash(relativePath), directory, nil
}
