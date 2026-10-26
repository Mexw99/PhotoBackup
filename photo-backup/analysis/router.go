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
	yolo     ImageAnalyzer
	vision   ImageAnalyzer
	fallback ImageAnalyzer
}

// LocalStatus describes the two local engines without performing inference.
func (a *CombinedAnalyzer) LocalStatus(_ context.Context) (string, string) {
	yoloReady := false
	if yolo, ok := a.yolo.(*YOLOAnalyzer); ok {
		yoloReady = yolo.Configured()
	}
	blipState, blipMessage := "not_running", "ไม่พบ BLIP worker หรือโมเดล"
	if blip, ok := a.vision.(*BLIPAnalyzer); ok && blip.Configured() {
		blipState, blipMessage = "ready", "BLIP พร้อมใช้งาน"
	}
	if yoloReady && blipState == "ready" {
		if a.fallback != nil {
			return "ready", "YOLO และ BLIP พร้อมใช้งาน; Gemini จะช่วยเมื่อ YOLO ไม่พบ Tag"
		}
		return "ready", "YOLO และตัวสร้างคำอธิบายภาพพร้อมใช้งาน"
	}
	if yoloReady {
		message := "YOLO พร้อมใช้งาน; BLIP ยังไม่พร้อม: " + blipMessage
		if a.fallback != nil {
			message += "; Gemini จะช่วยเมื่อ YOLO ไม่พบ Tag"
		}
		return "partial", message
	}
	return "error", "YOLO ยังไม่พร้อม; " + blipMessage
}

func (a *CombinedAnalyzer) Analyze(path string) (AIResult, error) {
	type outcome struct {
		result AIResult
		err    error
	}
	visionResult := make(chan outcome, 1)
	go func() { result, err := a.vision.Analyze(path); visionResult <- outcome{result, err} }()

	yoloResult, yoloErr := a.yolo.Analyze(path)
	merged := AIResult{Tags: cleanTags(yoloResult.Tags)}
	var lastErr error
	if yoloErr != nil {
		lastErr = yoloErr
	}

	var fallbackResult AIResult
	var fallbackErr error
	if len(merged.Tags) == 0 && a.fallback != nil {
		attempts := envInt("AI_FALLBACK_RETRIES", 2)
		if attempts < 1 {
			attempts = 1
		}
		if attempts > 3 {
			attempts = 3
		}
		for attempt := 0; attempt < attempts; attempt++ {
			fallbackResult, fallbackErr = a.fallback.Analyze(path)
			if fallbackErr == nil && len(cleanTags(fallbackResult.Tags)) > 0 {
				merged.Tags = cleanTags(fallbackResult.Tags)
				break
			}
			if fallbackErr != nil {
				lastErr = fallbackErr
			}
		}
	}

	vision := <-visionResult
	if vision.err != nil {
		lastErr = vision.err
	} else {
		merged.Description = strings.TrimSpace(vision.result.Description)
		merged.Tags = append(merged.Tags, vision.result.Tags...)
	}
	if merged.Description == "" {
		merged.Description = strings.TrimSpace(fallbackResult.Description)
	}
	merged.Tags = cleanTags(merged.Tags)
	if merged.Description == "" && len(merged.Tags) == 0 && lastErr != nil {
		return AIResult{}, lastErr
	}
	return merged, nil
}

func (a *CombinedAnalyzer) Close() error {
	for _, analyzer := range []ImageAnalyzer{a.yolo, a.vision, a.fallback} {
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

// NewAnalyzerFromEnv creates one explicitly selected provider. Portable BLIP is
// the safe local default; cloud providers remain opt-in so opening the application
// cannot unexpectedly create charges.
func NewAnalyzerFromEnv() ImageAnalyzer {
	loadPortableConfig()
	providerSetting := strings.ToLower(strings.TrimSpace(os.Getenv("AI_PROVIDER")))
	if providerSetting == "" {
		providerSetting = "blip"
	}
	// Older config files may still contain a removed local provider name.
	// Treat unknown local values as the bundled BLIP pipeline so upgrades do
	// not silently disable AI.
	if providerSetting != "all" && providerSetting != "blip" && providerSetting != "hybrid" &&
		providerSetting != "gemini" && providerSetting != "openai" &&
		providerSetting != "gpt" && !strings.Contains(providerSetting, ",") {
		providerSetting = "blip"
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
			providers = append(providers, newLocalCombinedAnalyzer(false))
		}
		if len(providers) == 0 {
			return nil
		}
		return &MultiAnalyzer{providers: providers}
	}
	if providerSetting == "blip" || providerSetting == "hybrid" {
		if !localAIEnabled() {
			return nil
		}
		if strings.EqualFold(strings.TrimSpace(os.Getenv("YOLO_ENABLED")), "false") {
			return newLocalCaptionAnalyzer()
		}
		return newLocalCombinedAnalyzer(true)
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

func newLocalCaptionAnalyzer() ImageAnalyzer {
	return NewBLIPAnalyzerFromEnv()
}

func newLocalCombinedAnalyzer(withFallback bool) ImageAnalyzer {
	var fallback ImageAnalyzer
	if withFallback && strings.EqualFold(strings.TrimSpace(os.Getenv("AI_FALLBACK_PROVIDER")), "gemini") && envBool("AI_ALLOW_CLOUD") {
		fallback = NewGeminiAnalyzerFromEnv()
	}
	return &CombinedAnalyzer{yolo: NewYOLOAnalyzerFromEnv(), vision: newLocalCaptionAnalyzer(), fallback: fallback}
}

func envBool(name string) bool {
	value, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(name)))
	return err == nil && value
}

// localAIEnabled enables bundled local AI by default. An explicit false still
// disables it for CI or machines without the local models.
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
		provider = "blip"
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
	case "blip", "hybrid":
		model = "blip-image-captioning-base"
		if !strings.EqualFold(strings.TrimSpace(os.Getenv("YOLO_ENABLED")), "false") {
			model = "yolo11n+" + model
			provider = "local-portable"
		}
		if strings.EqualFold(strings.TrimSpace(os.Getenv("AI_FALLBACK_PROVIDER")), "gemini") && envBool("AI_ALLOW_CLOUD") && len(geminiAPIKeys()) > 0 {
			model += "+gemini-fallback"
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
