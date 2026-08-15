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
	"sync"
	"time"

	"orpheus/internal/pm"
)

type sfCall struct {
	wg  sync.WaitGroup
	val string
	err error
}

type singleflightGroup struct {
	mu sync.Mutex
	m  map[string]*sfCall
}

func (g *singleflightGroup) Do(key string, fn func() (string, error)) (string, error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*sfCall)
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}
	c := new(sfCall)
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err
}

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
	provider         Provider
	model            string
	endpoint         string
	apiKey           string
	client           *http.Client
	sf               singleflightGroup
	mu               sync.Mutex
	rateLimitedUntil time.Time
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

	a.mu.Lock()
	if time.Now().Before(a.rateLimitedUntil) {
		remaining := time.Until(a.rateLimitedUntil).Round(time.Second)
		a.mu.Unlock()
		return "", fmt.Errorf("rate limited: cooling down for %s", remaining)
	}
	a.mu.Unlock()

	key := pkg.Name + "@" + pkg.Version
	return a.sf.Do(key, func() (string, error) {
		res, err := a.analyzeUncached(ctx, pkg, explicitNames)
		if err != nil && strings.Contains(err.Error(), "rate limited") {
			a.mu.Lock()
			a.rateLimitedUntil = time.Now().Add(30 * time.Second)
			a.mu.Unlock()
		}
		return res, err
	})
}

func (a *Analyzer) analyzeUncached(ctx context.Context, pkg *pm.Package, explicitNames []string) (string, error) {

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
	const maxAttempts = 6
	const maxBackoff = 5 * time.Minute

	for attempt := range maxAttempts {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
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
			// Respect the Retry-After header if present — providers like Groq
			// set this to the exact number of seconds to wait. Fall back to
			// our exponential backoff if the header is absent or unparseable.
			wait := retryAfterDelay(resp, backoff)
			if wait > maxBackoff {
				// The API wants us to wait longer than our cap — give up now
				// and let the next app launch re-attempt (cache miss = retry).
				return "", fmt.Errorf("rate limited: retry after %s (skipping for now, will retry next launch)", wait.Round(time.Second))
			}
			backoff = wait
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
	return "", fmt.Errorf("rate limited after %d attempts — skipping, will retry next launch", maxAttempts)
}

const minRetryDelay = 3 * time.Second

// retryAfterDelay reads the Retry-After or x-ratelimit-reset-requests header
// from a 429 response and returns how long to wait. Falls back to the provided
// default if the header is absent or unparseable. Enforces a minimum backoff floor
// of 3 seconds to prevent sub-millisecond retry loops on small header values (e.g. 0.002s).
func retryAfterDelay(resp *http.Response, fallback time.Duration) time.Duration {
	delay := fallback
	// Groq uses x-ratelimit-reset-requests (seconds as float string, e.g. "3.5s" or "3500ms")
	// Standard HTTP uses Retry-After (integer seconds or HTTP date)
	for _, header := range []string{"retry-after", "x-ratelimit-reset-requests"} {
		val := strings.TrimSpace(resp.Header.Get(header))
		if val == "" {
			continue
		}
		// Try parsing as a Go duration string (e.g. "3.5s", "500ms")
		if d, err := time.ParseDuration(val); err == nil {
			delay = d
			break
		}
		// Try parsing as plain integer seconds
		var secs float64
		if _, err := fmt.Sscanf(val, "%f", &secs); err == nil && secs > 0 {
			delay = time.Duration(secs * float64(time.Second))
			break
		}
	}
	if delay < minRetryDelay {
		delay = minRetryDelay
	}
	return delay
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
