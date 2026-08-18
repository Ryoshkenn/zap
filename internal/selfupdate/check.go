package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"
)

const (
	// latestReleaseURL is the GitHub API endpoint for the newest non-draft,
	// non-prerelease release.
	latestReleaseURL = "https://api.github.com/repos/Ryoshkenn/zap/releases/latest"

	// checkTimeout keeps a background check from hanging a TUI startup.
	checkTimeout = 5 * time.Second
)

// Release is the subset of the GitHub release payload zap needs.
type Release struct {
	TagName string  `json:"tag_name"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

// Asset is one uploaded release file.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// Result is the outcome of an update check.
type Result struct {
	Current   Version
	Latest    Version
	Release   *Release
	Available bool
}

// httpClient is package-level so tests can swap in a stub transport.
var httpClient = &http.Client{Timeout: checkTimeout}

// Check asks GitHub for the latest release and compares it to currentVersion.
//
// Any network or decoding failure is returned as an error; callers doing a
// background check should treat that as "no update known" and stay silent
// rather than surfacing noise for an offline user.
func Check(ctx context.Context, currentVersion string) (*Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "zap/"+currentVersion)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no published releases found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("check for updates: github returned %s", resp.Status)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}

	cur := ParseVersion(currentVersion)
	latest := ParseVersion(rel.TagName)
	return &Result{
		Current:   cur,
		Latest:    latest,
		Release:   &rel,
		Available: latest.IsNewerThan(cur),
	}, nil
}

// AssetName renders the goreleaser archive name for a version on this
// platform. It must stay in lockstep with the name_template in
// .goreleaser.yaml — a mismatch here means "asset not found" at download time.
//
//	zap_1.2.0_macos_arm64.tar.gz
//	zap_1.2.0_windows_x86_64.zip
func AssetName(version string) string {
	v := strings.TrimPrefix(version, "v")

	osName := runtime.GOOS
	if osName == "darwin" {
		osName = "macos"
	}

	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "arm64"
	}

	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("zap_%s_%s_%s.%s", v, osName, arch, ext)
}

// FindAsset locates the archive for this platform in a release.
func (r *Release) FindAsset(version string) (*Asset, error) {
	want := AssetName(version)
	for i := range r.Assets {
		if r.Assets[i].Name == want {
			return &r.Assets[i], nil
		}
	}
	return nil, fmt.Errorf("no release asset named %q for %s/%s", want, runtime.GOOS, runtime.GOARCH)
}

// FindChecksums locates the goreleaser checksums.txt asset.
func (r *Release) FindChecksums() (*Asset, error) {
	for i := range r.Assets {
		if r.Assets[i].Name == "checksums.txt" {
			return &r.Assets[i], nil
		}
	}
	return nil, fmt.Errorf("release has no checksums.txt; refusing to update unverified")
}
