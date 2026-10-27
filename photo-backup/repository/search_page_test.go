package repository_test

import (
	"fmt"
	"testing"
	"time"
)

func TestSearchPhotoPagesWithThousandPhotos(t *testing.T) {
	r := newTestRepository(t)
	var cats, beaches []uint
	for i := 0; i < 1000; i++ {
		p := createTestPhoto(t, r, fmt.Sprintf("IMG_%04d.JPG", i))
		if i%2 == 0 {
			cats = append(cats, p.ID)
		}
		if i%5 == 0 {
			beaches = append(beaches, p.ID)
		}
	}
	if _, err := r.AddTagToPhotos(cats, "Cat"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.AddTagToPhotos(beaches, "ทะเล"); err != nil {
		t.Fatal(err)
	}
	seen := map[uint]bool{}
	start := time.Now()
	for page := 1; page <= 17; page++ {
		result, err := r.SearchPhotoPage("", nil, page, 60)
		if err != nil || result.Total != 1000 || len(result.Photos) > 60 {
			t.Fatalf("page %d: %+v, %v", page, result, err)
		}
		for _, p := range result.Photos {
			if seen[p.ID] {
				t.Fatalf("duplicate page result: %d", p.ID)
			}
			seen[p.ID] = true
		}
	}
	if len(seen) != 1000 {
		t.Fatalf("missing photos: %d", len(seen))
	}
	t.Logf("17 paginated SQLite searches over 1000 records: %s", time.Since(start))
	for _, test := range []struct {
		q     string
		tags  []string
		count int64
	}{
		{"img_000", nil, 10}, {"", []string{"cat"}, 500}, {"", []string{"cat", "ทะเล"}, 600},
		{"IMG_000", []string{"CAT"}, 5}, {"%", nil, 0}, {"' OR 1=1 --", nil, 0}, {"missing", nil, 0},
	} {
		result, err := r.SearchPhotoPage(test.q, test.tags, 1, 60)
		if err != nil || result.Total != test.count {
			t.Fatalf("query %q, %v => %d want %d: %v", test.q, test.tags, result.Total, test.count, err)
		}
	}
}
