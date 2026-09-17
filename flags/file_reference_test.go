package flags

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenFileReferencePreservesBinaryAndResolvesFromBase(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "plans")
	if err := os.Mkdir(base, 0o700); err != nil {
		t.Fatal(err)
	}
	content := []byte{0, 1, 255, 0}
	if err := os.WriteFile(filepath.Join(root, "data.xlsx"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	opened, err := OpenFileReference(context.Background(), "@../data.xlsx", FileReferenceOptions{BaseDir: base, Root: root, Remote: true})
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Body.Close()
	got, err := io.ReadAll(opened.Body)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Name != "data.xlsx" || string(got) != string(content) {
		t.Fatalf("got %q %v; want data.xlsx %v", opened.Name, got, content)
	}
}

func TestOpenFileReferenceAcceptsRelativeRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "data.csv"), []byte("Name\nAlice\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relativeRoot, err := filepath.Rel(workingDir, root)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := OpenFileReference(context.Background(), "@data.csv", FileReferenceOptions{BaseDir: root, Root: relativeRoot, Remote: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := opened.Body.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenFileReferenceRejectsRootEscapeThroughSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "data.csv"), []byte("Name\nAlice\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "data.csv"), filepath.Join(root, "data.csv")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := OpenFileReference(context.Background(), "@data.csv", FileReferenceOptions{BaseDir: root, Root: root, Remote: true})
	if err == nil || !strings.Contains(err.Error(), "escapes root") {
		t.Fatalf("expected root escape, got %v", err)
	}
}

func TestOpenFileReferenceRejectsRemoteLocalWithoutRoot(t *testing.T) {
	_, err := OpenFileReference(context.Background(), "@data.csv", FileReferenceOptions{Remote: true})
	if err == nil || !strings.Contains(err.Error(), "root") {
		t.Fatalf("expected root requirement, got %v", err)
	}
}

func TestOpenFileReferenceReadsURLWithoutLosingExtension(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Name\nAlice\n"))
	}))
	defer server.Close()
	opened, err := OpenFileReference(context.Background(), "@"+server.URL+"/data.csv?version=1", FileReferenceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Body.Close()
	got, err := io.ReadAll(opened.Body)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Name != "data.csv" || string(got) != "Name\nAlice\n" {
		t.Fatalf("got %q %q", opened.Name, got)
	}
}

func TestOpenFileReferenceRejectsLoopbackURLForRemoteCaller(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("should not be fetched"))
	}))
	defer server.Close()
	opened, err := OpenFileReference(context.Background(), "@"+server.URL+"/data.csv?token=private-value", FileReferenceOptions{Remote: true})
	if err == nil {
		_ = opened.Body.Close()
		t.Fatal("expected defensive HTTP client to reject loopback")
	}
	if strings.Contains(err.Error(), "private-value") {
		t.Fatalf("error exposed URL query: %v", err)
	}
}

func TestOpenFileReferenceRequiresAtPrefix(t *testing.T) {
	_, err := OpenFileReference(context.Background(), "data.csv", FileReferenceOptions{})
	if err == nil || !strings.Contains(err.Error(), "@") {
		t.Fatalf("expected @ requirement, got %v", err)
	}
}
