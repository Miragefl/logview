package tui

import (
	"os"
	"testing"
)

// TestMain 统一把 HOME 指到临时目录：usage.json/session.json 等状态文件
// 不污染开发者真实环境，用例间也相互隔离频次数据。
func TestMain(m *testing.M) {
	os.Setenv("HOME", os.TempDir())
	os.Exit(m.Run())
}
