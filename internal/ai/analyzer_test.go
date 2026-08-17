package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"orpheus/internal/pm"
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
