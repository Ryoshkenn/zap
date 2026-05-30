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
