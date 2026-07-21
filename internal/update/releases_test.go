package update_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/maxBRT/ship-cli/internal/update"
)

func TestReleases_alreadyCurrentLeavesBinaryIntact(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "ship")
	original := []byte("original-binary-contents")
	if err := os.WriteFile(exe, original, 0o755); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/maxBRT/ship-cli/releases/latest" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"tag_name":"v1.2.3","assets":[]}`)
	}))
	t.Cleanup(srv.Close)

	up := &update.Releases{
		Owner:      "maxBRT",
		Repo:       "ship-cli",
		Version:    "v1.2.3",
		Executable: exe,
		APIBase:    srv.URL,
		Client:     srv.Client(),
	}
	res, err := up.Update(context.Background())
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !res.AlreadyCurrent {
		t.Fatalf("AlreadyCurrent = false, want true; %+v", res)
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Errorf("binary changed; want intact original")
	}
}

func TestReleases_newerReleaseReplacesBinary(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	if goos != "linux" && goos != "darwin" && goos != "windows" {
		t.Skip("unsupported GOOS for this test")
	}
	if goarch != "amd64" && goarch != "arm64" {
		t.Skip("unsupported GOARCH for this test")
	}

	exe := filepath.Join(t.TempDir(), "ship")
	if err := os.WriteFile(exe, []byte("old-ship"), 0o755); err != nil {
		t.Fatal(err)
	}

	newBinary := []byte("new-ship-binary-v2")
	archiveBytes := makeTarGz(t, "ship", newBinary)
	ver := "2.0.0"
	asset := archiveName(ver, goos, goarch)
	sums := checksumLine(sha256Sum(archiveBytes), asset)

	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/maxBRT/ship-cli/releases/latest":
			fmt.Fprintf(w, `{
				"tag_name":"v%s",
				"assets":[
					{"name":%q,"browser_download_url":%q},
					{"name":%q,"browser_download_url":%q}
				]
			}`, ver, asset, srvURL+"/download/"+asset,
				fmt.Sprintf("ship-cli_%s_checksums.txt", ver), srvURL+"/download/checksums.txt")
		case r.URL.Path == "/download/"+asset:
			_, _ = w.Write(archiveBytes)
		case r.URL.Path == "/download/checksums.txt":
			_, _ = io.WriteString(w, sums)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	srvURL = srv.URL

	up := &update.Releases{
		Owner:      "maxBRT",
		Repo:       "ship-cli",
		Version:    "v1.0.0",
		Executable: exe,
		GOOS:       goos,
		GOARCH:     goarch,
		APIBase:    srv.URL,
		Client:     srv.Client(),
	}
	res, err := up.Update(context.Background())
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if res.AlreadyCurrent {
		t.Fatal("AlreadyCurrent = true, want false")
	}
	if res.OldVersion != "v1.0.0" || res.NewVersion != "v"+ver {
		t.Errorf("versions = %+v, want old=v1.0.0 new=v%s", res, ver)
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, newBinary) {
		t.Errorf("binary = %q, want %q", got, newBinary)
	}
}

func TestReleases_checksumMismatchLeavesBinaryIntact(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	exe := filepath.Join(t.TempDir(), "ship")
	original := []byte("old-ship")
	if err := os.WriteFile(exe, original, 0o755); err != nil {
		t.Fatal(err)
	}

	archiveBytes := makeTarGz(t, "ship", []byte("new-ship"))
	ver := "2.0.0"
	asset := archiveName(ver, goos, goarch)
	badSums := checksumLine(sha256Sum([]byte("not-the-archive")), asset)

	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/maxBRT/ship-cli/releases/latest":
			fmt.Fprintf(w, `{
				"tag_name":"v%s",
				"assets":[
					{"name":%q,"browser_download_url":%q},
					{"name":%q,"browser_download_url":%q}
				]
			}`, ver, asset, srvURL+"/download/"+asset,
				fmt.Sprintf("ship-cli_%s_checksums.txt", ver), srvURL+"/download/checksums.txt")
		case r.URL.Path == "/download/"+asset:
			_, _ = w.Write(archiveBytes)
		case r.URL.Path == "/download/checksums.txt":
			_, _ = io.WriteString(w, badSums)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	srvURL = srv.URL

	up := &update.Releases{
		Owner: "maxBRT", Repo: "ship-cli", Version: "v1.0.0",
		Executable: exe, GOOS: goos, GOARCH: goarch,
		APIBase: srv.URL, Client: srv.Client(),
	}
	_, err := up.Update(context.Background())
	if err == nil {
		t.Fatal("Update error = nil, want checksum failure")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("error should mention checksum; got %v", err)
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("binary changed after verify failure; want intact")
	}
}

func TestReleases_unsupportedPlatformError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v2.0.0","assets":[]}`)
	}))
	t.Cleanup(srv.Close)

	exe := filepath.Join(t.TempDir(), "ship")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	up := &update.Releases{
		Owner: "maxBRT", Repo: "ship-cli", Version: "v1.0.0",
		Executable: exe, GOOS: "plan9", GOARCH: "amd64",
		APIBase: srv.URL, Client: srv.Client(),
	}
	_, err := up.Update(context.Background())
	if err == nil {
		t.Fatal("Update error = nil, want unsupported platform")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("error should mention unsupported; got %v", err)
	}
}

