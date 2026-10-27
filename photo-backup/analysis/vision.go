package analysis

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

const (
	defaultVisionEndpoint = "https://generativelanguage.googleapis.com/v1beta/interactions"
	defaultVisionModel    = "gemini-3.5-flash"
	maxImageDimension     = 1600
	maxTags               = 8
	maxBatchImages        = 5
	maxVisionAttempts     = 4
)

type Result struct {
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type ImageInput struct {
	ID   uint
	Path string
}

type VisionClient struct {
	apiKeys  []string
	nextKey  atomic.Uint64
	model    string
	endpoint string
	client   *http.Client
}

func NewVisionClientFromEnv() *VisionClient {
	apiKeys := splitAPIKeys(os.Getenv("GEMINI_API_KEYS"))
	apiKeys = appendUniqueAPIKeys(apiKeys, splitAPIKeys(os.Getenv("GEMINI_API_KEY"))...)
	if len(apiKeys) == 0 {
		return nil
	}
	model := strings.TrimSpace(os.Getenv("PHOTO_BACKUP_AI_MODEL"))
	if model == "" {
		model = defaultVisionModel
	}
	endpoint := strings.TrimSpace(os.Getenv("GEMINI_API_ENDPOINT"))
	if endpoint == "" {
		endpoint = defaultVisionEndpoint
	}
	return NewVisionClientWithKeys(apiKeys, model, endpoint, &http.Client{Timeout: 5 * time.Minute})
}

func NewVisionClient(apiKey, model, endpoint string, client *http.Client) *VisionClient {
	return NewVisionClientWithKeys(splitAPIKeys(apiKey), model, endpoint, client)
}

func NewVisionClientWithKeys(apiKeys []string, model, endpoint string, client *http.Client) *VisionClient {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	return &VisionClient{
		apiKeys:  appendUniqueAPIKeys(nil, apiKeys...),
		model:    strings.TrimSpace(model),
		endpoint: strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		client:   client,
	}
}

func (c *VisionClient) AnalyzeImage(ctx context.Context, path string) (Result, error) {
	if c == nil || len(c.apiKeys) == 0 {
		return Result{}, fmt.Errorf("ยังไม่ได้ตั้งค่า GEMINI_API_KEY")
	}
	if c.endpoint == "" || c.model == "" {
		return Result{}, fmt.Errorf("การตั้งค่า Gemini AI ไม่ครบถ้วน")
	}

	mimeType, encodedImage, err := imageData(path)
	if err != nil {
		return Result{}, fmt.Errorf("เตรียมรูปส่งให้ Gemini ไม่สำเร็จ: %w", err)
	}

	payload := map[string]interface{}{
		"model": c.model,
		"input": []interface{}{
			map[string]string{
				"type":      "image",
				"mime_type": mimeType,
				"data":      encodedImage,
			},
			map[string]string{
				"type": "text",
				"text": "วิเคราะห์รูปภาพนี้เพื่อจัดหมวดหมู่ในระบบ PhotoBackup " +
					"ตอบเป็นภาษาไทยตาม JSON schema เท่านั้น " +
					"เขียน description สั้น ๆ 1 ประโยค และสร้าง Tag ที่สื่อถึงวัตถุ สถานที่ กิจกรรม หรือบรรยากาศ " +
					"จำนวน 3 ถึง 8 Tag ห้ามสร้าง Tag ที่เป็นสีอย่างเดียว และถ้าไม่แน่ใจให้เลือก Tag ที่เห็นได้ชัดเท่านั้น",
			},
		},
		"response_format": []interface{}{
			map[string]interface{}{
				"type":      "text",
				"mime_type": "application/json",
				"schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"description": map[string]string{"type": "string"},
						"tags": map[string]interface{}{
							"type":  "array",
							"items": map[string]string{"type": "string"},
						},
					},
					"required": []string{"description", "tags"},
				},
			},
		},
		"generation_config": map[string]interface{}{
			"thinking_level": "low",
		},
	}
	outputText, err := c.sendPayload(ctx, payload)
	if err != nil {
		return Result{}, err
	}
	result, err := parseResult(outputText)
	if err != nil {
		return Result{}, err
	}
	return normalizeResult(result)
}

