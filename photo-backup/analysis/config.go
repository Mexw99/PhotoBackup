package analysis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type portableAIConfig struct {
	Provider            string `json:"aiProvider"`
	GeminiKeys          string `json:"geminiApiKeys"`
	GeminiModel         string `json:"geminiModel"`
	OpenAIKeys          string `json:"openaiApiKeys"`
	OpenAIModel         string `json:"openaiModel"`
	YOLOEnabled         string `json:"yoloEnabled"`
	YOLOModel           string `json:"yoloModelPath"`
	YOLOPython          string `json:"yoloPython"`
	YOLOScript          string `json:"yoloScript"`
	YOLOWorker          string `json:"yoloWorker"`
	CaptionProvider     string `json:"captionProvider"`
	BLIPModel           string `json:"blipModelPath"`
	BLIPTranslatorModel string `json:"blipTranslatorModelPath"`
	BLIPPython          string `json:"blipPython"`
	BLIPScript          string `json:"blipScript"`
	BLIPWorker          string `json:"blipWorker"`
}

// loadPortableConfig loads config.json from the working directory or beside
// the executable. Environment variables still take precedence when present.
func loadPortableConfig() {
	paths := make([]string, 0, 2)
	if workingDir, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(workingDir, "config.json"))
	}
	if executable, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(executable), "config.json"))
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var config portableAIConfig
		if json.Unmarshal(data, &config) != nil {
			continue
		}
		setIfMissing("AI_PROVIDER", config.Provider)
		setIfMissing("GEMINI_API_KEYS", config.GeminiKeys)
		setIfMissing("GEMINI_MODEL", config.GeminiModel)
		setIfMissing("OPENAI_API_KEYS", config.OpenAIKeys)
		setIfMissing("OPENAI_MODEL", config.OpenAIModel)
		setIfMissing("YOLO_ENABLED", config.YOLOEnabled)
		setPathIfMissing("YOLO_MODEL_PATH", config.YOLOModel, filepath.Dir(path))
		setPathIfMissing("YOLO_PYTHON", config.YOLOPython, filepath.Dir(path))
		setPathIfMissing("YOLO_SCRIPT", config.YOLOScript, filepath.Dir(path))
		setPathIfMissing("YOLO_WORKER", config.YOLOWorker, filepath.Dir(path))
		setIfMissing("AI_CAPTION_PROVIDER", config.CaptionProvider)
		setPathIfMissing("BLIP_MODEL_PATH", config.BLIPModel, filepath.Dir(path))
		setPathIfMissing("BLIP_TRANSLATOR_MODEL_PATH", config.BLIPTranslatorModel, filepath.Dir(path))
		setPathIfMissing("BLIP_PYTHON", config.BLIPPython, filepath.Dir(path))
		setPathIfMissing("BLIP_SCRIPT", config.BLIPScript, filepath.Dir(path))
		setPathIfMissing("BLIP_WORKER", config.BLIPWorker, filepath.Dir(path))
		return
	}
}

func setIfMissing(name, value string) {
	if strings.TrimSpace(os.Getenv(name)) == "" && strings.TrimSpace(value) != "" {
		_ = os.Setenv(name, value)
	}
}

func setPathIfMissing(name, value, baseDir string) {
	value = strings.TrimSpace(value)
	if value == "" || strings.TrimSpace(os.Getenv(name)) != "" {
		return
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(baseDir, value)
	}
	_ = os.Setenv(name, value)
}
