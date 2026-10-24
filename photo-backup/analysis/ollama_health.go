package analysis

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type OllamaHealthStatus struct {
	State   string `json:"state"`
	Model   string `json:"model"`
	Message string `json:"message"`
}

// HealthCheck ตรวจเฉพาะ Ollama ที่เลือกไว้ ไม่เริ่ม Ollama และไม่ดาวน์โหลด Model
func (a *OllamaAnalyzer) HealthCheck(ctx context.Context) OllamaHealthStatus {
	if a == nil {
		return OllamaHealthStatus{State: "not_running", Message: "ยังไม่ได้เลือก Ollama"}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, a.endpoint+"/api/tags", nil)
	if err != nil {
		return OllamaHealthStatus{State: "not_running", Model: a.model, Message: "สร้าง Health Check ไม่ได้"}
	}
	client := a.client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return OllamaHealthStatus{State: "not_running", Model: a.model, Message: "เชื่อมต่อ Ollama ไม่ได้"}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return OllamaHealthStatus{State: "not_running", Model: a.model, Message: "Ollama ตอบกลับผิดพลาด"}
	}
	var payload struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return OllamaHealthStatus{State: "model_unusable", Model: a.model, Message: "อ่านรายการ Model ไม่ได้"}
	}
	for _, model := range payload.Models {
		if strings.EqualFold(model.Name, a.model) || strings.HasPrefix(strings.ToLower(model.Name), strings.ToLower(a.model+":")) {
			return OllamaHealthStatus{State: "ready", Model: a.model}
		}
	}
	return OllamaHealthStatus{State: "model_missing", Model: a.model, Message: "ยังไม่มี Model ที่เลือกใน Ollama"}
}
