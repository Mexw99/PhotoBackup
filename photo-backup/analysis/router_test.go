package analysis

import "testing"

func TestNewAnalyzerFromEnvAllBuildsConfiguredProviders(t *testing.T) {
	t.Setenv("AI_PROVIDER", "all")
	t.Setenv("AI_ALLOW_CLOUD", "true")
	t.Setenv("AI_ALLOW_LOCAL", "false")
	t.Setenv("GEMINI_API_KEYS", "test-gemini-key")
	t.Setenv("OPENAI_API_KEYS", "test-openai-key")

	analyzer := NewAnalyzerFromEnv()
	multi, ok := analyzer.(*MultiAnalyzer)
	if !ok {
		t.Fatalf("analyzer type = %T, want *MultiAnalyzer", analyzer)
	}
	if multi.ProviderCount() != 2 {
		t.Fatalf("provider count = %d, want 2", multi.ProviderCount())
	}
	provider, model := ConfiguredProviderModel()
	if provider != "multi" || model != "multi-provider" {
		t.Fatalf("metadata = %q/%q, want multi/multi-provider", provider, model)
	}
}

func TestNewAnalyzerFromEnvDoesNotEnableCloudWithoutConsent(t *testing.T) {
	t.Setenv("AI_PROVIDER", "gemini")
	t.Setenv("AI_ALLOW_CLOUD", "false")
	t.Setenv("GEMINI_API_KEYS", "test-gemini-key")
	if analyzer := NewAnalyzerFromEnv(); analyzer != nil {
		t.Fatalf("cloud analyzer was enabled without AI_ALLOW_CLOUD: %T", analyzer)
	}
}

func TestNewAnalyzerFromEnvDefaultsToPortableLocalAI(t *testing.T) {
	t.Setenv("AI_PROVIDER", "")
	t.Setenv("AI_ALLOW_LOCAL", "")
	t.Setenv("AI_ALLOW_CLOUD", "false")

	analyzer := NewAnalyzerFromEnv()
	if _, ok := analyzer.(*CombinedAnalyzer); !ok {
		t.Fatalf("default analyzer = %T, want *CombinedAnalyzer", analyzer)
	}
	provider, model := ConfiguredProviderModel()
	if provider != "local-portable" || model != "yolo11n+blip-image-captioning-base" {
		t.Fatalf("default metadata = %q/%q, want local-portable/yolo11n+blip-image-captioning-base", provider, model)
	}
}

func TestExplicitLocalDisableStillDisablesPortableLocalAI(t *testing.T) {
	t.Setenv("AI_PROVIDER", "blip")
	t.Setenv("AI_ALLOW_LOCAL", "false")
	if analyzer := NewAnalyzerFromEnv(); analyzer != nil {
		t.Fatalf("portable local AI was enabled despite explicit AI_ALLOW_LOCAL=false: %T", analyzer)
	}
}

func TestParseAIResultAcceptsMarkdownWrappedJSON(t *testing.T) {
	result, err := parseAIResult("คำตอบ:\n```json\n{\"description\":\"สุนัขในสวน\",\"tags\":[\"สุนัข\",\"สวน\",\"สวน\"]}\n```")
	if err != nil {
		t.Fatalf("parseAIResult returned error: %v", err)
	}
	if result.Description != "สุนัขในสวน" || len(result.Tags) != 2 {
		t.Fatalf("unexpected parsed result: %+v", result)
	}
}

func TestGeminiAPIKeysCombinesBothEnvironmentVariables(t *testing.T) {
	t.Setenv("GEMINI_API_KEYS", "first,second")
	t.Setenv("GEMINI_API_KEY", "second,third")
	keys := geminiAPIKeys()
	if len(keys) != 3 || keys[0] != "first" || keys[1] != "second" || keys[2] != "third" {
		t.Fatalf("keys = %v, want three unique keys in order", keys)
	}
}

type fixedAnalyzer struct {
	result AIResult
	err    error
}

func TestCombinedAnalyzerKeepsYOLOTagsWhenVisionIsUnavailable(t *testing.T) {
	analyzer := &CombinedAnalyzer{
		yolo:   fixedAnalyzer{result: AIResult{Tags: []string{"cat"}}},
		vision: fixedAnalyzer{err: &ProviderError{Message: "BLIP is not available"}},
	}
	result, err := analyzer.Analyze("test.jpg")
	if err != nil {
		t.Fatalf("YOLO fallback returned error: %v", err)
	}
	if len(result.Tags) != 1 || result.Tags[0] != "cat" {
		t.Fatalf("unexpected YOLO fallback result: %+v", result)
	}
}

func (a fixedAnalyzer) Analyze(string) (AIResult, error) { return a.result, a.err }

func TestCombinedAnalyzerMergesLocalTagsAndDescription(t *testing.T) {
	analyzer := &CombinedAnalyzer{
		yolo:   fixedAnalyzer{result: AIResult{Tags: []string{"cat", "chair"}}},
		vision: fixedAnalyzer{result: AIResult{Description: "แมวอยู่บนเก้าอี้", Tags: []string{"แมว", "chair"}}},
	}
	result, err := analyzer.Analyze("test.jpg")
	if err != nil {
		t.Fatalf("combined analyzer returned error: %v", err)
	}
	if result.Description != "แมวอยู่บนเก้าอี้" || len(result.Tags) != 3 {
		t.Fatalf("unexpected combined result: %+v", result)
	}
}

func TestCombinedAnalyzerUsesGeminiFallbackWhenYOLOHasNoTags(t *testing.T) {
	t.Setenv("AI_FALLBACK_RETRIES", "2")
	fallback := &sequenceAnalyzer{results: []AIResult{
		{},
		{Description: "แมวอยู่ในบ้าน", Tags: []string{"แมว", "บ้าน"}},
	}}
	analyzer := &CombinedAnalyzer{
		yolo:     fixedAnalyzer{result: AIResult{}},
		vision:   fixedAnalyzer{result: AIResult{Description: "คำอธิบายจาก BLIP"}},
		fallback: fallback,
	}
	result, err := analyzer.Analyze("test.jpg")
	if err != nil {
		t.Fatalf("fallback returned error: %v", err)
	}
	if fallback.calls != 2 || result.Description != "คำอธิบายจาก BLIP" || len(result.Tags) != 2 {
		t.Fatalf("unexpected fallback result: calls=%d result=%+v", fallback.calls, result)
	}
}

func TestCombinedAnalyzerSkipsGeminiWhenYOLOHasTags(t *testing.T) {
	fallback := &sequenceAnalyzer{results: []AIResult{{Tags: []string{"cloud"}}}}
	analyzer := &CombinedAnalyzer{
		yolo:     fixedAnalyzer{result: AIResult{Tags: []string{"cat"}}},
		vision:   fixedAnalyzer{},
		fallback: fallback,
	}
	result, err := analyzer.Analyze("test.jpg")
	if err != nil {
		t.Fatalf("YOLO result returned error: %v", err)
	}
	if fallback.calls != 0 || len(result.Tags) != 1 || result.Tags[0] != "cat" {
		t.Fatalf("Gemini should be skipped when YOLO has tags: calls=%d result=%+v", fallback.calls, result)
	}
}

type sequenceAnalyzer struct {
	results []AIResult
	calls   int
}

func (a *sequenceAnalyzer) Analyze(string) (AIResult, error) {
	a.calls++
	if len(a.results) == 0 {
		return AIResult{}, nil
	}
	result := a.results[0]
	a.results = a.results[1:]
	return result, nil
}
