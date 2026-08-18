package selfupdate

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// AssetName must render exactly what goreleaser uploads. If someone edits the
// archives name_template in .goreleaser.yaml without updating AssetName, every
// self-update silently fails with "no release asset named ...". This test reads
// the real config so the two cannot drift apart unnoticed.
func TestAssetNameMatchesGoreleaserTemplate(t *testing.T) {
	data, err := os.ReadFile("../../.goreleaser.yaml")
	if err != nil {
		t.Fatalf("read goreleaser config: %v", err)
	}
	cfg := string(data)

	for _, want := range []string{
		"{{ .ProjectName }}_{{ .Version }}_",
		`{{- if eq .Os "darwin" }}macos`,
		`{{- if eq .Arch "amd64" }}x86_64`,
		`{{- else if eq .Arch "arm64" }}arm64`,
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("goreleaser name_template changed: missing %q\n"+
				"update AssetName in check.go to match", want)
		}
	}

	if !strings.Contains(cfg, "formats: [tar.gz]") {
		t.Error("goreleaser default archive format is no longer tar.gz; update AssetName")
	}
	if !strings.Contains(cfg, "formats: [zip]") {
		t.Error("goreleaser windows override is no longer zip; update AssetName")
	}
}

func TestAssetNameShape(t *testing.T) {
	got := AssetName("v1.2.0")
	// zap_<ver>_<os>_<arch>.<ext> with the leading v stripped.
	re := regexp.MustCompile(`^zap_1\.2\.0_(macos|linux|windows)_(x86_64|arm64)\.(tar\.gz|zip)$`)
	if !re.MatchString(got) {
		t.Errorf("AssetName(%q) = %q, which does not match the goreleaser shape", "v1.2.0", got)
	}
	if AssetName("1.2.0") != got {
		t.Errorf("AssetName should strip a leading v: %q != %q", AssetName("1.2.0"), got)
	}
}

func TestFindAssetAndChecksums(t *testing.T) {
	name := AssetName("v1.2.0")
	rel := &Release{
		TagName: "v1.2.0",
		Assets: []Asset{
			{Name: "checksums.txt", URL: "https://example.invalid/checksums.txt"},
			{Name: name, URL: "https://example.invalid/" + name},
		},
	}

	a, err := rel.FindAsset("v1.2.0")
	if err != nil {
		t.Fatalf("FindAsset: %v", err)
	}
	if a.Name != name {
		t.Errorf("FindAsset returned %q, want %q", a.Name, name)
	}

	if _, err := rel.FindChecksums(); err != nil {
		t.Errorf("FindChecksums: %v", err)
	}

	// A release with no checksums must be refused, not silently trusted.
	bare := &Release{TagName: "v1.2.0", Assets: []Asset{{Name: name}}}
	if _, err := bare.FindChecksums(); err == nil {
		t.Error("expected an error when checksums.txt is absent")
	}
}
