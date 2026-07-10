// Package ai used for the ai integration
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"orpheus/internal/pm"
)

const (
	defaultModel = "llama-3.3-70b-versatile"
	groqEndpoint = "https://api.groq.com/openai/v1/chat/completions"
)

type Analyzer struct {
	model  string
	client *http.Client
}

func New() *Analyzer {
	model := os.Getenv("ORPHEUS_MODEL")
	if model == "" {
		model = defaultModel
	}
	return &Analyzer{
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *Analyzer) Analyze(ctx context.Context, pkg *pm.Package, explicitNames []string) (string, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GROQ_API_KEY not set — get a free key at console.groq.com")
	}

	body, _ := json.Marshal(map[string]any{
		"model": a.model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a Linux package analyzer. Give concise, honest analysis. No markdown headers or bullet points. Plain text only.",
			},
			{
				"role":    "user",
				"content": buildPrompt(pkg, explicitNames),
			},
		},
		"max_tokens": 300,
	})

	backoff := 5 * time.Second
	for attempt := range 4 {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, groqEndpoint, bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := a.client.Do(req)
		if err != nil {
			return "", err
		}

		data, err := io.ReadAll(resp.Body)
		if err := resp.Body.Close(); err != nil {
			log.Fatal(err)
		}
		if err != nil {
			return "", err
		}

		if resp.StatusCode == 429 {
			continue
		}
		if resp.StatusCode != 200 {
			return "", fmt.Errorf("groq error %d: %s", resp.StatusCode, extractError(data))
		}

		text, err := extractContent(data)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(text), nil
	}
	return "", fmt.Errorf("rate limited — try again in a moment")
}

func extractContent(data []byte) (string, error) {
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty response")
	}
	return resp.Choices[0].Message.Content, nil
}

func extractError(data []byte) string {
	var resp struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return string(data)
	}
	return resp.Error.Message
}

func buildPrompt(pkg *pm.Package, explicitNames []string) string {
	deps := "none"
	if len(pkg.Dependencies) > 0 {
		d := pkg.Dependencies
		if len(d) > 8 {
			d = d[:8]
		}
		deps = strings.Join(d, ", ")
	}

	installDate := "unknown"
	if !pkg.InstallDate.IsZero() {
		installDate = pkg.InstallDate.Format("Jan 02, 2006")
	}

	explicitStr := strings.Join(explicitNames, ", ")

	return fmt.Sprintf(`Analyze this Linux package in 3-4 sentences:
1. Its purpose on this system
2. Why the user might have installed this specific package, given the other explicit packages on their system
3. What would happen if the user removed this package
Package: %s %s
Description: %s
Install reason: %s
Installed: %s
Size: %s
Depends on: %s
All explicit packages on this system: %s`,
		pkg.Name, pkg.Version,
		pkg.Description,
		pkg.InstallReason,
		installDate,
		pkg.FormatSize(),
		deps,
		explicitStr,
	)
}
