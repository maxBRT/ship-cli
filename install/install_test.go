package install_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallScript_installsShipIntoBinDir(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	if !supportedPlatform(goos, goarch) {
		t.Skip("unsupported GOOS/GOARCH for install script test")
	}

	binDir := t.TempDir()
	binary := []byte("ship-binary-v1")
	archiveBytes := makeTarGz(t, "ship", binary)
	ver := "1.0.0"
	asset := archiveName(ver, goos, goarch)
	sums := checksumLine(sha256Sum(archiveBytes), asset)
	srv := newReleaseServer(t, ver, asset, archiveBytes, sums)

	script := filepath.Join("..", "install.sh")
	cmd := exec.Command("bash", script)
	cmd.Env = append(os.Environ(),
		"SHIP_INSTALL_DIR="+binDir,
		"SHIP_GITHUB_API="+srv.URL,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, out)
	}

	installed := filepath.Join(binDir, "ship")
	got, err := os.ReadFile(installed)
	if err != nil {
		t.Fatalf("read installed ship: %v\n%s", err, out)
	}
	if !bytes.Equal(got, binary) {
		t.Errorf("installed binary = %q, want %q", got, binary)
	}
	info, err := os.Stat(installed)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("installed ship is not executable: mode=%v", info.Mode())
	}
}

func TestInstallScript_windowsIsUnixFirstClearError(t *testing.T) {
	binDir := t.TempDir()
	script := filepath.Join("..", "install.sh")
	cmd := exec.Command("bash", script)
	cmd.Env = cleanEnv(
		"SHIP_INSTALL_DIR="+binDir,
		"SHIP_GITHUB_API=http://127.0.0.1:1",
		"SHIP_OS=windows",
		"SHIP_ARCH=amd64",
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("install.sh succeeded on windows, want error\n%s", out)
	}
	if !strings.Contains(string(out), "unsupported") {
		t.Errorf("error should mention unsupported; got:\n%s", out)
	}
}

func TestInstallScript_selectsAssetForLinuxDarwinAmd64Arm64(t *testing.T) {
	platforms := []struct{ os, arch string }{
		{"linux", "amd64"},
		{"linux", "arm64"},
		{"darwin", "amd64"},
		{"darwin", "arm64"},
	}
	for _, p := range platforms {
		t.Run(p.os+"_"+p.arch, func(t *testing.T) {
			binDir := t.TempDir()
			payload := []byte("ship-" + p.os + "-" + p.arch)
			archiveBytes := makeTarGz(t, "ship", payload)
			ver := "3.1.4"
			asset := archiveName(ver, p.os, p.arch)
			sums := checksumLine(sha256Sum(archiveBytes), asset)
			sumsName := fmt.Sprintf("ship-cli_%s_checksums.txt", ver)

			var srvURL string
			var downloaded string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/repos/maxBRT/ship-cli/releases/latest":
					// Publish all four assets so a wrong pick would still 200 if the script
					// chose another name; we assert via downloaded asset path instead.
					var assets []string
					for _, q := range platforms {
						name := archiveName(ver, q.os, q.arch)
						assets = append(assets, githubAssetJSON(name, srvURL+"/download/"+name))
					}
					assets = append(assets, githubAssetJSON(sumsName, srvURL+"/download/"+sumsName))
					fmt.Fprintf(w, `{"url":"https://api.github.com/repos/maxBRT/ship-cli/releases/1","tag_name":"v%s","name":"v%s","assets":[%s]}`,
						ver, ver, strings.Join(assets, ","))
				case strings.HasPrefix(r.URL.Path, "/download/"):
					name := strings.TrimPrefix(r.URL.Path, "/download/")
					if name == sumsName {
						_, _ = io.WriteString(w, sums)
						return
					}
					downloaded = name
					if name != asset {
						http.NotFound(w, r)
						return
					}
					_, _ = w.Write(archiveBytes)
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(srv.Close)
			srvURL = srv.URL

			script := filepath.Join("..", "install.sh")
			cmd := exec.Command("bash", script)
			cmd.Env = cleanEnv(
				"SHIP_INSTALL_DIR="+binDir,
				"SHIP_GITHUB_API="+srv.URL,
				"SHIP_OS="+p.os,
				"SHIP_ARCH="+p.arch,
				"PATH="+binDir+string(os.PathListSeparator)+"/usr/bin:/bin",
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("install.sh failed: %v\n%s", err, out)
			}
			if downloaded != asset {
				t.Fatalf("downloaded %q, want %q", downloaded, asset)
			}
			got, err := os.ReadFile(filepath.Join(binDir, "ship"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, payload) {
				t.Errorf("binary = %q, want %q", got, payload)
			}
		})
	}
}

func TestInstallScript_defaultsToUserLocalBin(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	if !supportedPlatform(goos, goarch) {
		t.Skip("unsupported GOOS/GOARCH for install script test")
	}

	home := t.TempDir()
	wantDir := filepath.Join(home, ".local", "bin")
	binary := []byte("ship-binary-default")
	archiveBytes := makeTarGz(t, "ship", binary)
	ver := "1.0.0"
	asset := archiveName(ver, goos, goarch)
	sums := checksumLine(sha256Sum(archiveBytes), asset)
	srv := newReleaseServer(t, ver, asset, archiveBytes, sums)

	script := filepath.Join("..", "install.sh")
	cmd := exec.Command("bash", script)
	cmd.Env = cleanEnv(
		"HOME="+home,
		"SHIP_GITHUB_API="+srv.URL,
		"PATH="+wantDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, out)
	}
	got, err := os.ReadFile(filepath.Join(wantDir, "ship"))
	if err != nil {
		t.Fatalf("expected ship in ~/.local/bin: %v\n%s", err, out)
	}
	if !bytes.Equal(got, binary) {
		t.Errorf("installed binary = %q, want %q", got, binary)
	}
}

