package sshconf

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestHostsFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := `# comment
Host devbox
    HostName 10.0.0.5
    User me

Host prod staging
    HostName example.com

Host *.internal
    User admin

host lower-case-keyword
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got := hostsFromFile(path)
	want := []string{"devbox", "lower-case-keyword", "prod", "staging"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestHostsMissingFile(t *testing.T) {
	if got := hostsFromFile(filepath.Join(t.TempDir(), "nope")); got != nil {
		t.Errorf("missing file should return nil, got %v", got)
	}
}
