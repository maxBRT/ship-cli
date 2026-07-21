package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Releases is the GitHub Releases Updater. It resolves the latest release,
// downloads and verifies the matching asset, and replaces Executable in place.
type Releases struct {
	Owner      string
	Repo       string
	Version    string // currently installed version
	Executable string // path of the running binary to replace
	GOOS       string // empty → runtime.GOOS
	GOARCH     string // empty → runtime.GOARCH

	// APIBase overrides the GitHub API root (tests). Empty → https://api.github.com.
	APIBase string
	// Client is the HTTP client for API and asset downloads. Nil → http.DefaultClient.
	Client *http.Client
}

var _ Port = (*Releases)(nil)

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func (r *Releases) client() *http.Client {
	if r.Client != nil {
		return r.Client
	}
	return http.DefaultClient
}

func (r *Releases) apiBase() string {
	if r.APIBase != "" {
		return strings.TrimRight(r.APIBase, "/")
	}
	return "https://api.github.com"
}

func (r *Releases) goos() string {
	if r.GOOS != "" {
		return r.GOOS
	}
	return runtime.GOOS
}

func (r *Releases) goarch() string {
	if r.GOARCH != "" {
		return r.GOARCH
	}
	return runtime.GOARCH
}

// Update brings the installation at Executable up to date with the latest release.
func (r *Releases) Update(ctx context.Context) (Result, error) {
	if r.Executable == "" {
		return Result{}, fmt.Errorf("update: executable path is required")
	}
	rel, err := r.fetchLatest(ctx)
	if err != nil {
		return Result{}, err
	}
	old := r.Version
	latest := rel.TagName
	if normVersion(old) == normVersion(latest) {
		return Result{AlreadyCurrent: true, OldVersion: old, NewVersion: latest}, nil
	}

	goos, goarch := r.goos(), r.goarch()
	if err := supportedPlatform(goos, goarch); err != nil {
		return Result{}, err
	}

	ver := normVersion(latest)
	assetName := fmt.Sprintf("ship-cli_%s_%s_%s.tar.gz", ver, goos, goarch)
	sumsName := fmt.Sprintf("ship-cli_%s_checksums.txt", ver)

	assetURL, err := findAssetURL(rel.Assets, assetName)
	if err != nil {
		return Result{}, err
	}
	sumsURL, err := findAssetURL(rel.Assets, sumsName)
	if err != nil {
		return Result{}, err
	}

	archive, err := r.download(ctx, assetURL)
	if err != nil {
		return Result{}, fmt.Errorf("update: download %s: %w", assetName, err)
	}
	sumsBody, err := r.download(ctx, sumsURL)
	if err != nil {
		return Result{}, fmt.Errorf("update: download checksums: %w", err)
	}
	if err := verifyChecksum(archive, assetName, sumsBody); err != nil {
		return Result{}, err
	}

	bin, err := extractShipBinary(archive)
	if err != nil {
		return Result{}, err
	}
	if err := replaceBinary(r.Executable, bin); err != nil {
		return Result{}, err
	}
	return Result{OldVersion: old, NewVersion: latest}, nil
}

func supportedPlatform(goos, goarch string) error {
	switch goos {
	case "linux", "darwin", "windows":
	default:
		return fmt.Errorf("update: unsupported platform %s/%s", goos, goarch)
	}
	switch goarch {
	case "amd64", "arm64":
	default:
		return fmt.Errorf("update: unsupported platform %s/%s", goos, goarch)
	}
	return nil
}

func findAssetURL(assets []ghAsset, name string) (string, error) {
	for _, a := range assets {
		if a.Name == name {
			if a.BrowserDownloadURL == "" {
				return "", fmt.Errorf("update: asset %q missing download URL", name)
			}
			return a.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("update: no release asset named %q (unsupported platform or incomplete release)", name)
}

func normVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

func (r *Releases) fetchLatest(ctx context.Context) (ghRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", r.apiBase(), r.Owner, r.Repo)
	body, status, err := r.get(ctx, url, "application/vnd.github+json")
	if err != nil {
		return ghRelease{}, fmt.Errorf("update: fetch latest release: %w", err)
	}
	if status == http.StatusForbidden || status == http.StatusTooManyRequests {
		return ghRelease{}, fmt.Errorf("update: GitHub rate limit or forbidden (%s)", http.StatusText(status))
	}
	if status != http.StatusOK {
		return ghRelease{}, fmt.Errorf("update: latest release: %s", http.StatusText(status))
	}
	var rel ghRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return ghRelease{}, fmt.Errorf("update: parse latest release: %w", err)
	}
	if rel.TagName == "" {
		return ghRelease{}, fmt.Errorf("update: latest release missing tag_name")
	}
	return rel, nil
}

func (r *Releases) download(ctx context.Context, url string) ([]byte, error) {
	body, status, err := r.get(ctx, url, "application/octet-stream")
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("%s", http.StatusText(status))
	}
	return body, nil
}

func (r *Releases) get(ctx context.Context, url, accept string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := r.client().Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func verifyChecksum(archive []byte, assetName string, sumsBody []byte) error {
	want, err := checksumFor(assetName, sumsBody)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(archive)
	got := hex.EncodeToString(sum[:])
	if got != want {
		return fmt.Errorf("update: checksum mismatch for %s", assetName)
	}
	return nil
}

func checksumFor(assetName string, sumsBody []byte) (string, error) {
	for _, line := range strings.Split(string(sumsBody), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// GoReleaser: "<hex>  <filename>"
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[len(fields)-1] == assetName {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("update: checksums file has no entry for %s", assetName)
}

func extractShipBinary(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("update: gunzip archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("update: read archive: %w", err)
		}
		base := filepath.Base(hdr.Name)
		if base != "ship" && base != "ship.exe" {
			continue
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("update: extract ship: %w", err)
		}
		return data, nil
	}
	return nil, fmt.Errorf("update: archive does not contain ship binary")
}

func replaceBinary(dest string, bin []byte) error {
	info, err := os.Stat(dest)
	if err != nil {
		return fmt.Errorf("update: stat %s: %w", dest, err)
	}
	dir := filepath.Dir(dest)
	if err := checkWritable(dir, dest, info.Mode()); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".ship-update-*")
	if err != nil {
		return fmt.Errorf("update: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(bin); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("update: write temp binary: %w", err)
	}
	if err := tmp.Chmod(info.Mode()); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("update: chmod temp binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("update: close temp binary: %w", err)
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return fmt.Errorf("update: replace binary: %w", err)
	}
	return nil
}

func checkWritable(dir, dest string, mode os.FileMode) error {
	f, err := os.CreateTemp(dir, ".ship-write-probe-*")
	if err != nil {
		return fmt.Errorf("update: binary directory not writable: %w", err)
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)

	if mode.Perm()&0200 == 0 {
		return fmt.Errorf("update: binary not writable: %s", dest)
	}
	return nil
}
