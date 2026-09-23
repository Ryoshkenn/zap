package launch

import "testing"

func TestBuildRemoteCommand(t *testing.T) {
	tests := []struct {
		name    string
		dir     string
		command string
		args    []string
		shell   string
		want    string
	}{
		{
			name:    "tilde dir preserves home expansion",
			dir:     "~/proj",
			command: "claude",
			want:    `exec "$SHELL" -lic 'cd ~/'\''proj'\'' && exec '\''claude'\'''`,
		},
		{
			name:    "bare tilde",
			dir:     "~",
			command: "codex",
			want:    `exec "$SHELL" -lic 'cd ~ && exec '\''codex'\'''`,
		},
		{
			name:    "absolute dir is fully quoted",
			dir:     "/srv/app",
			command: "gemini",
			want:    `exec "$SHELL" -lic 'cd '\''/srv/app'\'' && exec '\''gemini'\'''`,
		},
		{
			name:    "args are quoted",
			dir:     "~/x",
			command: "claude",
			args:    []string{"--dangerously-skip-permissions"},
			want:    `exec "$SHELL" -lic 'cd ~/'\''x'\'' && exec '\''claude'\'' '\''--dangerously-skip-permissions'\'''`,
		},
		{
			name:    "command with base args quotes each word",
			dir:     "~",
			command: "opencode run",
			args:    []string{"--auto"},
			want:    `exec "$SHELL" -lic 'cd ~ && exec '\''opencode'\'' '\''run'\'' '\''--auto'\'''`,
		},
		{
			name:    "custom shell",
			dir:     "",
			command: "opencode",
			shell:   "zsh",
			want:    `exec zsh -lic 'exec '\''opencode'\'''`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildRemoteCommand(tt.dir, tt.command, tt.args, tt.shell)
			if got != tt.want {
				t.Errorf("\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
}

func TestSSHArgsBareShell(t *testing.T) {
	got := SSHArgs("devbox", "~/x", "", nil, "")
	if len(got) != 2 || got[0] != "-t" || got[1] != "devbox" {
		t.Errorf("empty command should yield interactive shell args, got %v", got)
	}
}

func TestSSHArgsWithCommand(t *testing.T) {
	got := SSHArgs("me@host", "~", "claude", nil, "")
	if len(got) != 3 || got[0] != "-t" || got[1] != "me@host" {
		t.Fatalf("unexpected args: %v", got)
	}
}

func TestValidateSSHTarget(t *testing.T) {
	for _, ok := range []string{"devbox", "me@10.0.0.5", "ssh://me@host:2222"} {
		if err := ValidateSSHTarget(ok); err != nil {
			t.Errorf("%q should be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "-oProxyCommand=x", "me@host claude", "a\tb"} {
		if err := ValidateSSHTarget(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}
