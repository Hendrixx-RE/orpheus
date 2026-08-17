package tui

import (
	"strings"
	"testing"
)

func TestWrapText(t *testing.T) {
	longError := "AI unavailable: Post \"https://api.groq.com/openai/v1/chat/completions\": dial tcp: lookup api.groq.com: no such host"
	width := 30
	wrapped := wrapText(longError, width)

	lines := strings.Split(wrapped, "\n")
	for i, l := range lines {
		if len(l) > width {
			t.Errorf("line %d exceeds width %d: len=%d (%q)", i, width, len(l), l)
		}
	}

	// Ensure all words are present
	words := strings.Fields(longError)
	for _, w := range words {
		if len(w) <= width && !strings.Contains(wrapped, w) {
			t.Errorf("expected wrapped text to contain %q", w)
		}
	}
}

func TestWrapTextMultiline(t *testing.T) {
	multiline := "Line 1: Error occurred\nLine 2: Please verify your GROQ_API_KEY\nLine 3: Rate limit exceeded"
	wrapped := wrapText(multiline, 25)

	lines := strings.Split(wrapped, "\n")
	if len(lines) < 3 {
		t.Errorf("expected at least 3 lines, got %d", len(lines))
	}
	for i, l := range lines {
		if len(l) > 25 {
			t.Errorf("line %d exceeds width 25: len=%d (%q)", i, len(l), l)
		}
	}
}
