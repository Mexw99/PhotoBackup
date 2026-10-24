package analysis

import (
	"os"
	"testing"
)

// Run manually with PHOTO_BACKUP_YOLO_INTEGRATION=1 and
// YOLO_TEST_IMAGE pointing at a real image. It never calls a cloud API.
func TestYOLOAnalyzerRealWorker(t *testing.T) {
	if os.Getenv("PHOTO_BACKUP_YOLO_INTEGRATION") != "1" {
		t.Skip("set PHOTO_BACKUP_YOLO_INTEGRATION=1 to run the local YOLO worker")
	}
	imagePath := os.Getenv("YOLO_TEST_IMAGE")
	if imagePath == "" {
		t.Fatal("YOLO_TEST_IMAGE is required")
	}
	loadPortableConfig()
	analyzer := NewYOLOAnalyzerFromEnv()
	defer analyzer.Close()
	result, err := analyzer.Analyze(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tags) == 0 {
		t.Fatalf("YOLO returned no tags for %s", imagePath)
	}
}
