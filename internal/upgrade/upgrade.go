package upgrade

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	MirrorGitee  = "gitee"
	MirrorGithub = "github"

	githubRepo = "Miragefl/logview"
	giteeRepo  = "Mtok/logview"
)

// Latest returns the latest release tag (e.g. "v0.12.17") from the given mirror.
func Latest(mirror string) (string, error) {
	var url string
	switch mirror {
	case MirrorGitee:
		url = fmt.Sprintf("https://gitee.com/api/v5/repos/%s/releases/latest", giteeRepo)
	case MirrorGithub:
		url = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)
	default:
		return "", fmt.Errorf("unknown mirror: %s (use gitee or github)", mirror)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to query %s: %w", mirror, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s returned status %s", mirror, resp.Status)
	}

	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("failed to parse %s response: %w", mirror, err)
	}
	if body.TagName == "" {
		return "", fmt.Errorf("%s returned empty tag_name", mirror)
	}
	return body.TagName, nil
}

// AssetURL builds the download URL for the current platform from the given mirror and tag.
func AssetURL(mirror, tag string) (string, error) {
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}
	asset := fmt.Sprintf("logview_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	switch mirror {
	case MirrorGitee:
		return fmt.Sprintf("https://gitee.com/%s/releases/download/%s/%s", giteeRepo, tag, asset), nil
	case MirrorGithub:
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", githubRepo, tag, asset), nil
	default:
		return "", fmt.Errorf("unknown mirror: %s (use gitee or github)", mirror)
	}
}

// CompareVersions returns >0 if a is newer than b, 0 if equal, <0 if older.
// Both may optionally start with "v". Non-semver input is compared lexically.
func CompareVersions(a, b string) int {
	as := strings.TrimPrefix(a, "v")
	bs := strings.TrimPrefix(b, "v")
	var ai, bi []int
	for _, part := range strings.Split(as, ".") {
		n := 0
		if _, err := fmt.Sscanf(part, "%d", &n); err != nil {
			return strings.Compare(as, bs)
		}
		ai = append(ai, n)
	}
	for _, part := range strings.Split(bs, ".") {
		n := 0
		if _, err := fmt.Sscanf(part, "%d", &n); err != nil {
			return strings.Compare(as, bs)
		}
		bi = append(bi, n)
	}
	for i := 0; i < len(ai) || i < len(bi); i++ {
		var av, bv int
		if i < len(ai) {
			av = ai[i]
		}
		if i < len(bi) {
			bv = bi[i]
		}
		if av != bv {
			return av - bv
		}
	}
	return 0
}

// IsBrewInstall reports whether the executable at path was installed by Homebrew.
func IsBrewInstall(path string) bool {
	p := filepath.Clean(path)
	return strings.Contains(p, "/Cellar/logview/") ||
		strings.HasPrefix(p, "/opt/homebrew/bin/") ||
		strings.HasPrefix(p, "/usr/local/bin/") && strings.Contains(p, "homebrew") ||
		strings.HasPrefix(p, "/home/linuxbrew/.linuxbrew/bin/")
}

// Download fetches url into a temp file next to the current executable's directory
// and returns its path. Caller is responsible for cleanup on failure.
func Download(url string) (string, error) {
	tmp, err := os.CreateTemp("", "logview-upgrade-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", fmt.Errorf("download failed: status %s", resp.Status)
	}

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", fmt.Errorf("download failed: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("download failed: %w", err)
	}
	return tmp.Name(), nil
}

// ExtractBinary untars the "logview" binary from archivePath into dstDir
// and returns the path to the extracted executable.
func ExtractBinary(archivePath, dstDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("failed to open archive: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("failed to decompress archive: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return "", fmt.Errorf("logview binary not found in archive")
		}
		if err != nil {
			return "", fmt.Errorf("failed to read archive: %w", err)
		}
		name := filepath.Base(hdr.Name)
		if hdr.Typeflag != tar.TypeReg || (name != "logview" && name != "logview.exe") {
			continue
		}
		dst := filepath.Join(dstDir, name)
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return "", fmt.Errorf("failed to write binary: %w", err)
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return "", fmt.Errorf("failed to write binary: %w", err)
		}
		if err := out.Close(); err != nil {
			return "", fmt.Errorf("failed to write binary: %w", err)
		}
		return dst, nil
	}
}

// ReplaceSelf atomically replaces the current executable with the binary at newPath.
// Returns the path of the old binary backup if rename-back is needed.
func ReplaceSelf(newPath string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to locate current executable: %w", err)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	// write permission check first for a clearer error
	if info, err := os.Stat(filepath.Dir(self)); err == nil && info.Mode().Perm()&0o200 == 0 {
		return fmt.Errorf("no write permission to %s (try: sudo logview upgrade)", filepath.Dir(self))
	}

	if err := os.Rename(newPath, self); err != nil {
		return fmt.Errorf("failed to replace %s: %w (try: sudo logview upgrade)", self, err)
	}
	return nil
}
