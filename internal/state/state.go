package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	maxRecents      = 20
	stateFileName   = "state.json"
	stateDirSubpath = "zap"
)

// State is the persisted favorites + recents + per-provider preferences.
type State struct {
	FavoriteFolders   []string            `json:"favorite_folders"`
	FavoriteProviders []string            `json:"favorite_providers"`
	RecentFolders     []RecentFolder      `json:"recent_folders"`
	PreferredFlags    map[string][]string `json:"preferred_flags,omitempty"`
	LaunchModes       map[string]string   `json:"launch_modes,omitempty"`
	PreferredModels   map[string]string   `json:"preferred_models,omitempty"`
	// SSHHosts are remote computers the user has saved ("add a computer").
	SSHHosts []SSHHost `json:"ssh_hosts,omitempty"`
	// RecentRemotes tracks recently used folders per remote host (keyed by target).
	RecentRemotes []RecentRemote `json:"recent_remotes,omitempty"`
	// LastLaunch records the most recent launch so `zap last` can replay it.
	LastLaunch *LastLaunch `json:"last_launch,omitempty"`
	// InstallID is a stable anonymous identifier generated on first run.
	InstallID string `json:"install_id,omitempty"`
}

// SSHHost is a saved remote computer. Target is what gets passed to ssh —
// an alias from ~/.ssh/config (e.g. "devbox") or "user@host[:port handled via config]".
type SSHHost struct {
	Alias  string `json:"alias,omitempty"` // optional friendly name; falls back to Target
	Target string `json:"target"`          // ssh destination
	Shell  string `json:"shell,omitempty"` // remote login shell for PATH (default: bash)
}

// Label returns the display name for a host.
func (h SSHHost) Label() string {
	if h.Alias != "" {
		return h.Alias
	}
	return h.Target
}

// RecentRemote is a remote folder path used on a given ssh target.
type RecentRemote struct {
	Target string    `json:"target"`
	Path   string    `json:"path"`
	TS     time.Time `json:"ts"`
}

// LastLaunch captures enough to replay the previous launch via `zap last`.
type LastLaunch struct {
	ProviderID string   `json:"provider_id"`
	Folder     string   `json:"folder"` // local dir, or remote path when SSHTarget is set
	Flags      []string `json:"flags,omitempty"`
	Model      string   `json:"model,omitempty"`
	SSHTarget  string   `json:"ssh_target,omitempty"` // empty => local launch
	SSHShell   string   `json:"ssh_shell,omitempty"`
}

// SetLaunchMode persists the launch mode ("terminal" or "app") for a provider.
func (s *State) SetLaunchMode(providerID, mode string) {
	if s.LaunchModes == nil {
		s.LaunchModes = map[string]string{}
	}
	s.LaunchModes[providerID] = mode
}

// LaunchModeFor returns the saved launch mode and whether one was recorded.
func (s *State) LaunchModeFor(providerID string) (string, bool) {
	if s.LaunchModes == nil {
		return "", false
	}
	v, ok := s.LaunchModes[providerID]
	return v, ok
}

// SetPreferredFlags stores the user's chosen flag set for a provider.
// Pass an empty slice to clear (still records that the user explicitly chose "none").
func (s *State) SetPreferredFlags(providerID string, flags []string) {
	if s.PreferredFlags == nil {
		s.PreferredFlags = map[string][]string{}
	}
	clone := make([]string, len(flags))
	copy(clone, flags)
	s.PreferredFlags[providerID] = clone
}

// SetPreferredModel stores the user's chosen model for a provider.
func (s *State) SetPreferredModel(providerID, model string) {
	if s.PreferredModels == nil {
		s.PreferredModels = map[string]string{}
	}
	s.PreferredModels[providerID] = model
}

// PreferredModelFor returns the saved model and whether one was recorded.
func (s *State) PreferredModelFor(providerID string) (string, bool) {
	if s.PreferredModels == nil {
		return "", false
	}
	v, ok := s.PreferredModels[providerID]
	return v, ok
}

// PreferredFlagsFor returns the saved flag set and whether one was recorded.
func (s *State) PreferredFlagsFor(providerID string) ([]string, bool) {
	if s.PreferredFlags == nil {
		return nil, false
	}
	v, ok := s.PreferredFlags[providerID]
	return v, ok
}

// RecentFolder is a folder path with a last-used timestamp.
type RecentFolder struct {
	Path string    `json:"path"`
	TS   time.Time `json:"ts"`
}

// StateDir returns the directory where state.json lives.
// We use UserConfigDir (not UserCacheDir) because favorites, recents, and
// preferred flags are user data — cache dirs are documented as regenerable
// and may be wiped by the OS or cleanup tools.
func StateDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, stateDirSubpath), nil
}

// StatePath returns the resolved path to state.json.
func StatePath() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, stateFileName), nil
}

// Load reads state.json. Returns a zero-value State if missing.
func Load() (*State, error) {
	path, err := StatePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &State{}, nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	return &s, nil
}

