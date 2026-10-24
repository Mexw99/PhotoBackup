package analysis

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type OllamaAnalyzer struct {
	endpoint string
	model    string
	client   *http.Client
}

func NewOllamaAnalyzerFromEnv() *OllamaAnalyzer {
	endpoint := strings.TrimRight(strings.TrimSpace(os.Getenv("OLLAMA_URL")), "/")
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}
	model := strings.TrimSpace(os.Getenv("OLLAMA_MODEL"))
	if model == "" {
		model = "qwen2.5vl:3b"
	}
	return &OllamaAnalyzer{endpoint: endpoint, model: model, client: &http.Client{Timeout: 90 * time.Second}}
}

func (a *OllamaAnalyzer) Analyze(path string) (AIResult, error) {
	// Resize only the request payload. The backup file and its ContentHash are
	// never changed, while smaller vision input is much faster and uses less RAM.
	data, mimeType, err := readImageForCloudAI(path)
	if err != nil {
		return AIResult{}, err
	}
	body := map[string]interface{}{
		"model":      a.model,
		"stream":     false,
		"format":     "json",
		"keep_alive": "10m",
		"options": map[string]interface{}{
			"temperature": 0.1,
			"num_predict": envIntValue("OLLAMA_NUM_PREDICT", 160),
		},
		"messages": []interface{}{map[string]interface{}{
			"role":      "user",
			"content":   analysisPrompt,
			"images":    []string{base64.StdEncoding.EncodeToString(data)},
			"mime_type": mimeType,
		}},
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return AIResult{}, err
	}
	req, err := http.NewRequest(http.MethodPost, a.endpoint+"/api/chat", bytes.NewReader(encoded))
	if err != nil {
		return AIResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := a.client.Do(req)
	if err != nil {
		return AIResult{}, fmt.Errorf("เรียก Ollama ไม่ได้: %w", err)
	}
	defer res.Body.Close()
	var response struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Error string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return AIResult{}, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return AIResult{}, NewProviderError("Ollama", res)
	}
	return parseAIResult(response.Message.Content)
}

func envIntValue(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
