package analysis

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type healthRoundTripper func(*http.Request) (*http.Response, error)

func (f healthRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestOllamaHealthCheckReportsReadyAndMissingModel(t *testing.T) {
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/tags" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"models":[{"name":"qwen2.5vl:3b"}]}`))
	})
	newTestClient := func() *http.Client {
		return &http.Client{Transport: healthRoundTripper(func(request *http.Request) (*http.Response, error) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			return recorder.Result(), nil
		})}
	}
	t.Setenv("OLLAMA_URL", "http://test-ollama.invalid")
	t.Setenv("OLLAMA_MODEL", "qwen2.5vl:3b")
	readyAnalyzer := NewOllamaAnalyzerFromEnv()
	readyAnalyzer.client = newTestClient()
	ready := readyAnalyzer.HealthCheck(context.Background())
	if ready.State != "ready" {
		t.Fatalf("health state=%q message=%q, want ready", ready.State, ready.Message)
	}
	t.Setenv("OLLAMA_MODEL", "llava:latest")
	missingAnalyzer := NewOllamaAnalyzerFromEnv()
	missingAnalyzer.client = newTestClient()
	missing := missingAnalyzer.HealthCheck(context.Background())
	if missing.State != "model_missing" {
		t.Fatalf("health state=%q, want model_missing", missing.State)
	}
}
