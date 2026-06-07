package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"orpheus/internal/pm"
)

const (
	defaultModel   = "llama-3.3-70b-versatile"
	groqEndpoint   = "https://api.groq.com/openai/v1/chat/completions"
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

func (a *Analyzer) Analyze(ctx context.Context, pkg *pm.Package) (string, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GROQ_API_KEY not set — get a free key at console.groq.com")
	}

	body, _ := json.Marshal(map[string]any{
		"model": a.model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a Linux package analyzer. Give concise, honest analysis. No markdown headers or bullet points. Plain text only. End with exactly one line: Verdict: Keep / Review / Safe to remove.",
			},
			{
				"role":    "user",
				"content": buildPrompt(pkg),
			},
		},
		"max_tokens": 600,
	})

	backoff := 5 * time.Second
	for attempt := 0; attempt < 4; attempt++ {
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
		resp.Body.Close()
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

func buildPrompt(pkg *pm.Package) string {
	deps := "none"
	if len(pkg.Dependencies) > 0 {
		d := pkg.Dependencies
		if len(d) > 8 {
			d = d[:8]
		}
		deps = strings.Join(d, ", ")
	}
	reqBy := "nothing"
	if len(pkg.RequiredBy) > 0 {
		r := pkg.RequiredBy
		if len(r) > 6 {
			r = r[:6]
		}
		reqBy = strings.Join(r, ", ")
	}

	orphanStr := "No"
	if pkg.IsOrphan {
		orphanStr = "Yes — installed as a dependency but nothing requires it anymore"
	}

	installDate := "unknown"
	if !pkg.InstallDate.IsZero() {
		installDate = pkg.InstallDate.Format("Jan 02, 2006")
	}

	return fmt.Sprintf(`Analyze this Linux package in 3-4 sentences:
1. Its purpose on this system
2. Whether it's still useful (consider orphan status and dependents)
3. Blast radius if removed

Package: %s %s
Description: %s
Install reason: %s
Installed: %s
Size: %s
Orphan: %s
Required by: %s
Depends on: %s

End with exactly: Verdict: [Keep / Review / Safe to remove]`,
		pkg.Name, pkg.Version,
		pkg.Description,
		pkg.InstallReason,
		installDate,
		pkg.FormatSize(),
		orphanStr,
		reqBy,
		deps,
	)
}
