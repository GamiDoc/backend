package design

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gamidoc/backend/internal/http/response"
	"github.com/google/uuid"
)

// MaxImageBytes is the largest image accepted by the upload endpoint.
const MaxImageBytes = 5 << 20

var ErrUnsupportedImage = errors.New("unsupported image type")

// imageExtensions maps the sniffed content type to the stored file extension.
var imageExtensions = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
}

// IsImageUpload reports whether the request targets the design image upload endpoint.
// The global body limit skips it: the handler applies MaxImageBytes itself.
func IsImageUpload(r *http.Request) bool {
	return r.Method == http.MethodPost && strings.HasSuffix(strings.TrimRight(r.URL.Path, "/"), "/design/images")
}

// SaveImage stores a PNG or JPEG under design/<kind>/<id>/images/ and returns its public URL.
func (r *ReportService) SaveImage(ctx context.Context, kind string, id string, data []byte) (string, error) {
	ext, ok := imageExtensions[http.DetectContentType(data)]
	if !ok {
		return "", ErrUnsupportedImage
	}
	key := "design/" + kind + "/" + id + "/images/" + uuid.NewString() + ext
	return r.store.Save(ctx, key, data)
}

// imageKey returns the storage key of an uploaded design image, if url points to one.
func (r *ReportService) imageKey(url string) (string, bool) {
	key, ok := r.store.KeyFromURL(url)
	if !ok || !strings.HasPrefix(key, "design/") || !strings.Contains(key, "/images/") {
		return "", false
	}
	return key, true
}

func (h *Handler) uploadImage(w http.ResponseWriter, r *http.Request, o owner) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxImageBytes+1<<20) // room for the multipart envelope
	file, _, err := r.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			response.WriteError(w, http.StatusRequestEntityTooLarge, "IMAGE_TOO_LARGE", "Image larger than 5 MB", nil)
			return
		}
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Send the image as multipart field \"file\"", nil)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, MaxImageBytes+1))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid image", nil)
		return
	}
	if len(data) > MaxImageBytes {
		response.WriteError(w, http.StatusRequestEntityTooLarge, "IMAGE_TOO_LARGE", "Image larger than 5 MB", nil)
		return
	}

	url, err := h.reports.SaveImage(r.Context(), o.kind, o.id, data)
	if err != nil {
		if errors.Is(err, ErrUnsupportedImage) {
			response.WriteError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_IMAGE", "Only PNG and JPEG images are accepted", nil)
			return
		}
		h.documents.logger.Error("image upload failed", "owner", ownerKey(o), "error", err)
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}

	response.WriteJSON(w, http.StatusCreated, map[string]string{"url": url})
}

// sectionImages loads the uploaded images referenced by a section's content
// (string values that are URLs of design images), in content order.
func (r *ReportService) sectionImages(ctx context.Context, content json.RawMessage) ([]RenderImage, map[string]bool) {
	var values map[string]json.RawMessage
	if len(content) == 0 || json.Unmarshal(content, &values) != nil {
		return nil, nil
	}

	var images []RenderImage
	keys := map[string]bool{}
	for _, label := range orderedKeys(content) {
		var url string
		if json.Unmarshal(values[label], &url) != nil {
			continue
		}
		key, ok := r.imageKey(url)
		if !ok {
			continue
		}
		data, err := r.store.Read(ctx, key)
		if err != nil {
			continue
		}
		imageType := "PNG"
		if strings.HasSuffix(key, ".jpg") {
			imageType = "JPG"
		}
		images = append(images, RenderImage{Label: label, Data: data, Type: imageType, Name: key})
		keys[label] = true
	}
	return images, keys
}

// orderedKeys returns the top-level keys of a JSON object in document order.
func orderedKeys(content json.RawMessage) []string {
	decoder := json.NewDecoder(strings.NewReader(string(content)))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return nil
	}
	var keys []string
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return keys
		}
		key, _ := token.(string)
		keys = append(keys, key)
		var skip json.RawMessage
		if err := decoder.Decode(&skip); err != nil {
			return keys
		}
	}
	return keys
}
