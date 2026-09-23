package ui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ryoshkenn/zap/internal/config"
	"github.com/Ryoshkenn/zap/internal/detect"
	"github.com/Ryoshkenn/zap/internal/launch"
	"github.com/Ryoshkenn/zap/internal/state"
	"github.com/Ryoshkenn/zap/internal/telemetry"
)

type screen int

const (
	screenFolder screen = iota
	screenBrowse
	screenProvider
	screenSettings
	screenModelPicker
	screenHost
	screenAddHost
	screenRemoteFolder
)

type app struct {
	screen   screen
	cfg      *config.Config
	statuses []detect.Status
	state    *state.State

	folder       *folderModel
	browse       *browseModel
	provider     *providerModel
	settings     *settingsModel
	modelPicker  *modelPickerModel
	host         *hostModel
	addHost      *addHostModel
	remoteFolder *remoteFolderModel

	chosenFolder   string
	chosenProvider *detect.Status

	// remote is true once the user has chosen the SSH path; sshTarget/sshShell
	// hold the selected host. Cleared by gotoProvider so a later local launch
	// can never accidentally run over ssh.
	remote    bool
	sshTarget string
	sshShell  string

	width, height int

	// Background release check results, surfaced as a banner under any screen.
	updateAvailable bool
	updateLatest    string
	updateURL       string
	updateErr       error

	finalLaunch *launchResult
	err         error
}

type launchResult struct {
	Folder        string
	Command       string
	Args          []string
	LaunchMode    string // "terminal" or "app"
	AppBundlePath string // macOS: /Applications/<bundle>.app, triggers `open -a`
	ProviderID    string // for recording the last launch
	Model         string // for recording the last launch (model-selector providers)
	SSHTarget     string // non-empty => launch over ssh on this host
	SSHShell      string // remote login shell when SSHTarget is set
}

// Run launches the interactive TUI. On selection, the chosen provider is exec'd
// in the chosen folder, replacing the zap process (Unix) or running as child (Windows).
// version is the running zap build, used by the background update check.
func Run(version string) error {
	Version = version

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	statuses := detect.Detect(cfg)
	st, _ := state.Load()
	if st == nil {
		st = &state.State{}
	}

	a := &app{
		cfg:      cfg,
		statuses: statuses,
		state:    st,
		screen:   screenFolder,
	}
	a.folder = newFolderModel(a)

	p := tea.NewProgram(a, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	final := finalModel.(*app)
	if final.err != nil {
		return final.err
	}
	if final.finalLaunch == nil {
		return nil // user quit without selection
	}

	fl := final.finalLaunch

	// Remote launch: record per-host recents + last launch, then ssh away.
	if fl.SSHTarget != "" {
		final.state.TouchRemote(fl.SSHTarget, fl.Folder)
		final.state.SetLastLaunch(state.LastLaunch{
			ProviderID: fl.ProviderID,
			Folder:     fl.Folder,
			Flags:      launchArgs(fl),
			Model:      fl.Model,
			SSHTarget:  fl.SSHTarget,
			SSHShell:   fl.SSHShell,
		})
		_ = final.state.Save()
		telemetry.Track("zap_launch", map[string]any{
			"provider":    fl.ProviderID,
			"is_remote":   true,
			"is_yolo":     false,
			"has_model":   fl.Model != "",
			"launch_mode": "terminal",
			"trigger":     "interactive",
		})
		telemetry.Shutdown()
		return launch.ExecSSH(fl.SSHTarget, fl.Folder, fl.Command, fl.Args, fl.SSHShell)
	}

	// Local launch: persist recent + last launch before exec replaces us.
	final.state.TouchRecent(fl.Folder)
	final.state.SetLastLaunch(state.LastLaunch{
		ProviderID: fl.ProviderID,
		Folder:     fl.Folder,
		Flags:      launchArgs(fl),
		Model:      fl.Model,
	})
	_ = final.state.Save()

	telemetry.Track("zap_launch", map[string]any{
		"provider":    fl.ProviderID,
		"is_remote":   false,
		"is_yolo":     false,
		"has_model":   fl.Model != "",
		"launch_mode": fl.LaunchMode,
		"trigger":     "interactive",
	})
	telemetry.Shutdown()

	if fl.LaunchMode == "app" {
		return launch.Open(fl.Folder, fl.Command, fl.Args, fl.AppBundlePath)
	}
	return launch.Exec(fl.Folder, fl.Command, fl.Args, os.Environ())
}

// launchArgs returns the args to persist for `zap last`. It clones the slice so
// the stored state is independent of the live launch result.
func launchArgs(fl *launchResult) []string {
	if len(fl.Args) == 0 {
		return nil
	}
	out := make([]string, len(fl.Args))
	copy(out, fl.Args)
	return out
}

func (a *app) Init() tea.Cmd {
	return backgroundUpdateCheck(a)
}

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if wm, ok := msg.(tea.WindowSizeMsg); ok {
		a.width, a.height = wm.Width, wm.Height
		a.resize()
		return a, nil
	}
	if um, ok := msg.(updateCheckedMsg); ok {
		a.applyUpdateCheck(um)
		a.resize() // the banner may have just appeared
		// Fall through: Settings shows the outcome of a manual check.
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		if km.String() == "ctrl+c" || (km.String() == "q" && !a.typing()) {
			return a, tea.Quit
		}
	}

	switch a.screen {
	case screenFolder:
		return a.folder.Update(msg)
	case screenBrowse:
		return a.browse.Update(msg)
	case screenProvider:
		return a.provider.Update(msg)
	case screenSettings:
		return a.settings.Update(msg)
	case screenModelPicker:
		return a.modelPicker.Update(msg)
	case screenHost:
		return a.host.Update(msg)
	case screenAddHost:
		return a.addHost.Update(msg)
	case screenRemoteFolder:
		return a.remoteFolder.Update(msg)
	}
	return a, nil
}

