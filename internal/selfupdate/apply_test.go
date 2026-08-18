package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestChecksumFor(t *testing.T) {
	sums := "abc123  zap_1.2.0_macos_arm64.tar.gz\n" +
		"def456  zap_1.2.0_linux_x86_64.tar.gz\n"

	got, err := checksumFor(sums, "zap_1.2.0_linux_x86_64.tar.gz")
	if err != nil {
		t.Fatalf("checksumFor: %v", err)
	}
	if got != "def456" {
		t.Errorf("got %q, want %q", got, "def456")
	}

	if _, err := checksumFor(sums, "zap_9.9.9_plan9_arm64.tar.gz"); err == nil {
		t.Error("expected an error for an asset with no checksum entry")
	}
}

func TestExtractBinaryFromTarGz(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("tar.gz path is not used on windows")
	}
	want := []byte("#!/bin/sh\necho zap\n")

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	// A README alongside the binary must not be mistaken for it.
	for _, f := range []struct {
		name string
		body []byte
	}{
		{"README.md", []byte("not the binary")},
		{"zap", want},
	} {
		if err := tw.WriteHeader(&tar.Header{Name: f.name, Mode: 0o755, Size: int64(len(f.body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(f.body); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gz.Close()

	got, err := extractBinary(buf.Bytes(), "zap_1.2.0_linux_x86_64.tar.gz")
	if err != nil {
		t.Fatalf("extractBinary: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("extracted %q, want %q", got, want)
	}
}

func TestReplaceExecutableSwapsAndPreservesMode(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "zap")
	if err := os.WriteFile(exe, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := replaceExecutable(exe, []byte("new binary")); err != nil {
		t.Fatalf("replaceExecutable: %v", err)
	}

	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new binary" {
		t.Errorf("binary content = %q, want %q", got, "new binary")
	}

	fi, err := os.Stat(exe)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o755 {
		t.Errorf("mode = %v, want 0755", fi.Mode().Perm())
	}

	// No staging or backup litter should survive a successful swap.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected only the binary to remain, found %v", names)
	}
}
