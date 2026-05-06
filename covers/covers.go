package covers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var client = &http.Client{Timeout: 15 * time.Second}

type Store struct {
	dir string
}

func New(dir string) *Store {
	return &Store{dir: dir}
}

// Download fetches the image at remoteURL and saves it locally.
// Returns the local URL path (e.g. "/metadata/covers/9780141036144.jpg").
// If remoteURL is empty or already local, it is returned as-is.
// Errors are logged but never returned — a failed download does not block book creation.
func (s *Store) Download(remoteURL, key string) string {
	if s == nil || s.dir == "" || remoteURL == "" {
		return remoteURL
	}
	if strings.HasPrefix(remoteURL, "/metadata/") {
		return remoteURL
	}

	resp, err := client.Get(remoteURL)
	if err != nil {
		log.Printf("covers: fetch %s: %v", remoteURL, err)
		return remoteURL
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		log.Printf("covers: unexpected content-type %q for %s", ct, remoteURL)
		return remoteURL
	}

	ext := extFromContentType(ct)
	filename := key + ext

	if err := os.MkdirAll(s.dir, 0755); err != nil {
		log.Printf("covers: mkdir %s: %v", s.dir, err)
		return remoteURL
	}

	tmp, err := os.CreateTemp(s.dir, ".tmp-*")
	if err != nil {
		log.Printf("covers: create temp: %v", err)
		return remoteURL
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName) // no-op if rename succeeded
	}()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		log.Printf("covers: write %s: %v", filename, err)
		return remoteURL
	}
	tmp.Close()

	dest := filepath.Join(s.dir, filename)
	if err := os.Rename(tmpName, dest); err != nil {
		log.Printf("covers: rename to %s: %v", dest, err)
		return remoteURL
	}

	return fmt.Sprintf("/metadata/covers/%s", filename)
}

func extFromContentType(ct string) string {
	switch {
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "gif"):
		return ".gif"
	case strings.Contains(ct, "webp"):
		return ".webp"
	default:
		return ".jpg"
	}
}