// typing reports whether the current screen is capturing text, so 'q' must
// reach it as a character instead of quitting.
func (a *app) typing() bool {
	switch a.screen {
	case screenBrowse, screenModelPicker, screenAddHost, screenRemoteFolder:
		return true
	case screenFolder:
		return a.folder.list.FilterState() == list.Filtering
	case screenProvider:
		return a.provider.list.FilterState() == list.Filtering
	}
	return false
}

func (a *app) View() string {
	return clipLines(a.screenView()+a.updateBanner(), a.width)
}

func (a *app) screenView() string {
	switch a.screen {
	case screenFolder:
		return a.folder.View()
	case screenBrowse:
		return a.browse.View()
	case screenProvider:
		return a.provider.View()
	case screenSettings:
		return a.settings.View()
	case screenModelPicker:
		return a.modelPicker.View()
	case screenHost:
		return a.host.View()
	case screenAddHost:
		return a.addHost.View()
	case screenRemoteFolder:
		return a.remoteFolder.View()
	}
	return ""
}

func (a *app) gotoSettings() tea.Cmd {
	a.settings = newSettingsModel(a)
	a.screen = screenSettings
	return nil
}

// transitions

// gotoProvider is the LOCAL path. It clears remote state so a launch that
// reaches here can never accidentally run over ssh, even after the user
// backed out of the SSH flow with esc.
func (a *app) gotoProvider(folder string) tea.Cmd {
	a.remote = false
	a.sshTarget = ""
	a.sshShell = ""
	a.chosenFolder = folder
	a.provider = newProviderModel(a)
	a.screen = screenProvider
	return nil
}

// gotoRemoteProvider is the SSH path: it keeps remote=true and the chosen host.
func (a *app) gotoRemoteProvider(remotePath string) tea.Cmd {
	a.chosenFolder = remotePath
	a.provider = newProviderModel(a)
	a.screen = screenProvider
	return nil
}

// gotoHost opens the remote-host picker (saved hosts + ~/.ssh/config + add).
func (a *app) gotoHost() tea.Cmd {
	a.host = newHostModel(a)
	a.screen = screenHost
	return nil
}

