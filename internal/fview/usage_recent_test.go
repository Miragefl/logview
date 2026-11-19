// UsageRecent 存在性过滤测试:file: 项剔除 os.IsNotExist 的本地文件,
// 其他 stat 错误(权限)与远程 key(ssh:/frp:/k8sres: 无法本地检测)原样保留。
package fview

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Miragefl/logview/internal/usage"
)

func TestUsageRecentFiltersMissingFiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	usage.ResetForTest()

	// 存在的文件 / 已消失的文件 / 远程 key(不参与检测)
	exist := filepath.Join(t.TempDir(), "app.log")
	if err := os.WriteFile(exist, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(t.TempDir(), "gone.log")

	UsageTouch("file:" + exist)
	UsageTouch("file:" + gone)
	UsageTouch("ssh:host:/var/log/a.log")

	got := UsageRecent()
	has := func(k string) bool { return slices.Contains(got, k) }
	if !has("file:"+exist) {
		t.Fatalf("存在的文件应保留, got %v", got)
	}
	if has("file:"+gone) {
		t.Fatalf("已消失的文件应剔除, got %v", got)
	}
	if !has("ssh:host:/var/log/a.log") {
		t.Fatalf("远程 key 不参与存在性检测,应保留, got %v", got)
	}
}
