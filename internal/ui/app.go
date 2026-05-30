package ui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ryoshkenn/zap/internal/config"
	"github.com/Ryoshkenn/zap/internal/detect"
	"github.com/Ryoshkenn/zap/internal/launch"
	"github.com/Ryoshkenn/zap/internal/state"
)

type screen int

const (
	screenFolder screen = iota
	screenBrowse
	screenProvider
	screenFlags
	screenSettings
	screenModelPicker
)

type app struct {
	screen   screen
	cfg      *config.Config
	statuses []detect.Status
	state    *state.State

	folder      *folderModel
	browse      *browseModel
	provider    *providerModel
	flagsModel  *flagsModel
	settings    *settingsModel
	modelPicker *modelPickerModel

	chosenFolder   string
	chosenProvider *detect.Status

	width, height int

	finalLaunch *launchResult
	err         error
}

type launchResult struct {
	Folder        string
	Command       string
	Args          []string
	LaunchMode    string // "terminal" or "app"
	AppBundlePath string // macOS: /Applications/<bundle>.app, triggers `open -a`
}

// Run launches the interactive TUI. On selection, the chosen provider is exec'd
// in the chosen folder, replacing the zap process (Unix) or running as child (Windows).
func Run() error {
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

	// Persist recent before exec replaces us.
	final.state.TouchRecent(final.finalLaunch.Folder)
	_ = final.state.Save()

	if final.finalLaunch.LaunchMode == "app" {
		return launch.Open(final.finalLaunch.Folder, final.finalLaunch.Command, final.finalLaunch.Args, final.finalLaunch.AppBundlePath)
	}
	return launch.Exec(final.finalLaunch.Folder, final.finalLaunch.Command, final.finalLaunch.Args, os.Environ())
}

func (a *app) Init() tea.Cmd {
	return nil
}

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if wm, ok := msg.(tea.WindowSizeMsg); ok {
		a.width, a.height = wm.Width, wm.Height
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		if km.String() == "ctrl+c" || (km.String() == "q" && a.screen != screenBrowse && a.screen != screenModelPicker) {
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
	case screenFlags:
		return a.flagsModel.Update(msg)
	case screenSettings:
		return a.settings.Update(msg)
	case screenModelPicker:
		return a.modelPicker.Update(msg)
	}
	return a, nil
}

func (a *app) View() string {
	switch a.screen {
	case screenFolder:
		return a.folder.View()
	case screenBrowse:
		return a.browse.View()
	case screenProvider:
		return a.provider.View()
	case screenFlags:
		return a.flagsModel.View()
	case screenSettings:
		return a.settings.View()
	case screenModelPicker:
		return a.modelPicker.View()
	}
	return ""
}

func (a *app) gotoSettings() tea.Cmd {
	a.settings = newSettingsModel(a)
	a.screen = screenSettings
	return nil
}

// transitions

func (a *app) gotoProvider(folder string) tea.Cmd {
	a.chosenFolder = folder
	a.provider = newProviderModel(a)
	a.screen = screenProvider
	return nil
}

func (a *app) gotoBrowse() tea.Cmd {
	a.browse = newBrowseModel(a)
	a.screen = screenBrowse
	return a.browse.init()
}

func (a *app) gotoFlags(st *detect.Status) tea.Cmd {
	a.chosenProvider = st
	if st.Provider.ModelSelector {
		return a.gotoModelPicker(st, screenProvider)
	}
	// Apply saved preferred flags if present, otherwise use defaults — skip the
	// flags picker screen entirely (flags are configurable in Settings).
	var extra []string
	if saved, ok := a.state.PreferredFlagsFor(st.Provider.ID); ok {
		extra = saved
	} else {
		for _, f := range st.Provider.Flags {
			flag := f.Flag
			on := f.Default
			for _, df := range st.Provider.DefaultFlags {
				if df == flag {
					on = true
				}
			}
			if on {
				extra = append(extra, flag)
			}
		}
	}
	copy := *st
	copy.Provider.DefaultFlags = nil
	return a.launch(&copy, extra)
}

func (a *app) gotoModelPicker(st *detect.Status, returnTo screen) tea.Cmd {
	a.chosenProvider = st
	onSelect := func(model string) tea.Cmd {
		a.state.SetPreferredModel(st.Provider.ID, model)
		_ = a.state.Save()
		if returnTo == screenSettings {
			a.settings.rebuild()
			a.screen = screenSettings
			return nil
		}
		// Launch flow: pass "run" + model as args.
		copy := *st
		copy.Provider.DefaultFlags = nil
		return a.launch(&copy, []string{"run", model})
	}
	a.modelPicker = newModelPickerModel(a, st.Provider.ID, onSelect, returnTo, returnTo == screenSettings)
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
	}
	return tea.Quit
}