// gotoAddHost opens the "add a computer" text input.
func (a *app) gotoAddHost() tea.Cmd {
	a.addHost = newAddHostModel(a)
	a.screen = screenAddHost
	return a.addHost.Init()
}

// enterRemote records the chosen host and opens the remote-folder picker.
func (a *app) enterRemote(target, shell string) tea.Cmd {
	a.remote = true
	a.sshTarget = target
	a.sshShell = shell
	a.state.AddSSHHost(state.SSHHost{Target: target, Shell: shell})
	_ = a.state.Save()
	a.remoteFolder = newRemoteFolderModel(a)
	a.screen = screenRemoteFolder
	return a.remoteFolder.Init()
}

func (a *app) gotoBrowse() tea.Cmd {
	a.browse = newBrowseModel(a)
	a.screen = screenBrowse
	return a.browse.init()
}

// launchSelected launches the chosen provider with its saved flags (or its
// defaults). Flags are configured in Settings, so there is no per-launch step.
func (a *app) launchSelected(st *detect.Status) tea.Cmd {
	a.chosenProvider = st
	if st.Provider.ModelSelector {
		// Model selection lives in Settings now. Launch straight away with the
		// chosen default; if none is set yet, send the user to Settings to pick
		// (or download) one.
		model, ok := a.state.PreferredModelFor(st.Provider.ID)
		if !ok {
			return a.gotoSettings()
		}
		copy := *st
		copy.Provider.DefaultFlags = nil
		return a.launch(&copy, []string{"run", model})
	}
	// Apply saved preferred flags if present, otherwise use defaults.
	var extra []string
	if saved, ok := a.state.PreferredFlagsFor(st.Provider.ID); ok {
		extra = st.Provider.NormalizeFlags(saved)
	} else {
		extra = st.Provider.DefaultFlagSet()
	}
	copy := *st
	copy.Provider.DefaultFlags = nil
	return a.launch(&copy, extra)
}

// gotoModelPicker opens the Ollama model manager: choose a default from
// downloaded/cloud models, or download a new one. returnTo is the screen the
// picker goes back to once a model is chosen or the picker is dismissed.
func (a *app) gotoModelPicker(st *detect.Status, returnTo screen) tea.Cmd {
	a.chosenProvider = st
	settingsMode := returnTo == screenSettings
	onSelect := func(model string) tea.Cmd {
		a.state.SetPreferredModel(st.Provider.ID, model)
		_ = a.state.Save()
		if settingsMode {
			a.settings.rebuild()
		}
		a.screen = returnTo
		return nil
	}
	a.modelPicker = newModelPickerModel(a, st.Provider.ID, onSelect, returnTo, settingsMode)
	a.screen = screenModelPicker
	return a.modelPicker.Init()
}

func (a *app) launch(st *detect.Status, extraFlags []string) tea.Cmd {
	args := append([]string(nil), st.Provider.DefaultFlags...)
	for _, f := range extraFlags {
		dup := false
		for _, existing := range args {
			if existing == f {
				dup = true
				break
			}
		}
		if !dup {
			args = append(args, f)
		}
	}

	mode := st.Provider.LaunchMode
	if mode == "" {
		mode = "terminal"
	}
	if saved, ok := a.state.LaunchModeFor(st.Provider.ID); ok {
		mode = saved
	}

	a.finalLaunch = &launchResult{
		Folder:        a.chosenFolder,
		Command:       st.Provider.Command,
		Args:          args,
		LaunchMode:    mode,
		AppBundlePath: st.AppBundlePath,
		ProviderID:    st.Provider.ID,
	}

	// Remote launches always run in a terminal over ssh; GUI app-mode is
	// meaningless across an ssh connection.
	if a.remote {
		a.finalLaunch.LaunchMode = "terminal"
		a.finalLaunch.AppBundlePath = ""
		a.finalLaunch.SSHTarget = a.sshTarget
		a.finalLaunch.SSHShell = a.sshShell
	}
	return tea.Quit
}
