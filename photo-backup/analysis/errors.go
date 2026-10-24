package analysis

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ProviderError keeps retry/diagnostic metadata without retaining credentials
// or echoing an arbitrary provider response body into the database.
type ProviderError struct {
	Provider   string
	StatusCode int
	RetryAfter time.Duration
	Message    string
}

func (e *ProviderError) Error() string {
	if e == nil {
		return "AI provider error"
	}
	if e.Message != "" {
		return fmt.Sprintf("%s: %s", e.Provider, e.Message)
	}
	return fmt.Sprintf("%s API ตอบกลับ HTTP %d", e.Provider, e.StatusCode)
}

func NewProviderError(provider string, response *http.Response) *ProviderError {
	if response == nil {
		return &ProviderError{Provider: provider, Message: "ไม่มีการตอบกลับจาก Provider"}
	}
	return &ProviderError{
		Provider: provider, StatusCode: response.StatusCode,
		RetryAfter: parseRetryAfter(response.Header.Get("Retry-After")),
		Message:    fmt.Sprintf("HTTP %d", response.StatusCode),
	}
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		if delay := time.Until(when); delay > 0 {
			return delay
		}
	}
	return 0
}
