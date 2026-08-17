package tui

import (
	"testing"
)

func TestSplitVerdictAndCleanCommand(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantVerdict string
		wantCommand string
	}{
		{
			name:        "Standard separate command with parentheses",
			input:       "Neovim is a text editor.\nVerdict: [KEEP]\n(Command: nvim)",
			wantVerdict: "Verdict: [KEEP]",
			wantCommand: "nvim",
		},
		{
			name:        "Standard separate command without parentheses",
			input:       "Git is a version control system.\nVerdict: [KEEP]\nCommand: git",
			wantVerdict: "Verdict: [KEEP]",
			wantCommand: "git",
		},
		{
			name:        "Command with backticks",
			input:       "GCC is a compiler.\nVerdict: [KEEP]\nCommand: `gcc`",
			wantVerdict: "Verdict: [KEEP]",
			wantCommand: "gcc",
		},
		{
			name:        "Nil command filtered out",
			input:       "glibc is a core C library.\nVerdict: [KEEP]\n(Command: nil)",
			wantVerdict: "Verdict: [KEEP]",
			wantCommand: "",
		},
		{
			name:        "None command filtered out",
			input:       "Some library.\nVerdict: [SAFE]\nCommand: None",
			wantVerdict: "Verdict: [SAFE]",
			wantCommand: "",
		},
		{
			name:        "N/A command filtered out",
			input:       "Some header package.\nVerdict: [SAFE]\n(Command: N/A)",
			wantVerdict: "Verdict: [SAFE]",
			wantCommand: "",
		},
		{
			name:        "Embedded command in verdict line",
			input:       "Btop is a system monitor.\nVerdict: [KEEP] (Command: btop)",
			wantVerdict: "Verdict: [KEEP]",
			wantCommand: "btop",
		},
		{
			name:        "Terminal command prefix",
			input:       "Alacritty terminal emulator.\nVerdict: [KEEP]\nTerminal Command: alacritty",
			wantVerdict: "Verdict: [KEEP]",
			wantCommand: "alacritty",
		},
		{
			name:        "No command present",
			input:       "Just a plain description.\nVerdict: [SAFE]",
			wantVerdict: "Verdict: [SAFE]",
			wantCommand: "",
		},
		{
			name:        "Gemini markdown bold formatting",
			input:       "Neovim is an extensible editor.\n\n**Verdict:** [KEEP]\n**Command:** `nvim`",
			wantVerdict: "Verdict: [KEEP]",
			wantCommand: "nvim",
		},
		{
			name:        "Gemini bullet list formatting",
			input:       "* **Purpose:** Text editor.\n* **Verdict:** [KEEP]\n* **Command:** git",
			wantVerdict: "Verdict: [KEEP]",
			wantCommand: "git",
		},
		{
			name:        "Gemini header formatting with embedded command",
			input:       "Container runtime.\n### **Verdict:** [CAUTION] (Command: `docker`)",
			wantVerdict: "Verdict: [CAUTION]",
			wantCommand: "docker",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, gotVerdict, gotCommand := splitVerdict(tt.input)
			if gotVerdict != tt.wantVerdict {
				t.Errorf("splitVerdict() gotVerdict = %q, want %q", gotVerdict, tt.wantVerdict)
			}
			if gotCommand != tt.wantCommand {
				t.Errorf("splitVerdict() gotCommand = %q, want %q", gotCommand, tt.wantCommand)
			}
		})
	}
}
