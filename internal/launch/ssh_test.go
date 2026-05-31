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
			want:    `bash -lc 'cd ~/'\''proj'\'' && exec '\''claude'\'''`,
		},
		{
			name:    "bare tilde",
			dir:     "~",
			command: "codex",
			want:    `bash -lc 'cd ~ && exec '\''codex'\'''`,
		},
		{
			name:    "absolute dir is fully quoted",
			dir:     "/srv/app",
			command: "gemini",
			want:    `bash -lc 'cd '\''/srv/app'\'' && exec '\''gemini'\'''`,
		},
		{
			name:    "args are quoted",
			dir:     "~/x",
			command: "claude",
			args:    []string{"--dangerously-skip-permissions"},
			want:    `bash -lc 'cd ~/'\''x'\'' && exec '\''claude'\'' '\''--dangerously-skip-permissions'\'''`,
		},
		{
			name:    "custom shell",
			dir:     "",
			command: "opencode",
			shell:   "zsh",
			want:    `zsh -lc 'exec '\''opencode'\'''`,
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