func TestInstallScript_rejectsNonHTTPSDownloadURL(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	if !supportedPlatform(goos, goarch) {
		t.Skip("unsupported GOOS/GOARCH for install script test")
	}

	binDir := t.TempDir()
	ver := "1.0.0"
	asset := archiveName(ver, goos, goarch)
	sumsName := fmt.Sprintf("ship-cli_%s_checksums.txt", ver)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/maxBRT/ship-cli/releases/latest" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, `{"tag_name":"v%s","name":"v%s","assets":[%s,%s]}`,
			ver, ver,
			githubAssetJSON(asset, "http://example.com/download/"+asset),
			githubAssetJSON(sumsName, "http://example.com/download/checksums.txt"),
		)
	}))
	t.Cleanup(srv.Close)

	script := filepath.Join("..", "install.sh")
	cmd := exec.Command("bash", script)
	cmd.Env = cleanEnv(
		"SHIP_INSTALL_DIR="+binDir,
		"SHIP_GITHUB_API="+srv.URL,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("install.sh succeeded, want HTTPS failure\n%s", out)
	}
	if !strings.Contains(string(out), "HTTPS") {
		t.Errorf("error should mention HTTPS; got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(binDir, "ship")); !os.IsNotExist(err) {
		t.Errorf("ship should not be installed after HTTPS failure; stat err=%v", err)
	}
}

func cleanEnv(extra ...string) []string {
	out := make([]string, 0, len(os.Environ())+len(extra))
	for _, e := range os.Environ() {
		switch {
		case strings.HasPrefix(e, "SHIP_INSTALL_DIR="),
			strings.HasPrefix(e, "SHIP_GITHUB_API="),
			strings.HasPrefix(e, "SHIP_OS="),
			strings.HasPrefix(e, "SHIP_ARCH="),
			strings.HasPrefix(e, "SHIP_REPO="),
			strings.HasPrefix(e, "HOME="),
			strings.HasPrefix(e, "PATH="):
			continue
		}
		out = append(out, e)
	}
	return append(out, extra...)
}

