# zap

A terminal launcher for AI coding CLIs. Pick a folder, pick a provider (Claude Code, Codex, Gemini, opencode…), and zap.

```
$ zap
┌─ Pick a folder ──────────────────────────┐
│ ⭐ ~/projects/zap                         │
│ 🕘 ~/work/api-server                      │
│ 🕘 ~/projects/notes                       │
│ 📁 Current: ~/Documents/code              │
│ ➜  Browse folders…                        │
└──────────────────────────────────────────┘
```

## Why

If you bounce between Claude Code, Codex, Gemini, and opencode across many repos, you spend a surprising amount of time typing `cd ~/projects/foo && claude --dangerously-skip-permissions`. zap collapses that into one keystroke.

## Install

### macOS / Linux (Homebrew)
```sh
brew install Ryoshkenn/zap/zap
```

### Windows (Scoop)
```powershell
scoop bucket add zap https://github.com/Ryoshkenn/scoop-zap
scoop install zap
```

### From source (any platform)
```sh
go install github.com/Ryoshkenn/zap/cmd/zap@latest
```

### Pre-built binaries
Grab a release archive from [GitHub Releases](https://github.com/Ryoshkenn/zap/releases) and drop the binary in your `PATH`.

## Usage

### Interactive
```sh
zap
```
Pick a folder (favorites → recents → current → browse), pick a provider, optionally toggle flags, launch.

### Fast non-interactive
```sh
zap claude                      # launch Claude in $PWD
zap claude /path/to/repo        # launch Claude in /path/to/repo
zap claude --yolo               # add --dangerously-skip-permissions
zap claude --safe               # remove any default dangerous flags
zap codex
zap gemini ~/projects/foo
zap opencode
```

### Favorites
```sh
zap favorite                    # star the current folder
zap favorite ~/work/api         # star a folder by path
zap favorite claude             # star a provider (appears first in picker)
zap unfavorite claude           # remove
zap list favorites              # show all stars
```

### Other
```sh
zap list                        # show all providers + install status
zap config                      # print the resolved config path
zap config edit                 # open ~/.config/zap/config.yaml in $EDITOR
zap --version
```

## Configuration

Optional. `zap` works out of the box. Override defaults by creating `~/.config/zap/config.yaml` (or `%APPDATA%\zap\config.yaml` on Windows):

```yaml
# Per-provider defaults
providers:
  claude:
    # `zap claude` runs with this flag set; `zap claude --safe` removes it.
    default_flags: ["--dangerously-skip-permissions"]

# Add providers not in the built-in registry
custom_providers:
  - id: my-cli
    name: My Internal CLI
    command: acme-cli
    icon: "🛠️"
    install_hint: "Contact infra"
```

Favorites, recents, and per-provider flag preferences are stored at `~/Library/Application Support/zap/state.json` (macOS), `~/.config/zap/state.json` (Linux), or `%APPDATA%\zap\state.json` (Windows). zap manages this file — don't hand-edit.

## Built-in providers

| Provider | Command | Notes |
|---|---|---|
| Claude Code | `claude` | `--yolo` toggles `--dangerously-skip-permissions` |
| Codex CLI | `codex` | |
| Gemini CLI | `gemini` | |
| opencode | `opencode` | |

Providers not installed are shown grayed out with an install hint.

## Adding a new provider

Open a PR adding a YAML entry to [`internal/config/defaults.yaml`](internal/config/defaults.yaml):

```yaml
- id: myprovider
  name: My Provider
  command: myprovider-cli
  icon: "🚀"
  install_hint: "npm install -g myprovider"
  flags:
    - id: yolo
      label: "Skip safety prompts"
      flag: "--unsafe"
      default: false
```

That's it — no Go code required.

## How it works

- Interactive picker is built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).
- On launch, zap `chdir`s into the chosen folder and (on Unix) `syscall.Exec`s the provider command — zap disappears, the CLI owns the TTY directly. On Windows, zap stays as a parent process and forwards stdin/stdout/stderr + exit code.
- Provider detection uses `exec.LookPath`.

## License

[MIT](LICENSE)
