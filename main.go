package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"orpheus/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

// entry point and app loop
func main() {
	loadEnv()
	m := tui.New()
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// Env loading code
func loadEnv() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	f, err := os.Open(filepath.Join(filepath.Dir(exe), ".env"))
	if err != nil {
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Fatal(err)
		}
	}()

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
		if os.Getenv(key) == "" {
			if err := os.Setenv(key, val); err != nil {
				log.Fatal(err)
			}
		}
	}
}