func TestInstallScript_errorsWhenInstallDirNotOnPATH(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	if !supportedPlatform(goos, goarch) {
		t.Skip("unsupported GOOS/GOARCH for install script test")
	}

	binDir := t.TempDir()
	binary := []byte("ship-binary-v1")
	archiveBytes := makeTarGz(t, "ship", binary)
	ver := "1.0.0"
	asset := archiveName(ver, goos, goarch)
	sums := checksumLine(sha256Sum(archiveBytes), asset)
	srv := newReleaseServer(t, ver, asset, archiveBytes, sums)

	// PATH deliberately excludes binDir.
	script := filepath.Join("..", "install.sh")
	cmd := exec.Command("bash", script)
	cmd.Env = append(os.Environ(),
		"SHIP_INSTALL_DIR="+binDir,
		"SHIP_GITHUB_API="+srv.URL,
		"PATH=/usr/bin:/bin",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "error:") || !strings.Contains(string(out), "not on PATH") {
		t.Errorf("expected PATH error; got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(binDir, "ship")); err != nil {
		t.Fatalf("ship should still be installed: %v", err)
	}
}

func TestInstallScript_unsupportedPlatformClearError(t *testing.T) {
	binDir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v1.0.0","assets":[]}`)
	}))
	t.Cleanup(srv.Close)

	script := filepath.Join("..", "install.sh")
	cmd := exec.Command("bash", script)
	cmd.Env = append(os.Environ(),
		"SHIP_INSTALL_DIR="+binDir,
		"SHIP_GITHUB_API="+srv.URL,
		"SHIP_OS=plan9",
		"SHIP_ARCH=amd64",
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("install.sh succeeded, want unsupported platform error\n%s", out)
	}
	if !strings.Contains(string(out), "unsupported") {
		t.Errorf("error should mention unsupported; got:\n%s", out)
	}
	if strings.Contains(string(out), "404") {
		t.Errorf("error should not be a cryptic download failure; got:\n%s", out)
	}
}

func TestInstallScript_checksumMismatchDoesNotInstall(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	if !supportedPlatform(goos, goarch) {
		t.Skip("unsupported GOOS/GOARCH for install script test")
	}

	binDir := t.TempDir()
	archiveBytes := makeTarGz(t, "ship", []byte("ship-binary-v1"))
	ver := "1.0.0"
	asset := archiveName(ver, goos, goarch)
	badSums := checksumLine(sha256Sum([]byte("not-the-archive")), asset)
	srv := newReleaseServer(t, ver, asset, archiveBytes, badSums)

	script := filepath.Join("..", "install.sh")
	cmd := exec.Command("bash", script)
	cmd.Env = append(os.Environ(),
		"SHIP_INSTALL_DIR="+binDir,
		"SHIP_GITHUB_API="+srv.URL,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("install.sh succeeded, want checksum failure\n%s", out)
	}
	if !strings.Contains(string(out), "checksum") {
		t.Errorf("stderr should mention checksum; got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(binDir, "ship")); !os.IsNotExist(err) {
		t.Errorf("ship should not be installed after checksum failure; stat err=%v", err)
	}
}

func supportedPlatform(goos, goarch string) bool {
	switch goos {
	case "linux", "darwin":
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

// githubAssetJSON mirrors the GitHub Releases API shape: "name" appears before a
// nested "uploader" object, and "browser_download_url" comes after it.
func githubAssetJSON(name, downloadURL string) string {
	return fmt.Sprintf(`{
		"url":"https://api.github.com/repos/maxBRT/ship-cli/releases/assets/1",
		"id":1,
		"node_id":"RA_test",
		"name":%q,
		"label":"",
		"uploader":{
			"login":"releaser",
			"id":1,
			"node_id":"U_test",
			"avatar_url":"https://example.test/a",
			"gravatar_id":"",
			"url":"https://api.github.com/users/releaser",
			"html_url":"https://github.com/releaser",
			"type":"User",
			"site_admin":false
		},
		"content_type":"application/gzip",
		"state":"uploaded",
		"size":1,
		"digest":null,
		"download_count":0,
		"created_at":"2026-01-01T00:00:00Z",
		"updated_at":"2026-01-01T00:00:00Z",
		"browser_download_url":%q
	}`, name, downloadURL)
}

func newReleaseServer(t *testing.T, ver, asset string, archive []byte, sums string) *httptest.Server {
	t.Helper()
	sumsName := fmt.Sprintf("ship-cli_%s_checksums.txt", ver)
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/maxBRT/ship-cli/releases/latest":
			fmt.Fprintf(w, `{"url":"https://api.github.com/repos/maxBRT/ship-cli/releases/1","tag_name":"v%s","name":"v%s","assets":[%s,%s]}`,
				ver, ver,
				githubAssetJSON(asset, srvURL+"/download/"+asset),
				githubAssetJSON(sumsName, srvURL+"/download/checksums.txt"),
			)
		case r.URL.Path == "/download/"+asset:
			_, _ = w.Write(archive)
		case r.URL.Path == "/download/checksums.txt":
			_, _ = io.WriteString(w, sums)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	srvURL = srv.URL
	return srv
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
