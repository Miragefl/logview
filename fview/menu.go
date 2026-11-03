package main

import (
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
)

// buildAppMenu 构造应用菜单(macOS 专属):fview(App 菜单)>退出 ⌘Q、文件 > 关闭标签页 ⌘W。
// ⌘W/⌘Q 是 mac 肌肉记忆,但 WKWebView 的 keydown 拦不住系统级按键,
// 故走原生菜单 key equivalent 路由(Chrome/VSCode 同款机制):关闭标签页回调经
// onCloseTab 广播 shortcut:closeTab 事件(前端 App.vue 订阅后关 activeId),
// 退出回调经 onQuit 触发 runtime.Quit(shutdown 钩子兜底杀 frpc visitor)。
// macOS 的 ⌘Q 退出必须由 app 菜单项提供(AppKit 不自动注入),无此项则按键无人消费。
// 非 darwin 平台返回 nil:ctrl+w 已由 webview keydown 接管(CTRL_MAP.w),
// 若注册菜单加速键会与 webview 双触发,一次按键关两个 tab。
func buildAppMenu(goos string, onCloseTab, onQuit func()) *menu.Menu {
	if goos != "darwin" {
		return nil
	}
	m := menu.NewMenu()
	// App 菜单必须是第一个子菜单(macOS 把首位当应用菜单,加粗显示)
	appMenu := m.AddSubmenu("fview")
	appMenu.AddText("退出 fview", keys.CmdOrCtrl("q"), func(*menu.CallbackData) {
		onQuit()
	})
	fileMenu := m.AddSubmenu("文件")
	fileMenu.AddText("关闭标签页", keys.CmdOrCtrl("w"), func(*menu.CallbackData) {
		onCloseTab()
	})
	return m
}
