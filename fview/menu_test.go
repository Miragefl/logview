package main

import (
	"testing"

	"github.com/wailsapp/wails/v2/pkg/menu"
)

// TestBuildAppMenuDarwin mac 菜单:App 菜单(⌘Q 退出)+文件>关闭标签页(⌘W)
// +Edit 菜单(⌘C/⌘V/⌘A 原生 selector——自定义 Menu 替换默认菜单后无此项则粘贴失效)。
func TestBuildAppMenuDarwin(t *testing.T) {
	closed, quit := false, false
	m := buildAppMenu("darwin", func() { closed = true }, func() { quit = true })
	if m == nil {
		t.Fatal("darwin 应返回菜单")
	}
	if len(m.Items) != 3 {
		t.Fatalf("应为 App+文件+Edit 三个子菜单,got %d", len(m.Items))
	}
	// Edit 菜单必须存在:WKWebView 的 ⌘V paste: 依赖菜单项派发 responder chain,
	// 无 Edit 菜单则 FRP 表单等所有输入框无法粘贴(2026-11-04 bug)
	if m.Items[2].Role != menu.EditMenuRole {
		t.Fatalf("第三项应为 EditMenuRole,got %+v", m.Items[2])
	}
	// 第一项必须是 App 菜单(macOS 惯例:首位即应用菜单,含 ⌘Q 退出)
	appItem := m.Items[0].SubMenu.Items[0]
	if m.Items[0].Label != "fview" || appItem.Label != "退出 fview" || appItem.Accelerator == nil {
		t.Fatalf("应为 App 菜单>退出 fview+加速键,got %+v", m.Items[0])
	}
	appItem.Click(&menu.CallbackData{MenuItem: appItem})
	if !quit {
		t.Fatal("退出项 Click 应触发 onQuit")
	}
	fileItem := m.Items[1].SubMenu.Items[0]
	if m.Items[1].Label != "文件" || fileItem.Label != "关闭标签页" || fileItem.Accelerator == nil {
		t.Fatalf("应为 文件>关闭标签页+加速键,got %+v", m.Items[1])
	}
	fileItem.Click(&menu.CallbackData{MenuItem: fileItem})
	if !closed {
		t.Fatal("关闭标签页 Click 应触发 onCloseTab")
	}
}

// TestBuildAppMenuNonDarwinNil 非 mac 返回 nil(ctrl+w 留给 webview,防双触发)。
func TestBuildAppMenuNonDarwinNil(t *testing.T) {
	for _, goos := range []string{"windows", "linux"} {
		if m := buildAppMenu(goos, func() {}, func() {}); m != nil {
			t.Fatalf("%s 应返回 nil", goos)
		}
	}
}
