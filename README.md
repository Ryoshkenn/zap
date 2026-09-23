# zap

A terminal launcher for AI coding CLIs and coding apps. Pick a folder, pick a provider (Claude Code, Codex, opencode, Kimi Code, Command Code, Gemini, Cursor, VS Code…), and zap.

```
  Pick a folder

▸ 📁 ~/Documents/code/zap  current
  ────────────────────────────────
  ⭐ ~/Documents/code
  🕘 ~/work/api-server
  🕘 ~/projects/notes
  ────────────────────────────────
  🔎 Browse folders…
  🌐 SSH / Remote…
  🔧 Settings…

  ↑/↓ move · enter select · / filter · f star/unstar · i settings · q quit
```

## Why

If you bounce between Claude Code, Codex, opencode, Kimi, and Command Code across many repos, you spend a surprising amount of time typing `cd ~/projects/foo && claude --dangerously-skip-permissions`. zap collapses that into one keystroke.

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
Pick a folder (current directory → favorites → recents → browse), pick a provider, launch. Per-provider flags are set once in Settings.

### Fast non-interactive
```sh
zap claude                      # launch Claude in $PWD
zap claude /path/to/repo        # launch Claude in /path/to/repo
zap claude --yolo               # add --dangerously-skip-permissions
zap claude --safe               # remove any default dangerous flags
zap codex
zap opencode                    # runs with --auto by default (--safe to drop it)
zap kimi --yolo                 # add --yolo (auto-approve tool calls)
zap commandcode --yolo          # Command Code, bypassing permission prompts
zap gemini ~/projects/foo
zap cursor ~/projects/foo
zap vscode ~/projects/foo
```

### Remote (SSH)
Launch a provider on another computer over SSH — same flow, different machine.

In the interactive picker choose **🌐 SSH / Remote…**, then add a computer (an
alias from your `~/.ssh/config`, or `user@host`), pick a remote folder, and pick
a provider. zap hands off to `ssh -t` and the CLI runs on that machine.

```sh
zap ssh devbox                  # open a shell on devbox
zap ssh devbox claude           # launch Claude in ~ on devbox
zap ssh me@host claude ~/api    # launch Claude in ~/api on me@host
zap ssh devbox codex --print    # show the ssh command without running it
zap ssh devbox claude --shell zsh   # force zsh instead of your remote login shell
```

zap connects with your normal ssh keys and `~/.ssh/config` — it stores no
passwords. The remote command runs in your remote login shell, started as a
login + interactive shell (`$SHELL -lic`), so PATH set up in `~/.profile`,
`~/.bashrc` or `~/.zshrc` (nvm, bun, `~/.local/bin`…) is picked up. A path under
your local home (an unquoted `~/api` your local shell already expanded) is sent
as `~/api` so it resolves on the remote. To force a specific shell, pass
`--shell zsh` (it's remembered per host).

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
zap last                        # re-launch the most recent folder + provider
zap claude --print              # print the command instead of running it
zap doctor                      # check config, ssh, and provider availability
zap list                        # show all providers + install status
zap config                      # print the resolved config path
zap config edit                 # open the config file in $EDITOR
zap uninstall                   # remove the zap binary (--purge also drops config/state)
zap --version
```

### Updating
```sh
zap update                      # check for a new release and install it
zap update --check              # only report whether an update is available
zap update --yes                # skip the confirmation prompt
```

zap upgrades itself the same way it was installed. Homebrew and Scoop installs
shell out to `brew upgrade zap` / `scoop update zap`, so the package manager
keeps ownership of its own bookkeeping. `go install` and manual installs get an
in-place binary swap: zap downloads the release archive for your platform,
verifies it against the release's SHA-256 checksums, and only then replaces the
binary — keeping the previous one aside to roll back to if the swap fails.

zap also checks for new releases in the background once every 24 hours and shows
a one-line notice when one is available. The result is cached in `state.json`, so
a launch inside that window costs no network call at all. Turn it off — or run a
check on demand — under **Updates** in the settings screen (`i` from the folder
picker, or the Settings entry in the list).

## Configuration

Optional. `zap` works out of the box. Override defaults by creating a `config.yaml` in zap's config directory — run `zap config` to print the exact path. That's `~/Library/Application Support/zap/config.yaml` (macOS), `~/.config/zap/config.yaml` (Linux), or `%APPDATA%\zap\config.yaml` (Windows):

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
| Codex CLI | `codex` | `--yolo` toggles `--dangerously-bypass-approvals-and-sandbox`; Settings also offers `--approve-for-me` |
| opencode | `opencode` | `--yolo` toggles `--auto` (default: on) |
| Kimi Code | `kimi` | `--yolo` toggles `--yolo`; Settings also offers `--auto` |
| Command Code | `command-code` | `--yolo` toggles `--yolo`; Settings also offers `--accept-edits` and `--trust` |
| Gemini CLI | `gemini` | `--yolo` toggles `--yolo` |
| Cursor | `/Applications/Cursor.app` or `cursor` | opens as an app by default; an installed app wins over a `cursor` CLI shim |
| VS Code | `code` or `/Applications/Visual Studio Code.app` | opens as an app by default |

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
- Provider detection uses `exec.LookPath` for CLIs and macOS app bundle detection for GUI coding apps.

## License

[MIT](LICENSE)