func (c *VisionClient) AnalyzeImages(ctx context.Context, images []ImageInput) (map[uint]Result, error) {
	if c == nil || len(c.apiKeys) == 0 {
		return nil, fmt.Errorf("ยังไม่ได้ตั้งค่า GEMINI_API_KEY")
	}
	if c.endpoint == "" || c.model == "" {
		return nil, fmt.Errorf("การตั้งค่า Gemini AI ไม่ครบถ้วน")
	}
	if len(images) == 0 {
		return map[uint]Result{}, nil
	}
	if len(images) > maxBatchImages {
		return nil, fmt.Errorf("ส่งรูปต่อชุดได้ไม่เกิน %d รูป", maxBatchImages)
	}

	input := []interface{}{
		map[string]string{
			"type": "text",
			"text": "วิเคราะห์ภาพทั้งหมดแยกกันเพื่อจัดหมวดหมู่ในระบบ PhotoBackup " +
				"ตอบเป็นภาษาไทยตาม JSON schema เท่านั้น " +
				"ต้องส่งผลลัพธ์ให้ครบทุก image_id และห้ามสลับ image_id " +
				"เขียน description สั้น ๆ 1 ประโยค และสร้าง Tag ที่สื่อถึงวัตถุ สถานที่ กิจกรรม หรือบรรยากาศ " +
				"จำนวน 3 ถึง 8 Tag ห้ามสร้าง Tag ที่เป็นสีอย่างเดียว และถ้าไม่แน่ใจให้เลือก Tag ที่เห็นได้ชัดเท่านั้น",
		},
	}
	for _, image := range images {
		if image.ID == 0 {
			return nil, fmt.Errorf("พบรูปที่ไม่มี ID")
		}
		mimeType, encodedImage, err := imageData(image.Path)
		if err != nil {
			return nil, fmt.Errorf("เตรียมรูป ID %d ส่งให้ Gemini ไม่สำเร็จ: %w", image.ID, err)
		}
		input = append(input,
			map[string]string{
				"type": "text",
				"text": fmt.Sprintf("image_id=photo-%d", image.ID),
			},
			map[string]string{
				"type":      "image",
				"mime_type": mimeType,
				"data":      encodedImage,
			},
		)
	}

	payload := map[string]interface{}{
		"model": c.model,
		"input": input,
		"response_format": []interface{}{
			map[string]interface{}{
				"type":      "text",
				"mime_type": "application/json",
				"schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"results": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"image_id":    map[string]string{"type": "string"},
									"description": map[string]string{"type": "string"},
									"tags": map[string]interface{}{
										"type":  "array",
										"items": map[string]string{"type": "string"},
									},
								},
								"required": []string{"image_id", "description", "tags"},
							},
						},
					},
					"required": []string{"results"},
				},
			},
		},
		"generation_config": map[string]interface{}{
			"thinking_level": "low",
		},
	}

	outputText, err := c.sendPayload(ctx, payload)
	if err != nil {
		return nil, err
	}
	var decoded struct {
		Results []struct {
			ImageID     string   `json:"image_id"`
			Description string   `json:"description"`
			Tags        []string `json:"tags"`
		} `json:"results"`
	}
	if err := parseJSONText(outputText, &decoded); err != nil {
		return nil, fmt.Errorf("ผลลัพธ์ Gemini แบบชุดไม่ตรง JSON schema: %w", err)
	}

	expected := make(map[string]uint, len(images))
	for _, image := range images {
		expected[fmt.Sprintf("photo-%d", image.ID)] = image.ID
	}
	results := make(map[uint]Result, len(decoded.Results))
	for _, item := range decoded.Results {
		photoID, ok := expected[strings.TrimSpace(item.ImageID)]
		if !ok {
			return nil, fmt.Errorf("Gemini ส่ง image_id ที่ไม่รู้จัก: %s", item.ImageID)
		}
		if _, exists := results[photoID]; exists {
			return nil, fmt.Errorf("Gemini ส่งผลซ้ำสำหรับ image_id: %s", item.ImageID)
		}
		result, err := normalizeResult(Result{Description: item.Description, Tags: item.Tags})
		if err != nil {
			return nil, fmt.Errorf("ผลลัพธ์ของ image_id %s ไม่สมบูรณ์: %w", item.ImageID, err)
		}
		results[photoID] = result
	}
	if len(results) != len(images) {
		return nil, fmt.Errorf("Gemini ส่งผลลัพธ์ไม่ครบ: ต้องการ %d รูป แต่ได้ %d รูป", len(images), len(results))
	}
	return results, nil
}

func (c *VisionClient) sendPayload(ctx context.Context, payload interface{}) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("สร้างคำขอ Gemini ไม่สำเร็จ: %w", err)
	}
	responseBody, err := c.doRequest(ctx, body)
	if err != nil {
		return "", err
	}

	var decoded struct {
		OutputText string `json:"output_text"`
		Steps      []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return "", fmt.Errorf("อ่าน JSON จาก Gemini ไม่สำเร็จ: %w", err)
	}
	outputText := strings.TrimSpace(decoded.OutputText)
	if outputText == "" {
		for _, step := range decoded.Steps {
			if step.Type != "model_output" {
				continue
			}
			for _, content := range step.Content {
				if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
					outputText = strings.TrimSpace(content.Text)
					break
				}
			}
			if outputText != "" {
				break
			}
		}
	}
	if outputText == "" {
		return "", fmt.Errorf("Gemini ไม่ส่งผลลัพธ์กลับมา")
	}
	return outputText, nil
}

