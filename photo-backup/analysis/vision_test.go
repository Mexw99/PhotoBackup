package analysis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVisionClientReturnsStructuredPhotoAnalysis(t *testing.T) {
	var requestBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("x-goog-api-key") != "test-key" {
			t.Fatalf("missing authorization header")
		}
		if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
			t.Fatal(err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte("{\"steps\":[{\"type\":\"model_output\",\"content\":[{\"type\":\"text\",\"text\":\"{\\\"description\\\":\\\"ภาพแมวบนเก้าอี้\\\",\\\"tags\\\":[\\\"แมว\\\",\\\"สัตว์\\\",\\\"แมว\\\",\\\"#บ้าน\\\"]}\"}]}]}"))
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "sample.png")
	if err := os.WriteFile(path, []byte{
		137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 0,
	}, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, tinyPNG(), 0600); err != nil {
		t.Fatal(err)
	}

	client := NewVisionClient("test-key", "test-model", server.URL, &http.Client{Timeout: time.Second})
	result, err := client.AnalyzeImage(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if result.Description != "ภาพแมวบนเก้าอี้" || len(result.Tags) != 3 || result.Tags[0] != "แมว" || result.Tags[2] != "บ้าน" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if requestBody["model"] != "test-model" {
		t.Fatalf("unexpected model: %#v", requestBody["model"])
	}
	input, ok := requestBody["input"].([]interface{})
	if !ok || len(input) != 2 {
		t.Fatalf("unexpected input: %#v", requestBody["input"])
	}
}

func TestVisionClientRetriesTransientResponse(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		attempts++
		if attempts == 1 {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte("{\"steps\":[{\"type\":\"model_output\",\"content\":[{\"type\":\"text\",\"text\":\"{\\\"description\\\":\\\"รูปทดสอบ\\\",\\\"tags\\\":[\\\"ทดสอบ\\\"]}\"}]}]}"))
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "sample.png")
	if err := os.WriteFile(path, tinyPNG(), 0600); err != nil {
		t.Fatal(err)
	}

	client := NewVisionClient("test-key", "test-model", server.URL, &http.Client{Timeout: 5 * time.Second})
	retries := 0
	ctx := WithRetryNotifier(context.Background(), func() { retries++ })
	result, err := client.AnalyzeImage(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || result.Description != "รูปทดสอบ" || len(result.Tags) != 1 {
		t.Fatalf("unexpected retry result: attempts=%d result=%+v", attempts, result)
	}
	if retries != 1 {
		t.Fatalf("retry notifications = %d, want 1", retries)
	}
}

func TestVisionClientReturnsBatchResultsByImageID(t *testing.T) {
	var requestBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
			t.Fatal(err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte("{\"steps\":[{\"type\":\"model_output\",\"content\":[{\"type\":\"text\",\"text\":\"{\\\"results\\\":[{\\\"image_id\\\":\\\"photo-11\\\",\\\"description\\\":\\\"รูปสิบเอ็ด\\\",\\\"tags\\\":[\\\"หนึ่ง\\\"]},{\\\"image_id\\\":\\\"photo-12\\\",\\\"description\\\":\\\"รูปสิบสอง\\\",\\\"tags\\\":[\\\"สอง\\\"]}]}\"}]}]}"))
	}))
	defer server.Close()

	firstPath := filepath.Join(t.TempDir(), "first.png")
	secondPath := filepath.Join(t.TempDir(), "second.png")
	if err := os.WriteFile(firstPath, tinyPNG(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, tinyPNG(), 0600); err != nil {
		t.Fatal(err)
	}

	client := NewVisionClient("test-key", "test-model", server.URL, &http.Client{Timeout: time.Second})
	results, err := client.AnalyzeImages(context.Background(), []ImageInput{
		{ID: 11, Path: firstPath},
		{ID: 12, Path: secondPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	if results[11].Description != "รูปสิบเอ็ด" || results[12].Description != "รูปสิบสอง" {
		t.Fatalf("unexpected batch results: %+v", results)
	}
	input, ok := requestBody["input"].([]interface{})
	if !ok || len(input) != 5 {
		t.Fatalf("unexpected batch input: %#v", requestBody["input"])
	}
}

func TestVisionClientRejectsInvalidImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.png")
	if err := os.WriteFile(path, []byte("not an image"), 0600); err != nil {
		t.Fatal(err)
	}
	client := NewVisionClient("test-key", "test-model", "http://127.0.0.1:1", &http.Client{Timeout: time.Second})
	if _, err := client.AnalyzeImage(context.Background(), path); err == nil {
		t.Fatal("expected invalid image error")
	}
}

func tinyPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0xf0,
		0x1f, 0x00, 0x05, 0x00, 0x01, 0xff, 0x89, 0x99,
		0x3d, 0x1d, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
		0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
}
