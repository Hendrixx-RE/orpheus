package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"packichu/internal/pm"
)

func TestAnalyzerSingleflightDeduplication(t *testing.T) {
	var requestCount int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		time.Sleep(100 * time.Millisecond) // simulate API latency
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "Test package summary.\nVerdict: [SAFE]"
				}
			}]
		}`))
	}))
	defer server.Close()

	analyzer := &Analyzer{
		provider: Groq,
		model:    "openai/gpt-oss-120b",
		endpoint: server.URL,
		apiKey:   "test-key",
		client:   server.Client(),
	}

	pkg := &pm.Package{
		Name:    "curl",
		Version: "8.0.0",
	}

	const goroutines = 10
	var wg sync.WaitGroup
	results := make([]string, goroutines)
	errs := make([]error, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			res, err := analyzer.Analyze(context.Background(), pkg, []string{"curl"})
			results[idx] = res
			errs[idx] = err
		}(i)
	}

	wg.Wait()

	if count := atomic.LoadInt64(&requestCount); count != 1 {
		t.Fatalf("expected exactly 1 HTTP request to backend, got %d", count)
	}

	for i := 0; i < goroutines; i++ {
		if errs[i] != nil {
			t.Errorf("goroutine %d returned error: %v", i, errs[i])
		}
		if results[i] == "" {
			t.Errorf("goroutine %d returned empty result", i)
		}
	}
}

func TestRetryAfterDelayFloor(t *testing.T) {
	resp := &http.Response{
		Header: make(http.Header),
	}
	resp.Header.Set("x-ratelimit-reset-requests", "0.002")

	delay := retryAfterDelay(resp, 1*time.Second)
	if delay < 3*time.Second {
		t.Fatalf("expected delay floor >= 3s, got %v", delay)
	}
}

func TestProviderAutoDetection(t *testing.T) {
	t.Setenv("PACKICHU_PROVIDER", "")
	t.Setenv("PACSEER_PROVIDER", "")
	t.Setenv("ORPHEUS_PROVIDER", "")
	t.Setenv("GROQ_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	// Test Gemini detection
	t.Setenv("GEMINI_API_KEY", "test-gemini-key")
	a := New()
	if a.provider != Gemini {
		t.Errorf("expected Gemini provider, got %s", a.provider)
	}
	if a.model != defaultGeminiModel {
		t.Errorf("expected default Gemini model %s, got %s", defaultGeminiModel, a.model)
	}

	// Test Groq detection
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GROQ_API_KEY", "test-groq-key")
	a = New()
	if a.provider != Groq {
		t.Errorf("expected Groq provider, got %s", a.provider)
	}
	if a.model != defaultGroqModel {
		t.Errorf("expected default Groq model %s, got %s", defaultGroqModel, a.model)
	}
}
