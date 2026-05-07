package covers

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func fakeImageClient(contentType string, body []byte) *http.Client {
	return &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{contentType}},
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}),
	}
}

func setClient(t *testing.T, c *http.Client) {
	t.Helper()
	orig := client
	client = c
	t.Cleanup(func() { client = orig })
}

// --- isAllowedURL ---

func TestIsAllowedURL(t *testing.T) {
	cases := []struct {
		url     string
		allowed bool
	}{
		{"https://books.google.com/covers/img.jpg", true},
		{"https://covers.openlibrary.org/b/isbn/123-L.jpg", true},
		{"http://books.google.com/img.jpg", false},
		{"ftp://example.com/img.jpg", false},
		{"https://127.0.0.1/img.jpg", false},
		{"https://[::1]/img.jpg", false},
		{"https://192.168.1.100/img.jpg", false},
		{"https://10.0.0.1/img.jpg", false},
		{"https://172.16.0.1/img.jpg", false},
		{"https://169.254.0.1/img.jpg", false},
		{"https://0.0.0.0/img.jpg", false},
		{"not a url", false},
		{"", false},
	}
	for _, c := range cases {
		got := isAllowedURL(c.url)
		if got != c.allowed {
			t.Errorf("isAllowedURL(%q) = %v, want %v", c.url, got, c.allowed)
		}
	}
}

// --- extFromContentType ---

func TestExtFromContentType(t *testing.T) {
	cases := []struct {
		ct   string
		want string
	}{
		{"image/jpeg", ".jpg"},
		{"image/jpg", ".jpg"},
		{"image/png", ".png"},
		{"image/gif", ".gif"},
		{"image/webp", ".webp"},
		{"image/png; charset=utf-8", ".png"},
		{"image/unknown", ".jpg"},
	}
	for _, c := range cases {
		got := extFromContentType(c.ct)
		if got != c.want {
			t.Errorf("extFromContentType(%q) = %q, want %q", c.ct, got, c.want)
		}
	}
}

// --- Download ---

func TestDownload_nil_store(t *testing.T) {
	var s *Store
	got := s.Download("https://example.com/img.jpg", "key")
	if got != "https://example.com/img.jpg" {
		t.Errorf("nil store: got %q, want original URL", got)
	}
}

func TestDownload_empty_url(t *testing.T) {
	s := New(t.TempDir())
	got := s.Download("", "key")
	if got != "" {
		t.Errorf("empty URL: got %q, want empty", got)
	}
}

func TestDownload_empty_dir(t *testing.T) {
	s := &Store{dir: ""}
	url := "https://example.com/img.jpg"
	got := s.Download(url, "key")
	if got != url {
		t.Errorf("empty dir: got %q, want original URL", got)
	}
}

func TestDownload_already_local(t *testing.T) {
	s := New(t.TempDir())
	local := "/metadata/covers/existing.jpg"
	got := s.Download(local, "key")
	if got != local {
		t.Errorf("local URL: got %q, want %q", got, local)
	}
}

func TestDownload_blocked_http_url(t *testing.T) {
	s := New(t.TempDir())
	url := "http://example.com/img.jpg"
	got := s.Download(url, "key")
	if got != url {
		t.Errorf("blocked http: got %q, want original URL", got)
	}
}

func TestDownload_success_jpeg(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	setClient(t, fakeImageClient("image/jpeg", []byte("fake-jpeg-data")))

	got := s.Download("https://books.google.com/covers/fake.jpg", "testkey")
	if !strings.HasPrefix(got, "/metadata/covers/") {
		t.Errorf("Download success: got %q, want /metadata/covers/...", got)
	}
	if !strings.HasSuffix(got, ".jpg") {
		t.Errorf("expected .jpg extension, got %q", got)
	}

	filename := filepath.Base(got)
	if _, err := os.Stat(filepath.Join(dir, filename)); err != nil {
		t.Errorf("file not saved to disk: %v", err)
	}
}

func TestDownload_success_png(t *testing.T) {
	s := New(t.TempDir())
	setClient(t, fakeImageClient("image/png", []byte("fake-png")))

	got := s.Download("https://books.google.com/covers/fake.png", "pngkey")
	if !strings.HasSuffix(got, ".png") {
		t.Errorf("expected .png extension, got %q", got)
	}
}

func TestDownload_non_image_content_type(t *testing.T) {
	s := New(t.TempDir())
	setClient(t, &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"text/html"}},
				Body:       io.NopCloser(strings.NewReader("<html>")),
			}, nil
		}),
	})

	url := "https://books.google.com/not-an-image"
	got := s.Download(url, "key")
	if got != url {
		t.Errorf("non-image: got %q, want original URL", got)
	}
}

func TestDownload_key_in_filename(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	setClient(t, fakeImageClient("image/jpeg", []byte("data")))

	got := s.Download("https://books.google.com/img.jpg", "myisbn123")
	if !strings.Contains(got, "myisbn123") {
		t.Errorf("filename should contain key, got %q", got)
	}
}
