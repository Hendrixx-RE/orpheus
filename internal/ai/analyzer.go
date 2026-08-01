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
	defaultGroqModel      = "llama-3.3-70b-versatile"
	defaultOpenAIModel    = "gpt-4o-mini"
	defaultGeminiModel    = "gemini-1.5-flash"
	defaultAnthropicModel = "claude-3-5-haiku-latest"

	groqEndpoint      = "https://api.groq.com/openai/v1/chat/completions"
	openAIEndpoint    = "https://api.openai.com/v1/chat/completions"
	geminiEndpoint    = "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions"
	anthropicEndpoint = "https://api.anthropic.com/v1/messages"
)

type Provider string

const (
	Groq      Provider = "groq"
	OpenAI    Provider = "openai"
	Gemini    Provider = "gemini"
	Anthropic Provider = "anthropic"
)

type Analyzer struct {
	provider Provider
	model    string
	endpoint string
	apiKey   string
	client   *http.Client
}

func New() *Analyzer {
	providerStr := strings.ToLower(os.Getenv("ORPHEUS_PROVIDER"))
	if providerStr == "" {
		providerStr = "groq"
	}

	a := &Analyzer{
		provider: Provider(providerStr),
		client:   &http.Client{Timeout: 30 * time.Second},
	}

	model := os.Getenv("ORPHEUS_MODEL")
	
	switch a.provider {
	case OpenAI:
		a.endpoint = openAIEndpoint
		a.apiKey = os.Getenv("OPENAI_API_KEY")
		a.model = model
		if a.model == "" {
			a.model = defaultOpenAIModel
		}
	case Gemini:
		a.endpoint = geminiEndpoint
		a.apiKey = os.Getenv("GEMINI_API_KEY")
		a.model = model
		if a.model == "" {
			a.model = defaultGeminiModel
		}
	case Anthropic:
		a.endpoint = anthropicEndpoint
		a.apiKey = os.Getenv("ANTHROPIC_API_KEY")
		a.model = model
		if a.model == "" {
			a.model = defaultAnthropicModel
		}
	default:
		a.provider = Groq
		a.endpoint = groqEndpoint
		a.apiKey = os.Getenv("GROQ_API_KEY")
		a.model = model
		if a.model == "" {
			a.model = defaultGroqModel
		}
	}

	return a
}

func (a *Analyzer) Analyze(ctx context.Context, pkg *pm.Package, explicitNames []string) (string, error) {
	if a.apiKey == "" {
		return "", fmt.Errorf("%s API key not set in .env", strings.ToUpper(string(a.provider)))
	}

	var body []byte
	systemPrompt := "You are a Linux package analyzer. Give concise, honest analysis. No markdown headers or bullet points. Plain text only."
	userPrompt := buildPrompt(pkg, explicitNames)

	if a.provider == Anthropic {
		body, _ = json.Marshal(map[string]any{
			"model":      a.model,
			"system":     systemPrompt,
			"messages":   []map[string]string{{"role": "user", "content": userPrompt}},
			"max_tokens": 300,
		})
	} else {
		// OpenAI compatible format (Groq, OpenAI, Gemini)
		body, _ = json.Marshal(map[string]any{
			"model": a.model,
			"messages": []map[string]string{
				{"role": "system", "content": systemPrompt},
				{"role": "user", "content": userPrompt},
			},
			"max_tokens": 300,
		})
	}

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

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, bytes.NewReader(body))
		if err != nil {
			return "", err
		}

		if a.provider == Anthropic {
			req.Header.Set("x-api-key", a.apiKey)
			req.Header.Set("anthropic-version", "2023-06-01")
		} else {
			req.Header.Set("Authorization", "Bearer "+a.apiKey)
		}
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
			return "", fmt.Errorf("%s error %d: %s", a.provider, resp.StatusCode, extractError(data, a.provider))
		}

		text, err := extractContent(data, a.provider)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(text), nil
	}
	return "", fmt.Errorf("rate limited — try again in a moment")
}

func extractContent(data []byte, provider Provider) (string, error) {
	if provider == Anthropic {
		var resp struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return "", err
		}
		if len(resp.Content) == 0 {
			return "", fmt.Errorf("empty response")
		}
		return resp.Content[0].Text, nil
	}

	// OpenAI compatible
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

func extractError(data []byte, provider Provider) string {
	if provider == Anthropic {
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
4. The exact terminal command to launch this package. End your response with a new line containing exactly: (Command: <command>)
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
