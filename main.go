package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pacseer/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

// entry point and app loop
func main() {
	loadConfig()

	m := tui.New()
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// Config loading code supporting standard Linux config paths
func loadConfig() {
	var candidatePaths []string

	// 1. Current working directory .env
	candidatePaths = append(candidatePaths, ".env")

	// 2. XDG Config directory: ~/.config/pacseer/config.env and ~/.config/pacseer/.env
	if configDir, err := os.UserConfigDir(); err == nil {
		candidatePaths = append(candidatePaths,
			filepath.Join(configDir, "pacseer", "config.env"),
			filepath.Join(configDir, "pacseer", ".env"),
			filepath.Join(configDir, "pacseer", "config"),
		)
	}

	// 3. User Home directory: ~/.pacseer.env
	if homeDir, err := os.UserHomeDir(); err == nil {
		candidatePaths = append(candidatePaths, filepath.Join(homeDir, ".pacseer.env"))
	}

	// 4. Executable directory .env
	if exe, err := os.Executable(); err == nil {
		candidatePaths = append(candidatePaths, filepath.Join(filepath.Dir(exe), ".env"))
	}

	for _, p := range candidatePaths {
		loadFileEnv(p)
	}
}

func loadFileEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}