// Save writes state.json atomically.
func (s *State) Save() error {
	dir, err := StateDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, stateFileName)
	tmp := path + ".tmp"
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// AddFavoriteFolder adds path (absolute) to favorites if not present.
func (s *State) AddFavoriteFolder(path string) bool {
	for _, f := range s.FavoriteFolders {
		if f == path {
			return false
		}
	}
	s.FavoriteFolders = append(s.FavoriteFolders, path)
	return true
}

// RemoveFavoriteFolder removes path from favorites. Returns true if removed.
func (s *State) RemoveFavoriteFolder(path string) bool {
	for i, f := range s.FavoriteFolders {
		if f == path {
			s.FavoriteFolders = append(s.FavoriteFolders[:i], s.FavoriteFolders[i+1:]...)
			return true
		}
	}
	return false
}

// AddFavoriteProvider adds providerID to favorites if not present.
func (s *State) AddFavoriteProvider(id string) bool {
	for _, p := range s.FavoriteProviders {
		if p == id {
			return false
		}
	}
	s.FavoriteProviders = append(s.FavoriteProviders, id)
	return true
}

// RemoveFavoriteProvider removes providerID. Returns true if removed.
func (s *State) RemoveFavoriteProvider(id string) bool {
	for i, p := range s.FavoriteProviders {
		if p == id {
			s.FavoriteProviders = append(s.FavoriteProviders[:i], s.FavoriteProviders[i+1:]...)
			return true
		}
	}
	return false
}

// IsFavoriteFolder reports whether path is favorited.
func (s *State) IsFavoriteFolder(path string) bool {
	for _, f := range s.FavoriteFolders {
		if f == path {
			return true
		}
	}
	return false
}

// IsFavoriteProvider reports whether providerID is favorited.
func (s *State) IsFavoriteProvider(id string) bool {
	for _, p := range s.FavoriteProviders {
		if p == id {
			return true
		}
	}
	return false
}

// TouchRecent moves path to the front of recents with current timestamp.
// Deduplicates by path and caps at maxRecents.
func (s *State) TouchRecent(path string) {
	now := time.Now().UTC()
	filtered := s.RecentFolders[:0]
	for _, r := range s.RecentFolders {
		if r.Path != path {
			filtered = append(filtered, r)
		}
	}
	s.RecentFolders = append([]RecentFolder{{Path: path, TS: now}}, filtered...)
	if len(s.RecentFolders) > maxRecents {
		s.RecentFolders = s.RecentFolders[:maxRecents]
	}
}

// RecentsSorted returns recents sorted by timestamp descending, optionally capped.
func (s *State) RecentsSorted(limit int) []RecentFolder {
	out := make([]RecentFolder, len(s.RecentFolders))
	copy(out, s.RecentFolders)
	sort.Slice(out, func(i, j int) bool { return out[i].TS.After(out[j].TS) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// AddSSHHost saves a host, deduplicating by Target. An existing entry's alias
// and shell are updated. Returns true if a new host was added.
func (s *State) AddSSHHost(h SSHHost) bool {
	for i := range s.SSHHosts {
		if s.SSHHosts[i].Target == h.Target {
			if h.Alias != "" {
				s.SSHHosts[i].Alias = h.Alias
			}
			if h.Shell != "" {
				s.SSHHosts[i].Shell = h.Shell
			}
			return false
		}
	}
	s.SSHHosts = append(s.SSHHosts, h)
	return true
}

// RemoveSSHHost removes a host by target. Returns true if removed.
func (s *State) RemoveSSHHost(target string) bool {
	for i, h := range s.SSHHosts {
		if h.Target == target {
			s.SSHHosts = append(s.SSHHosts[:i], s.SSHHosts[i+1:]...)
			return true
		}
	}
	return false
}

// FindSSHHost returns the saved host for a target, or nil.
func (s *State) FindSSHHost(target string) *SSHHost {
	for i := range s.SSHHosts {
		if s.SSHHosts[i].Target == target {
			return &s.SSHHosts[i]
		}
	}
	return nil
}

// TouchRemote moves (target, path) to the front of remote recents.
// Deduplicates by (target, path) and caps the per-target history at maxRecents.
func (s *State) TouchRemote(target, path string) {
	now := time.Now().UTC()
	filtered := s.RecentRemotes[:0]
	count := 0
	for _, r := range s.RecentRemotes {
		if r.Target == target && r.Path == path {
			continue
		}
		if r.Target == target {
			count++
			if count >= maxRecents {
				continue
			}
		}
		filtered = append(filtered, r)
	}
	s.RecentRemotes = append([]RecentRemote{{Target: target, Path: path, TS: now}}, filtered...)
}

// RemoteRecents returns recent folders for a target, newest first, optionally capped.
func (s *State) RemoteRecents(target string, limit int) []RecentRemote {
	var out []RecentRemote
	for _, r := range s.RecentRemotes {
		if r.Target == target {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TS.After(out[j].TS) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// SetLastLaunch records the most recent launch for `zap last`.
func (s *State) SetLastLaunch(l LastLaunch) {
	s.LastLaunch = &l
}
