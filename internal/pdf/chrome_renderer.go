package pdf

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ChromeHTMLRenderer prints HTML to PDF with a local headless Chrome, Chromium or Edge
// (`--print-to-pdf`). It gives the same result as Gotenberg (also Chromium) without a
// separate service, e.g. for local development.
type ChromeHTMLRenderer struct {
	binary  string
	timeout time.Duration
}

func NewChromeHTMLRenderer(binary string, timeout time.Duration) *ChromeHTMLRenderer {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &ChromeHTMLRenderer{binary: binary, timeout: timeout}
}

// FindChrome returns the path of a Chromium-based browser: configured (if it exists),
// else the first one found among the usual install locations and the PATH.
func FindChrome(configured string) (string, bool) {
	if configured = strings.TrimSpace(configured); configured != "" {
		if _, err := os.Stat(configured); err == nil {
			return configured, true
		}
		if path, err := exec.LookPath(configured); err == nil {
			return path, true
		}
		return "", false
	}

	var candidates []string
	switch runtime.GOOS {
	case "windows":
		for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LocalAppData")} {
			if base == "" {
				continue
			}
			candidates = append(candidates,
				filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"),
				filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe"),
			)
		}
	case "darwin":
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
	}
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "microsoft-edge"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, true
		}
	}
	return "", false
}

func (r *ChromeHTMLRenderer) Render(ctx context.Context, document HTMLDocument) ([]byte, error) {
	return r.RenderHTML(ctx, document.HTML)
}

func (r *ChromeHTMLRenderer) RenderHTML(ctx context.Context, html string) ([]byte, error) {
	if r == nil || r.binary == "" {
		return nil, ErrPDFRendererUnavailable
	}

	dir, err := os.MkdirTemp("", "gamidoc-pdf-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	input := filepath.Join(dir, "document.html")
	output := filepath.Join(dir, "document.pdf")
	if err := os.WriteFile(input, []byte(html), 0o600); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	inputURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(input)}).String()
	if runtime.GOOS == "windows" {
		inputURL = "file:///" + filepath.ToSlash(input)
	}
	args := []string{
		"--headless=new",
		"--disable-gpu",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-extensions",
		"--no-pdf-header-footer",
		"--user-data-dir=" + filepath.Join(dir, "profile"),
	}
	if runtime.GOOS == "linux" {
		// In a container (e.g. the Docker image on Render) Chromium runs as root, where its
		// sandbox cannot start, and /dev/shm is tiny: without these flags it crashes.
		args = append(args, "--disable-dev-shm-usage", "--no-zygote")
		if os.Geteuid() == 0 {
			args = append(args, "--no-sandbox")
		}
	}
	args = append(args, "--print-to-pdf="+output, inputURL)
	cmd := exec.CommandContext(ctx, r.binary, args...)
	if detail, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("chrome print-to-pdf failed: %w: %s", err, strings.TrimSpace(string(detail)))
	}

	data, err := os.ReadFile(output)
	if err != nil {
		return nil, fmt.Errorf("chrome produced no pdf: %w", err)
	}
	if len(data) == 0 {
		return nil, errors.New("chrome produced an empty pdf")
	}
	return data, nil
}

// RenderHTML lets the Gotenberg renderer be used with a plain HTML string.
func (r *GotenbergHTMLRenderer) RenderHTML(ctx context.Context, html string) ([]byte, error) {
	return r.Render(ctx, HTMLDocument{HTML: html})
}
