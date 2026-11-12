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
	return NewReportService(ai.NewNoopAssistant(), nil, store, nil), store
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

func TestDocumentIncludesUploadedImages(t *testing.T) {
	reports, _ := newImageReports(t)
	url, err := reports.SaveImage(context.Background(), "sessions", "s-1", samplePNG(t))
	if err != nil {
		t.Fatal(err)
	}

	content, _ := json.Marshal(map[string]string{"Game rules": "one point per task", "Loop diagram": url})
	status := NewInitialStatus()
	status.Sections[SectionKey(4)] = SectionState{Content: content, Visited: true}

	images, labels := reports.sectionImages(context.Background(), status.Section(4).Content)
	if len(images) != 1 || images[0].Label != "Loop diagram" || !labels["Loop diagram"] {
		t.Fatalf("expected the loop diagram image, got %+v", images)
	}

	html, err := RenderDocumentHTML(fallbackDocument(ReportFormatGDD, status), ReportFormatGDD, status, "DES-1", map[int][]RenderImage{4: images}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "data:image/png;base64,") {
		t.Fatal("image not embedded in the document")
	}
	if strings.Contains(html, url) {
		t.Fatal("image URL printed as text")
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

func TestMarkMissingFlagsReportsWithoutFile(t *testing.T) {
	reports, _ := newImageReports(t)
	url, err := reports.SaveImage(context.Background(), "sessions", "s-1", samplePNG(t))
	if err != nil {
		t.Fatal(err)
	}

	marked := reports.MarkMissing(context.Background(), []Report{
		{ID: "kept", URL: url},
		{ID: "lost", URL: "/files/pdfs/design/sessions/s-1/lost.pdf"},
	})
	if marked[0].Missing || !marked[1].Missing {
		t.Fatalf("expected only the lost report to be missing, got %+v", marked)
	}
}