func (c *VisionClient) doRequest(ctx context.Context, body []byte) ([]byte, error) {
	var response *http.Response
	var err error
	for attempt := 0; attempt < maxVisionAttempts; attempt++ {
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
		if requestErr != nil {
			return nil, requestErr
		}
		request.Header.Set("x-goog-api-key", c.nextAPIKey())
		request.Header.Set("Content-Type", "application/json")

		response, err = c.client.Do(request)
		if err == nil && !retryableVisionStatus(response.StatusCode) {
			break
		}
		if response != nil {
			response.Body.Close()
		}
		if attempt == maxVisionAttempts-1 {
			if err != nil {
				return nil, fmt.Errorf("เชื่อมต่อ Gemini ไม่สำเร็จ: %w", err)
			}
			break
		}

		wait := time.Duration(1<<attempt) * time.Second
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
	if response == nil {
		return nil, fmt.Errorf("Gemini ไม่ส่งผลลัพธ์กลับมา")
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("อ่านผลลัพธ์ Gemini ไม่สำเร็จ: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Gemini ตอบกลับ HTTP %d: %s", response.StatusCode, compactError(responseBody))
	}
	return responseBody, nil
}

func (c *VisionClient) nextAPIKey() string {
	if c == nil || len(c.apiKeys) == 0 {
		return ""
	}
	index := c.nextKey.Add(1) - 1
	return c.apiKeys[index%uint64(len(c.apiKeys))]
}

func retryableVisionStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusInternalServerError,
		http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func parseResult(outputText string) (Result, error) {
	var result Result
	if err := parseJSONText(outputText, &result); err == nil {
		return result, nil
	}
	return Result{}, fmt.Errorf("ผลลัพธ์ Gemini ไม่ตรง JSON schema: %s", compactError([]byte(strings.TrimSpace(outputText))))
}

func parseJSONText(outputText string, target interface{}) error {
	text := strings.TrimSpace(outputText)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)
	if err := json.Unmarshal([]byte(text), target); err == nil {
		return nil
	}

	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(text[start:end+1]), target); err == nil {
			return nil
		}
	}
	return fmt.Errorf("JSON ไม่ถูกต้อง")
}

func normalizeResult(result Result) (Result, error) {
	result.Description = strings.TrimSpace(result.Description)
	result.Tags = normalizeTags(result.Tags)
	if result.Description == "" {
		return Result{}, fmt.Errorf("Gemini ไม่ส่งคำอธิบายรูปภาพ")
	}
	return result, nil
}

func normalizeTags(tags []string) []string {
	result := make([]string, 0, min(len(tags), maxTags))
	seen := make(map[string]struct{})
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.TrimPrefix(tag, "#"))
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, tag)
		if len(result) == maxTags {
			break
		}
	}
	return result
}

func splitAPIKeys(raw string) []string {
	return appendUniqueAPIKeys(nil, strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})...)
}

func appendUniqueAPIKeys(keys []string, values ...string) []string {
	seen := make(map[string]struct{}, len(keys)+len(values))
	result := make([]string, 0, len(keys)+len(values))
	for _, key := range append(keys, values...) {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}

func imageData(path string) (string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", "", err
	}
	mimeType := "image/jpeg"
	if format == "png" {
		mimeType = "image/png"
	} else if format == "gif" {
		mimeType = "image/gif"
	}
	if max(config.Width, config.Height) <= maxImageDimension {
		return mimeType, base64.StdEncoding.EncodeToString(data), nil
	}

	picture, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", "", err
	}
	scaled := resizeImage(picture, maxImageDimension)
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, scaled, &jpeg.Options{Quality: 80}); err != nil {
		return "", "", err
	}
	return "image/jpeg", base64.StdEncoding.EncodeToString(encoded.Bytes()), nil
}

func resizeImage(source image.Image, maxDimension int) *image.RGBA {
	width, height := source.Bounds().Dx(), source.Bounds().Dy()
	scale := float64(maxDimension) / float64(max(width, height))
	newWidth, newHeight := max(1, int(float64(width)*scale)), max(1, int(float64(height)*scale))
	destination := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			sourceX := source.Bounds().Min.X + x*width/newWidth
			sourceY := source.Bounds().Min.Y + y*height/newHeight
			destination.Set(x, y, source.At(sourceX, sourceY))
		}
	}
	return destination
}

func compactError(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 500 {
		return text[:500] + "..."
	}
	return text
}
