package analysis

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

const defaultGeminiModel = "gemini-2.5-flash"

// AIResult is the structured result returned by the image understanding model.
type AIResult struct {
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// GeminiAnalyzer calls Gemini only when GEMINI_API_KEY is configured.
// Keeping the key in an environment variable prevents it from being committed.
type GeminiAnalyzer struct {
	keys   []string
	model  string
	client *http.Client
	next   atomic.Uint64
}

func NewGeminiAnalyzerFromEnv() *GeminiAnalyzer {
	keys := geminiAPIKeys()
	if len(keys) == 0 {
		return nil
	}
	model := strings.TrimSpace(os.Getenv("GEMINI_MODEL"))
	if model == "" {
		model = defaultGeminiModel
	}
	return &GeminiAnalyzer{
		keys:   keys,
		model:  model,
		client: &http.Client{Timeout: 45 * time.Second},
	}
}

func geminiAPIKeys() []string {
	keys := make([]string, 0)
	seen := make(map[string]bool)
	for _, key := range append(splitAPIKeys(os.Getenv("GEMINI_API_KEYS")), splitAPIKeys(os.Getenv("GEMINI_API_KEY"))...) {
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return keys
}

func (a *GeminiAnalyzer) Analyze(path string) (AIResult, error) {
	if a == nil || len(a.keys) == 0 {
		return AIResult{}, nil
	}
	keyIndex := a.next.Add(1) - 1
	apiKey := a.keys[keyIndex%uint64(len(a.keys))]

	data, mimeType, err := readImageForCloudAI(path)
	if err != nil {
		return AIResult{}, err
	}
	if len(data) > 20*1024*1024 {
		return AIResult{}, fmt.Errorf("รูปภาพใหญ่เกิน 20 MB สำหรับการวิเคราะห์ AI: %s", filepath.Base(path))
	}

	body := map[string]interface{}{
		"contents": []interface{}{
			map[string]interface{}{
				"parts": []interface{}{
					map[string]string{
						"text": "วิเคราะห์ภาพโดยรวมและตอบเป็น JSON เท่านั้น ห้ามใส่ Markdown โดยใช้รูปแบบ {\"description\":\"คำอธิบายภาษาไทยไม่เกิน 2 ประโยค\",\"tags\":[\"Tag ภาษาไทย 1\",\"Tag ภาษาไทย 2\"]} ให้ Tags เป็นคำสั้น ๆ ที่สำคัญจริง สื่อถึงวัตถุ สถานที่ กิจกรรม หรือบรรยากาศในภาพ จำนวน 2-5 คำ ห้ามสร้างคำซ้ำหรือคำที่มีความหมายใกล้กันหลายคำ",
					},
					map[string]interface{}{
						"inlineData": map[string]string{
							"mimeType": mimeType,
							"data":     base64.StdEncoding.EncodeToString(data),
						},
					},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":      0.1,
			"responseMimeType": "application/json",
			"responseSchema": map[string]interface{}{
				"type":     "OBJECT",
				"required": []string{"description", "tags"},
				"properties": map[string]interface{}{
					"description": map[string]string{"type": "STRING"},
					"tags": map[string]interface{}{
						"type":  "ARRAY",
						"items": map[string]string{"type": "STRING"},
					},
				},
			},
		},
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return AIResult{}, err
	}
	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", a.model)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return AIResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	res, err := a.client.Do(req)
	if err != nil {
		return AIResult{}, err
	}
	defer res.Body.Close()
	var response struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return AIResult{}, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return AIResult{}, NewProviderError("Gemini", res)
	}
	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return AIResult{}, fmt.Errorf("Gemini API ไม่ส่งผลวิเคราะห์กลับมา")
	}

	text := strings.TrimSpace(response.Candidates[0].Content.Parts[0].Text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(strings.TrimSpace(text), "```")
	var result AIResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &result); err != nil {
		return AIResult{}, fmt.Errorf("อ่าน JSON จาก Gemini ไม่ได้: %w", err)
	}
	result.Description = strings.TrimSpace(result.Description)
	result.Tags = cleanTags(result.Tags)
	return result, nil
}

func splitAPIKeys(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})
	keys := make([]string, 0, len(parts))
	seen := make(map[string]bool)
	for _, part := range parts {
		key := strings.TrimSpace(part)
		if key != "" && !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return keys
}

func cleanTags(tags []string) []string {
	result := make([]string, 0, min(5, len(tags)))
	seen := make(map[string]bool)
	for _, tag := range tags {
		tag = strings.Join(strings.Fields(strings.TrimSpace(tag)), " ")
		key := strings.ToLower(tag)
		if tag != "" && !seen[key] {
			seen[key] = true
			result = append(result, tag)
			if len(result) == 5 {
				break
			}
		}
	}
	return result
}
