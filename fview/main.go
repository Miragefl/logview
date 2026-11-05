package main

import (
	"embed"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	mac "github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// fview 桌面端入口：Wails v2 工程根在 fview/（与仓库根 TUI main.go 隔离，
	// 共享同一 go module 与 internal/ 核心）。
	// 必须最先执行:GUI 启动的进程 PATH 缺 brew 目录,后面 exec frpc/kubectl 全靠它
	augmentPATH()
	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "fview",
		Width:  1400,
		Height: 900,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		// mac 标题栏融合(IDEA 式):交通灯保留、藏标题、内容顶到窗口顶,
		// tab 栏升格为标题栏行(前端 .topbar 左侧留 78px 红绿灯位,空隙可拖拽)。
		// 不用 TitleBarHiddenInset() 预设——其 UseToolbar 空工具栏会把系统标题栏
		// 撑高,交通灯垂直中心随之下移(topbar 内偏下);无 toolbar 时标题栏 28px,
		// 交通灯中心 14px,与 .topbar 28px 高度严格居中对齐
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  true,
				FullSizeContent:            true,
			},
		},
		// mac 注册 App 菜单(⌘Q 退出)+文件>关闭标签页 ⌘W 原生菜单（非 darwin 为 nil，ctrl+w 走 webview）
		Menu:      buildAppMenu(runtime.GOOS, app.shortcutCloseTab, app.menuQuit),
		OnStartup: app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
