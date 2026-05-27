package docs_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var builtInProviderNames = []string{
	"Claude Code",
	"Codex CLI",
	"Gemini CLI",
	"opencode",
	"Cursor",
	"Windsurf",
	"VS Code",
}

func TestSiteLogoKeepsUsersOnProjectSite(t *testing.T) {
	index := readFile(t, "index.html")
	if strings.Contains(index, `class="nav-logo">`) && strings.Contains(index, `href="/" class="nav-logo"`) {
		t.Fatal("nav logo points to the GitHub Pages user root instead of the zap project site")
	}
}

func TestSiteDoesNotReferenceMissingLocalOpenGraphAsset(t *testing.T) {
	index := readFile(t, "index.html")
	re := regexp.MustCompile(`<meta property="og:image" content="https://ryoshkenn\.github\.io/zap/([^"]+)" />`)
	match := re.FindStringSubmatch(index)
	if match == nil {
		return
	}
	if _, err := os.Stat(filepath.Join(".", match[1])); err != nil {
		t.Fatalf("og:image points at missing docs asset %q: %v", match[1], err)
	}
}

func TestPublicProviderListsIncludeAllBuiltIns(t *testing.T) {
	index := readFile(t, "index.html")
	readme := readFile(t, filepath.Join("..", "README.md"))

	for _, name := range builtInProviderNames {
		if !strings.Contains(index, name) {
			t.Errorf("docs/index.html missing built-in provider %q", name)
		}
		if !strings.Contains(readme, name) {
			t.Errorf("README.md missing built-in provider %q", name)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
