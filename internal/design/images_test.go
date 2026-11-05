package design

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gamidoc/backend/internal/ai"
	"github.com/gamidoc/backend/internal/storage/objectstore"
)

func samplePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for x := 0; x < 40; x++ {
		img.Set(x, 10, color.RGBA{R: 200, A: 255})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func newImageReports(t *testing.T) (*ReportService, *objectstore.LocalStore) {
	t.Helper()
	store := objectstore.NewLocalStore(t.TempDir(), "/files/pdfs")
	return NewReportService(ai.NewNoopAssistant(), NewReportBuilder(), store, nil), store
}

func TestSaveImageStoresPNGUnderTheOwner(t *testing.T) {
	reports, store := newImageReports(t)

	url, err := reports.SaveImage(context.Background(), "sessions", "s-1", samplePNG(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(url, "/files/pdfs/design/sessions/s-1/images/") || !strings.HasSuffix(url, ".png") {
		t.Fatalf("unexpected url %q", url)
	}
	key, _ := store.KeyFromURL(url)
	if _, err := store.Read(context.Background(), key); err != nil {
		t.Fatalf("image not stored: %v", err)
	}
}

func TestSaveImageRejectsOtherTypes(t *testing.T) {
	reports, _ := newImageReports(t)

	_, err := reports.SaveImage(context.Background(), "sessions", "s-1", []byte("%PDF-1.4 not an image"))
	if !errors.Is(err, ErrUnsupportedImage) {
		t.Fatalf("expected ErrUnsupportedImage, got %v", err)
	}
}

func TestReportReplacesImageURLWithTheImage(t *testing.T) {
	reports, _ := newImageReports(t)
	url, err := reports.SaveImage(context.Background(), "sessions", "s-1", samplePNG(t))
	if err != nil {
		t.Fatal(err)
	}

	content, _ := json.Marshal(map[string]string{"Game rules": "one point per task", "Loop diagram": url})
	status := NewInitialStatus()
	status.Sections[SectionKey(4)] = SectionState{Content: content, Visited: true}

	standard := renderStandard(status)
	enhanced, _ := reports.renderEnhanced(context.Background(), status)
	reports.attachImages(context.Background(), status, standard, enhanced)

	section := standard[3]
	if len(section.Images) != 1 || section.Images[0].Label != "Loop diagram" {
		t.Fatalf("expected the loop diagram image, got %+v", section.Images)
	}
	for _, line := range section.Lines {
		if strings.Contains(line, url) {
			t.Fatalf("image URL still printed as text: %q", line)
		}
	}
	if len(section.Lines) != 1 {
		t.Fatalf("expected the other line to stay, got %v", section.Lines)
	}

	pdf, err := NewReportBuilder().Build("Report", ReportVersionStandard, "", standard)
	if err != nil || !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("PDF with image not built: %v", err)
	}
}

func TestIsImageUpload(t *testing.T) {
	cases := map[string]bool{
		"POST /api/v1/sessions/s-1/design/images":  true,
		"POST /api/v1/projects/p-1/design/images/": true,
		"GET /api/v1/sessions/s-1/design/images":   false,
		"POST /api/v1/sessions/s-1/design/section": false,
	}
	for request, want := range cases {
		method, path, _ := strings.Cut(request, " ")
		if got := IsImageUpload(httptest.NewRequest(method, path, nil)); got != want {
			t.Fatalf("%s: expected %v, got %v", request, want, got)
		}
	}
}
