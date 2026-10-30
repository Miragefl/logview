package main

import "context"

// App fview 桌面端后端绑定根（后续任务在此挂 Manager/搜索/主题等绑定方法）。
type App struct {
	ctx context.Context
}

// NewApp 构造后端绑定根对象。
func NewApp() *App {
	return &App{}
}

// startup Wails 启动回调，保存 ctx 供 runtime 调用。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}