func TestReleases_nonHTTPSDownloadURLRejected(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	if !supportedTestPlatform(goos, goarch) {
		t.Skip("unsupported GOOS/GOARCH for this test")
	}
	exe := filepath.Join(t.TempDir(), "ship")
	original := []byte("old-ship")
	if err := os.WriteFile(exe, original, 0o755); err != nil {
		t.Fatal(err)
	}

	ver := "2.0.0"
	asset := archiveName(ver, goos, goarch)
	sumsName := fmt.Sprintf("ship-cli_%s_checksums.txt", ver)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/maxBRT/ship-cli/releases/latest" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, `{
			"tag_name":"v%s",
			"assets":[
				{"name":%q,"browser_download_url":"http://example.com/download/%s"},
				{"name":%q,"browser_download_url":"http://example.com/download/checksums.txt"}
			]
		}`, ver, asset, asset, sumsName)
	}))
	t.Cleanup(srv.Close)

	up := &update.Releases{
		Owner: "maxBRT", Repo: "ship-cli", Version: "v1.0.0",
		Executable: exe, GOOS: goos, GOARCH: goarch,
		APIBase: srv.URL, Client: srv.Client(),
	}
	_, err := up.Update(context.Background())
	if err == nil {
		t.Fatal("Update error = nil, want HTTPS failure")
	}
	if !strings.Contains(err.Error(), "HTTPS") {
		t.Errorf("error should mention HTTPS; got %v", err)
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("binary changed after HTTPS failure; want intact")
	}
}

func TestReleases_downloadRateLimitReadable(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	if !supportedTestPlatform(goos, goarch) {
		t.Skip("unsupported GOOS/GOARCH for this test")
	}
	exe := filepath.Join(t.TempDir(), "ship")
	original := []byte("old-ship")
	if err := os.WriteFile(exe, original, 0o755); err != nil {
		t.Fatal(err)
	}

	ver := "2.0.0"
	asset := archiveName(ver, goos, goarch)
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/maxBRT/ship-cli/releases/latest":
			fmt.Fprintf(w, `{
				"tag_name":"v%s",
				"assets":[
					{"name":%q,"browser_download_url":%q},
					{"name":%q,"browser_download_url":%q}
				]
			}`, ver, asset, srvURL+"/download/"+asset,
				fmt.Sprintf("ship-cli_%s_checksums.txt", ver), srvURL+"/download/checksums.txt")
		case strings.HasPrefix(r.URL.Path, "/download/"):
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, "rate limited")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	srvURL = srv.URL

	up := &update.Releases{
		Owner: "maxBRT", Repo: "ship-cli", Version: "v1.0.0",
		Executable: exe, GOOS: goos, GOARCH: goarch,
		APIBase: srv.URL, Client: srv.Client(),
	}
	_, err := up.Update(context.Background())
	if err == nil {
		t.Fatal("Update error = nil, want rate-limit failure")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("error should mention rate limit; got %v", err)
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("binary changed after rate-limit failure; want intact")
	}
}

func TestReleases_connectivityFailureClearError(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "ship")
	original := []byte("old-ship")
	if err := os.WriteFile(exe, original, 0o755); err != nil {
		t.Fatal(err)
	}

	up := &update.Releases{
		Owner:      "maxBRT",
		Repo:       "ship-cli",
		Version:    "v1.0.0",
		Executable: exe,
		APIBase:    "http://127.0.0.1:1", // nothing listening
		Client:     &http.Client{},
	}
	_, err := up.Update(context.Background())
	if err == nil {
		t.Fatal("Update error = nil, want connectivity failure")
	}
	if !strings.Contains(err.Error(), "connectivity") {
		t.Errorf("error should mention connectivity; got %v", err)
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("binary changed after connectivity failure; want intact")
	}
}

func supportedTestPlatform(goos, goarch string) bool {
	switch goos {
	case "linux", "darwin", "windows":
	default:
		return false
	}
	switch goarch {
	case "amd64", "arm64":
		return true
	default:
		return false
	}
}

func TestReleases_nonWritableBinaryRefused(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	dir := t.TempDir()
	exe := filepath.Join(dir, "ship")
	original := []byte("old-ship")
	if err := os.WriteFile(exe, original, 0o555); err != nil {
		t.Fatal(err)
	}

	newBinary := []byte("new-ship-binary-v2")
	archiveBytes := makeTarGz(t, "ship", newBinary)
	ver := "2.0.0"
	asset := archiveName(ver, goos, goarch)
	sums := checksumLine(sha256Sum(archiveBytes), asset)

	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/maxBRT/ship-cli/releases/latest":
			fmt.Fprintf(w, `{
				"tag_name":"v%s",
				"assets":[
					{"name":%q,"browser_download_url":%q},
					{"name":%q,"browser_download_url":%q}
				]
			}`, ver, asset, srvURL+"/download/"+asset,
				fmt.Sprintf("ship-cli_%s_checksums.txt", ver), srvURL+"/download/checksums.txt")
		case r.URL.Path == "/download/"+asset:
			_, _ = w.Write(archiveBytes)
		case r.URL.Path == "/download/checksums.txt":
			_, _ = io.WriteString(w, sums)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	srvURL = srv.URL

	up := &update.Releases{
		Owner: "maxBRT", Repo: "ship-cli", Version: "v1.0.0",
		Executable: exe, GOOS: goos, GOARCH: goarch,
		APIBase: srv.URL, Client: srv.Client(),
	}
	_, err := up.Update(context.Background())
	if err == nil {
		t.Fatal("Update error = nil, want permission failure")
	}
	if !strings.Contains(err.Error(), "not writable") {
		t.Errorf("error should mention not writable; got %v", err)
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("binary changed after permission failure; want intact")
	}
}

func makeTarGz(t *testing.T, name string, contents []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(contents))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func archiveName(version, goos, goarch string) string {
	return fmt.Sprintf("ship-cli_%s_%s_%s.tar.gz", strings.TrimPrefix(version, "v"), goos, goarch)
}

func checksumLine(sum []byte, name string) string {
	return hex.EncodeToString(sum) + "  " + name + "\n"
}

func sha256Sum(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}
