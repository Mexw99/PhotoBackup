package analysis

import (
	"os"
	"testing"
)

// This test is opt-in because loading the local BLIP model is intentionally
// expensive. It never calls a cloud provider.
func TestBLIPAnalyzerRealWorker(t *testing.T) {
	if os.Getenv("PHOTO_BACKUP_BLIP_INTEGRATION") != "1" {
		t.Skip("set PHOTO_BACKUP_BLIP_INTEGRATION=1 to run the local BLIP worker")
	}
	imagePath := os.Getenv("BLIP_TEST_IMAGE")
	if imagePath == "" {
		t.Fatal("BLIP_TEST_IMAGE is required")
	}
	analyzer := NewBLIPAnalyzerFromEnv()
	defer analyzer.Close()
	result, err := analyzer.Analyze(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	if result.Description == "" {
		t.Fatal("BLIP returned an empty description")
	}
}
