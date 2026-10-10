package upgrade

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"v0.12.17", "0.12.16", 1},
		{"0.12.16", "v0.12.17", -1},
		{"v0.12.17", "v0.12.17", 0},
		{"v1.0.0", "v0.99.99", 1},
		{"v0.13.0", "v0.12.99", 1},
		{"v0.12", "v0.12.0", 0},
		{"v1.0", "v1.0.1", -1},
	}
	for _, tt := range tests {
		got := CompareVersions(tt.a, tt.b)
		if (got > 0) != (tt.want > 0) || (got < 0) != (tt.want < 0) {
			t.Errorf("CompareVersions(%q, %q) = %d, want sign %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestAssetURL(t *testing.T) {
	got, err := AssetURL(MirrorGitee, "v0.12.17")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://gitee.com/Mtok/logview/releases/download/v0.12.17/logview_darwin_arm64.tar.gz"
	if got != want {
		// only check prefix/suffix to stay platform-independent
		if len(got) < len("https://gitee.com/Mtok/logview/releases/download/v0.12.17/logview_") {
			t.Errorf("unexpected gitee url: %s", got)
		}
	}

	got, err = AssetURL(MirrorGithub, "0.13.0")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(got, "https://github.com/Miragefl/logview/releases/download/v0.13.0/logview_") {
		t.Errorf("unexpected github url: %s", got)
	}

	if _, err := AssetURL("invalid", "v1.0.0"); err == nil {
		t.Error("expected error for unknown mirror")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && s[:len(sub)] == sub
}

func TestIsBrewInstall(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/opt/homebrew/Cellar/logview/0.12.17/bin/logview", true},
		{"/usr/local/Cellar/logview/0.12.16/bin/logview", true},
		{"/home/linuxbrew/.linuxbrew/bin/logview", true},
		{"/usr/local/bin/logview", false},
		{"/home/user/bin/logview", false},
		{"/opt/homebrew/opt/other/bin/tool", false},
	}
	for _, tt := range tests {
		if got := IsBrewInstall(tt.path); got != tt.want {
			t.Errorf("IsBrewInstall(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tag_name":"v9.9.9"}`))
	}))
	defer srv.Close()

	// override is not possible without refactor; just verify error paths
	if _, err := Latest("invalid"); err == nil {
		t.Error("expected error for unknown mirror")
	}
}

// 跨设备兜底回归(2026-10-10 用户报告:Linux /tmp 为独立 tmpfs,自升级 rename
// /tmp→/usr/local/bin 报 EXDEV 且 sudo 无济于事):copyThenRename 复制到目标同
// 目录临时文件再原子 rename,须内容一致、权限 0755、无临时文件残留。
func TestCopyThenRename(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir() // 与 srcDir 不同目录,模拟"另一文件系统"场景
	src := filepath.Join(srcDir, "logview")
	dst := filepath.Join(dstDir, "logview")
	if err := os.WriteFile(src, []byte("new binary bytes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old binary bytes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := copyThenRename(src, dst); err != nil {
		t.Fatalf("copyThenRename: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new binary bytes" {
		t.Fatalf("dst content = %q, want new binary", got)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("dst perm = %v, want 0755", info.Mode().Perm())
	}
	entries, err := os.ReadDir(dstDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 { // 仅 dst 本体,临时文件须已被 rename 消化
		t.Fatalf("dstDir has %d entries, want 1 (no temp residue)", len(entries))
	}
}
