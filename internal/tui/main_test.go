package tui

import (
	"os"
	"testing"
)

// TestMain 把 HOME 指到稳定临时目录：usage.json/session.json 等状态文件
// 不污染开发者真实环境。需要完全隔离的用例自行 t.Setenv + resetUsageForTest。
func TestMain(m *testing.M) {
	os.Setenv("HOME", os.TempDir())
	os.Exit(m.Run())
}
