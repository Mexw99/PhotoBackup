package analysis

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
)

type MultiAnalyzer struct {
	providers []ImageAnalyzer
	next      atomic.Uint64
}

func (a *MultiAnalyzer) ProviderCount() int {
	if a == nil {
		return 0
	}
	return len(a.providers)
}

type ImageAnalyzer interface {
	Analyze(string) (AIResult, error)
}

// CombinedAnalyzer runs fast local object detection and vision description in
// parallel, then merges their tags. A failure in one local engine does not
// discard a successful result from the other.
type CombinedAnalyzer struct {
	yolo   ImageAnalyzer
	vision ImageAnalyzer
}

// LocalStatus describes the two local engines without performing inference.
func (a *CombinedAnalyzer) LocalStatus(ctx context.Context) (string, string) {
	yoloReady := false
	if yolo, ok := a.yolo.(*YOLOAnalyzer); ok {
		yoloReady = yolo.Configured()
	}
	ollamaState, ollamaMessage := "disabled", "Ollama ไม่ได้ตั้งค่า"
	if vision, ok := a.vision.(*OllamaAnalyzer); ok {
		health := vision.HealthCheck(ctx)
		ollamaState, ollamaMessage = health.State, health.Message
	}
	if yoloReady && ollamaState == "ready" {
		return "ready", "YOLO และ Ollama พร้อมใช้งาน"
	}
	if yoloReady {
		return "partial", "YOLO พร้อมใช้งาน; Ollama ยังไม่พร้อม: " + ollamaMessage
	}
	return "error", "YOLO ยังไม่พร้อม; " + ollamaMessage
}

func (a *CombinedAnalyzer) Analyze(path string) (AIResult, error) {
	type outcome struct {
		result AIResult
		err    error
	}
	results := make(chan outcome, 2)
	go func() { result, err := a.yolo.Analyze(path); results <- outcome{result, err} }()
	go func() { result, err := a.vision.Analyze(path); results <- outcome{result, err} }()
	first, second := <-results, <-results
	merged := AIResult{}
	var lastErr error
	for _, item := range []outcome{first, second} {
		if item.err != nil {
			lastErr = item.err
			continue
		}
		if merged.Description == "" {
			merged.Description = item.result.Description
		}
		merged.Tags = append(merged.Tags, item.result.Tags...)
	}
	merged.Tags = cleanTags(merged.Tags)
	if merged.Description == "" && len(merged.Tags) == 0 && lastErr != nil {
		return AIResult{}, lastErr
	}
	return merged, nil
}

func (a *CombinedAnalyzer) Close() error {
	for _, analyzer := range []ImageAnalyzer{a.yolo, a.vision} {
		if closer, ok := analyzer.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

const (
	DefaultAnalysisProfile = "vision-tags"
	DefaultPromptVersion   = "v1"
)

// NewAnalyzerFromEnv creates one explicitly selected provider. Local Ollama is
// the safe default; cloud providers remain opt-in so opening the application
// cannot unexpectedly create charges.
func NewAnalyzerFromEnv() ImageAnalyzer {
	loadPortableConfig()
	providerSetting := strings.ToLower(strings.TrimSpace(os.Getenv("AI_PROVIDER")))
	if providerSetting == "" {
		providerSetting = "ollama"
	}
	if providerSetting == "all" || strings.Contains(providerSetting, ",") {
		providers := make([]ImageAnalyzer, 0, 3)
		if envBool("AI_ALLOW_CLOUD") {
			providerList := strings.Split(providerSetting, ",")
			if providerSetting == "all" {
				providerList = []string{"gemini", "openai"}
			}
			for _, provider := range providerList {
				provider = strings.ToLower(strings.TrimSpace(provider))
				switch provider {
				case "all", "gemini":
					if analyzer := NewGeminiAnalyzerFromEnv(); analyzer != nil {
						providers = append(providers, analyzer)
					}
				case "openai", "gpt":
					if analyzer := NewOpenAIAnalyzerFromEnv(); analyzer != nil {
						providers = append(providers, analyzer)
					}
				}
			}
		}
		if localAIEnabled() {
			providers = append(providers, NewOllamaAnalyzerFromEnv())
		}
		if len(providers) == 0 {
			return nil
		}
		return &MultiAnalyzer{providers: providers}
	}
	if providerSetting == "ollama" {
		if !localAIEnabled() {
			return nil
		}
		if strings.EqualFold(strings.TrimSpace(os.Getenv("YOLO_ENABLED")), "false") {
			return NewOllamaAnalyzerFromEnv()
		}
		return &CombinedAnalyzer{yolo: NewYOLOAnalyzerFromEnv(), vision: NewOllamaAnalyzerFromEnv()}
	}
	if !envBool("AI_ALLOW_CLOUD") {
		return nil
	}
	switch providerSetting {
	case "gemini":
		return NewGeminiAnalyzerFromEnv()
	case "openai", "gpt":
		return NewOpenAIAnalyzerFromEnv()
	default:
		return nil
	}
}

func envBool(name string) bool {
	value, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(name)))
	return err == nil && value
}

// localAIEnabled enables Ollama by default when the setting is absent. An
// explicit false still disables it for CI or machines without Ollama.
func localAIEnabled() bool {
	value, present := os.LookupEnv("AI_ALLOW_LOCAL")
	if !present || strings.TrimSpace(value) == "" {
		return true
	}
	return envBool("AI_ALLOW_LOCAL")
}

// ConfiguredProviderModel returns metadata for cache records without making a
// network request. It intentionally never returns an API key.
func ConfiguredProviderModel() (string, string) {
	loadPortableConfig()
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("AI_PROVIDER")))
	if provider == "" {
		provider = "ollama"
	}
	switch provider {
	case "gpt":
		provider = "openai"
	case "all":
		provider = "multi"
	}
	model := ""
	switch provider {
	case "gemini":
		model = strings.TrimSpace(os.Getenv("GEMINI_MODEL"))
		if model == "" {
			model = defaultGeminiModel
		}
	case "openai":
		model = strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
		if model == "" {
			model = "gpt-4.1-mini"
		}
	case "ollama":
		model = strings.TrimSpace(os.Getenv("OLLAMA_MODEL"))
		if model == "" {
			model = "qwen2.5vl:3b"
		}
		if !strings.EqualFold(strings.TrimSpace(os.Getenv("YOLO_ENABLED")), "false") {
			model = "yolo11n+" + model
			provider = "local-hybrid"
		}
	case "multi":
		model = "multi-provider"
	}
	return provider, model
}

func (a *MultiAnalyzer) Analyze(path string) (AIResult, error) {
	if a == nil || len(a.providers) == 0 {
		return AIResult{}, nil
	}
	start := a.next.Add(1) - 1
	var lastErr error
	for attempt := 0; attempt < len(a.providers); attempt++ {
		provider := a.providers[(start+uint64(attempt))%uint64(len(a.providers))]
		result, err := provider.Analyze(path)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	return AIResult{}, lastErr
}

func atomicNext(value *uint64) uint64 { return atomic.AddUint64(value, 1) - 1 }
