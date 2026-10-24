package analysis

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type OpenAIAnalyzer struct {
	keys   []string
	model  string
	client *http.Client
	next   uint64
}

func NewOpenAIAnalyzerFromEnv() *OpenAIAnalyzer {
	keys := splitAPIKeys(os.Getenv("OPENAI_API_KEYS"))
	if len(keys) == 0 {
		keys = splitAPIKeys(os.Getenv("OPENAI_API_KEY"))
	}
	if len(keys) == 0 {
		return nil
	}
	model := strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	if model == "" {
		model = "gpt-4.1-mini"
	}
	return &OpenAIAnalyzer{
		keys: keys, model: model,
		client: &http.Client{Timeout: 45 * time.Second},
	}
}

func (a *OpenAIAnalyzer) Analyze(path string) (AIResult, error) {
	if a == nil || len(a.keys) == 0 {
		return AIResult{}, nil
	}
	data, mimeType, err := readImageForCloudAI(path)
	if err != nil {
		return AIResult{}, err
	}
	keyIndex := atomicNext(&a.next)
	apiKey := a.keys[keyIndex%uint64(len(a.keys))]
	body := map[string]interface{}{
		"model": a.model,
		"messages": []interface{}{map[string]interface{}{
			"role": "user",
			"content": []interface{}{
				map[string]string{"type": "text", "text": analysisPrompt},
				map[string]interface{}{"type": "image_url", "image_url": map[string]string{
					"url": "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), "detail": "low",
				}},
			},
		}},
		"response_format": map[string]string{"type": "json_object"},
		"max_tokens":      300,
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return AIResult{}, err
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(encoded))
	if err != nil {
		return AIResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := a.client.Do(req)
	if err != nil {
		return AIResult{}, err
	}
	defer res.Body.Close()
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return AIResult{}, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return AIResult{}, NewProviderError("OpenAI", res)
	}
	if len(response.Choices) == 0 {
		return AIResult{}, fmt.Errorf("OpenAI API ไม่ส่งผลวิเคราะห์กลับมา")
	}
	return parseAIResult(response.Choices[0].Message.Content)
}

const analysisPrompt = "วิเคราะห์ภาพโดยรวมและตอบเป็น JSON เท่านั้น ห้ามใส่ Markdown โดยใช้รูปแบบ {\"description\":\"คำอธิบายภาษาไทยไม่เกิน 2 ประโยค\",\"tags\":[\"Tag ภาษาไทย 1\",\"Tag ภาษาไทย 2\"]} ให้ Tags เป็นคำสั้น ๆ ที่สำคัญจริง สื่อถึงวัตถุ สถานที่ กิจกรรม หรือบรรยากาศในภาพ จำนวน 2-5 คำ ห้ามสร้างคำซ้ำหรือคำที่มีความหมายใกล้กันหลายคำ"

func parseAIResult(text string) (AIResult, error) {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(strings.TrimSpace(text), "```")
	text = extractJSONObject(text)
	var result AIResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &result); err != nil {
		return AIResult{}, fmt.Errorf("อ่าน JSON จาก AI ไม่ได้: %w", err)
	}
	result.Description = strings.TrimSpace(result.Description)
	result.Tags = cleanTags(result.Tags)
	return result, nil
}

// extractJSONObject tolerates a model adding a short explanation or Markdown
// fence around the requested JSON. It does not invent an analysis; it only
// isolates an object that the model actually returned.
func extractJSONObject(text string) string {
	start := strings.IndexByte(text, '{')
	end := strings.LastIndexByte(text, '}')
	if start >= 0 && end > start {
		return strings.TrimSpace(text[start : end+1])
	}
	return strings.TrimSpace(text)
}
