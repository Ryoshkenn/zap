package ollama

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"sort"
	"time"
)

const baseURL = "http://localhost:11434"

var client = &http.Client{Timeout: 3 * time.Second}

// RunningModel is a model currently loaded in Ollama's memory.
type RunningModel struct {
	Name   string
	SizeGB float64
}

// ModelKind classifies how a known model is accessed.
type ModelKind int

const (
	KindPull  ModelKind = iota // must be pulled/downloaded locally first
	KindCloud                   // runs on Ollama's hosted infrastructure; requires `ollama signin` (an Ollama account)
)

// KnownModel is an entry in the curated model catalogue.
type KnownModel struct {
	Name string
	Kind ModelKind
}

type tagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

type psResponse struct {
	Models []struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	} `json:"models"`
}

// IsRunning returns true if the Ollama API is reachable.
func IsRunning() bool {
	resp, err := client.Get(baseURL + "/api/tags")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// EnsureRunning starts `ollama serve` if not already running, then waits up to
// 5 s for it to become ready. The spawned `ollama serve` process is
// intentionally left running detached — it is a long-lived daemon and zap
// hands off to it rather than managing its lifecycle.
func EnsureRunning() error {
	if IsRunning() {
		return nil
	}
	cmd := exec.Command("ollama", "serve")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start ollama: %w", err)
	}
	for range 10 {
		time.Sleep(500 * time.Millisecond)
		if IsRunning() {
			return nil
		}
	}
	return fmt.Errorf("Ollama did not start — try running `ollama serve` manually")
}

// FetchModels returns the names of all locally downloaded models, sorted.
func FetchModels() ([]string, error) {
	resp, err := client.Get(baseURL + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("Ollama is not running — start it with `ollama serve`")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama API returned status %d", resp.StatusCode)
	}
	var tags tagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("failed to parse Ollama response: %w", err)
	}
	models := make([]string, 0, len(tags.Models))
	for _, m := range tags.Models {
		models = append(models, m.Name)
	}
	sort.Strings(models)
	return models, nil
}

// FetchRunningModels returns models currently loaded in Ollama's memory.
func FetchRunningModels() ([]RunningModel, error) {
	resp, err := client.Get(baseURL + "/api/ps")
	if err != nil {
		return nil, fmt.Errorf("could not reach Ollama API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("/api/ps returned status %d", resp.StatusCode)
	}
	var ps psResponse
	if err := json.NewDecoder(resp.Body).Decode(&ps); err != nil {
		return nil, fmt.Errorf("failed to parse /api/ps: %w", err)
	}
	out := make([]RunningModel, 0, len(ps.Models))
	for _, m := range ps.Models {
		out = append(out, RunningModel{
			Name:   m.Name,
			SizeGB: float64(m.Size) / (1024 * 1024 * 1024),
		})
	}
	return out, nil
}

// StopModel unloads the named model from Ollama's memory.
func StopModel(name string) error {
	out, err := exec.Command("ollama", "stop", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ollama stop %s: %s", name, string(out))
	}
	return nil
}

// knownModels is the curated catalogue shown in the model picker.
//
// Cloud models run on Ollama's hosted infrastructure — they require an Ollama
// account and `ollama signin` before use; no local download is needed.
// Their names carry a "-cloud" or ":cloud" suffix on the Ollama library.
//
// Pull models must be downloaded locally with `ollama pull` before first run.
//
// The library catalogue is not available via a stable programmatic API:
// /api/tags only lists already-downloaded models, and ollama.com/search has no
// documented JSON endpoint. This list therefore stays curated by hand.
// Source: https://ollama.com/search?c=cloud (confirmed May 2026).
var knownModels = []KnownModel{
	// Cloud — hosted on Ollama's infrastructure; requires `ollama signin`
	{Name: "gpt-oss:20b-cloud", Kind: KindCloud},
	{Name: "gpt-oss:120b-cloud", Kind: KindCloud},
	{Name: "qwen3-coder:480b-cloud", Kind: KindCloud},
	{Name: "deepseek-v3.1:671b-cloud", Kind: KindCloud},
	{Name: "glm-4.6:cloud", Kind: KindCloud},
	{Name: "gemma4:31b-cloud", Kind: KindCloud},
	{Name: "qwen3.5:cloud", Kind: KindCloud},
	{Name: "qwen3.5:397b-cloud", Kind: KindCloud},

	// Pull — local download required (~GBs); shown with download-confirm dialog
	{Name: "llama3.1:8b", Kind: KindPull},
	{Name: "llama3.2:3b", Kind: KindPull},
	{Name: "mistral:7b", Kind: KindPull},
	{Name: "gemma2:9b", Kind: KindPull},
	{Name: "deepseek-r1:7b", Kind: KindPull},
	{Name: "qwen2.5:7b", Kind: KindPull},
	{Name: "llama3.2", Kind: KindPull},
	{Name: "llama3.2:1b", Kind: KindPull},
	{Name: "llama3.1", Kind: KindPull},
	{Name: "llama3.1:70b", Kind: KindPull},
	{Name: "mistral", Kind: KindPull},
	{Name: "gemma2", Kind: KindPull},
	{Name: "gemma2:2b", Kind: KindPull},
	{Name: "phi3", Kind: KindPull},
	{Name: "phi3.5", Kind: KindPull},
	{Name: "phi4", Kind: KindPull},
	{Name: "qwen2.5", Kind: KindPull},
	{Name: "qwen2.5:14b", Kind: KindPull},
	{Name: "qwen2.5:32b", Kind: KindPull},
	{Name: "qwen2.5-coder:7b", Kind: KindPull},
	{Name: "qwen2.5-coder:14b", Kind: KindPull},
	{Name: "codellama", Kind: KindPull},
	{Name: "codellama:7b", Kind: KindPull},
	{Name: "codellama:13b", Kind: KindPull},
	{Name: "deepseek-coder-v2", Kind: KindPull},
	{Name: "deepseek-r1", Kind: KindPull},
	{Name: "llava", Kind: KindPull},
	{Name: "nomic-embed-text", Kind: KindPull},
}

// KnownModels returns the full curated model catalogue.
func KnownModels() []KnownModel { return knownModels }
