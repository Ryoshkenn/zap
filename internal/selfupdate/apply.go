package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// maxDownload caps how much we will read from a release asset. zap archives are
// single static binaries in the ~4-10 MB range; anything far past that is a
// signal something is wrong, and an unbounded read is a memory DoS.
const maxDownload = 128 << 20 // 128 MiB

// Apply downloads the release archive for this platform, verifies its SHA-256
// against the release's checksums.txt, and atomically swaps it over the running
// binary. It returns the path of the binary it replaced.
//
// The checksum step is not optional: the archive is executable code that will
// replace the running program, so an unverified download is a code-execution
// hole. If checksums.txt is missing, Apply fails rather than trusting the blob.
func Apply(ctx context.Context, rel *Release, exePath string) (string, error) {
	asset, err := rel.FindAsset(rel.TagName)
	if err != nil {
		return "", err
	}
	sumsAsset, err := rel.FindChecksums()
	if err != nil {
		return "", err
	}

	sums, err := download(ctx, sumsAsset.URL)
	if err != nil {
		return "", fmt.Errorf("download checksums.txt: %w", err)
	}
	wantSum, err := checksumFor(string(sums), asset.Name)
	if err != nil {
		return "", err
	}

	archive, err := download(ctx, asset.URL)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", asset.Name, err)
	}

	gotSum := sha256.Sum256(archive)
	if hex.EncodeToString(gotSum[:]) != wantSum {
		return "", fmt.Errorf("checksum mismatch for %s: refusing to install", asset.Name)
	}

	binary, err := extractBinary(archive, asset.Name)
	if err != nil {
		return "", err
	}

	if err := replaceExecutable(exePath, binary); err != nil {
		return "", err
	}
	return exePath, nil
}

// download fetches a URL fully into memory, bounded by maxDownload.
func download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/octet-stream")

	// Downloads are much larger than an API call, so give them their own,
	// longer-lived client rather than the 5s check timeout.
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github returned %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxDownload))
}

// checksumFor pulls the hash for name out of a goreleaser checksums.txt, whose
// lines look like "<sha256>  zap_1.2.0_macos_arm64.tar.gz".
func checksumFor(sums, name string) (string, error) {
	sc := bufio.NewScanner(strings.NewReader(sums))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && fields[1] == name {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("checksums.txt has no entry for %s", name)
}

// extractBinary pulls the zap executable out of a .tar.gz or .zip archive.
func extractBinary(archive []byte, name string) ([]byte, error) {
	want := "zap"
	if runtime.GOOS == "windows" {
		want = "zap.exe"
	}
	if strings.HasSuffix(name, ".zip") {
		return extractFromZip(archive, want)
	}
	return extractFromTarGz(archive, want)
}

func extractFromTarGz(archive []byte, want string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("open gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != want {
			continue
		}
		return io.ReadAll(io.LimitReader(tr, maxDownload))
	}
	return nil, fmt.Errorf("archive contains no %s binary", want)
}

func extractFromZip(archive []byte, want string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	for _, f := range zr.File {
		if filepath.Base(f.Name) != want {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(io.LimitReader(rc, maxDownload))
	}
	return nil, fmt.Errorf("archive contains no %s binary", want)
}

// replaceExecutable swaps newBinary over exePath as atomically as the platform
// allows, keeping the previous binary aside so a failed swap can roll back.
//
// The staging file is written into the same directory as the target because
// os.Rename is only atomic within a filesystem, and /tmp is frequently a
// different mount than /usr/local/bin.
func replaceExecutable(exePath string, newBinary []byte) error {
	dir := filepath.Dir(exePath)

	stage, err := os.CreateTemp(dir, ".zap-update-*")
	if err != nil {
		return fmt.Errorf("stage update in %s: %w (is the directory writable?)", dir, err)
	}
	stagePath := stage.Name()
	defer os.Remove(stagePath) // no-op once renamed away

	if _, err := stage.Write(newBinary); err != nil {
		stage.Close()
		return fmt.Errorf("write staged binary: %w", err)
	}
	if err := stage.Close(); err != nil {
		return err
	}

	// Match the mode of the binary we are replacing, defaulting to 0755.
	mode := os.FileMode(0o755)
	if fi, err := os.Stat(exePath); err == nil {
		mode = fi.Mode().Perm()
	}
	if err := os.Chmod(stagePath, mode); err != nil {
		return err
	}

	// Move the current binary aside rather than deleting it: on Windows a
	// running .exe cannot be overwritten but can be renamed, and on every
	// platform it gives us something to roll back to.
	backup := exePath + ".old"
	_ = os.Remove(backup)
	if err := os.Rename(exePath, backup); err != nil {
		return fmt.Errorf("move current binary aside: %w", err)
	}

	if err := os.Rename(stagePath, exePath); err != nil {
		// Put the old binary back so the user is not left with no zap at all.
		if rbErr := os.Rename(backup, exePath); rbErr != nil {
			return fmt.Errorf("install failed (%v) AND rollback failed (%v); your previous binary is at %s", err, rbErr, backup)
		}
		return fmt.Errorf("install new binary: %w", err)
	}

	// Best-effort cleanup. On Windows the old image may still be mapped by the
	// running process, in which case it is removed on the next update instead.
	_ = os.Remove(backup)
	return nil
}
