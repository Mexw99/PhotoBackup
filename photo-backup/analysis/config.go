package analysis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type portableAIConfig struct {
	Provider            string `json:"aiProvider"`
	FallbackProvider    string `json:"aiFallbackProvider"`
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
	loadDotEnv()
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
		setIfMissing("AI_FALLBACK_PROVIDER", config.FallbackProvider)
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

func loadDotEnv() {
	paths := make([]string, 0, 2)
	if workingDir, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(workingDir, ".env"))
	}
	if executable, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(executable), ".env"))
	}
	seen := make(map[string]bool)
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			line = strings.TrimPrefix(line, "export ")
			name, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			name = strings.TrimSpace(name)
			value = strings.TrimSpace(value)
			if name == "" {
				continue
			}
			if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
				value = value[1 : len(value)-1]
			}
			if _, exists := os.LookupEnv(name); !exists {
				_ = os.Setenv(name, value)
			}
		}
	}
	if strings.TrimSpace(os.Getenv("GEMINI_MODEL")) == "" {
		setIfMissing("GEMINI_MODEL", os.Getenv("PHOTO_BACKUP_AI_MODEL"))
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

func envInt(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil {
		return fallback
	}
	return value
}
