package analysis

import "testing"

func TestVisionClientCombinesAndRotatesAPIKeys(t *testing.T) {
	t.Setenv("GEMINI_API_KEYS", "one,two,three,four")
	t.Setenv("GEMINI_API_KEY", "four,five")
	client := NewVisionClientFromEnv()
	if client == nil || len(client.apiKeys) != 5 {
		t.Fatalf("api keys = %v, want five unique keys", client.apiKeys)
	}
	want := []string{"one", "two", "three", "four", "five"}
	for _, key := range want {
		if got := client.nextAPIKey(); got != key {
			t.Fatalf("next key = %q, want %q", got, key)
		}
	}
}
