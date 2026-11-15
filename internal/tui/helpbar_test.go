package tui

import (
	"strings"
	"testing"
)

// pickerShortcutItems 表驱动：底部快捷键栏按 tab×层级×输入态按需显示
// （picker-width-hints 键位正本，来自 handleSourcePickerKeys 实测行为）。
func TestPickerShortcutItems(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*App)
		want  []helpItem
	}{
		{"K8s ctx 层", func(a *App) { a.sourceTab = 0; a.pickerK8sLevel = 0 },
			[]helpItem{{"Enter", "进入"}, {"C-j/C-k", "移动"}, {"C-c/C-n", "直达"}, {"Tab", "切分类"}, {"Esc", "关闭"}}},
		{"K8s ns 层", func(a *App) { a.sourceTab = 0; a.pickerK8sLevel = 1 },
			[]helpItem{{"Enter", "选择"}, {"C-j/C-k", "移动"}, {"C-c/C-n", "直达"}, {"Tab", "切分类"}, {"Esc", "返回"}}},
		{"K8s 资源层", func(a *App) { a.sourceTab = 0; a.pickerK8sLevel = 2 },
			[]helpItem{{"Enter", "打开"}, {"Space", "多选"}, {"C-j/C-k", "移动"}, {"C-c/C-n", "直达"}, {"Tab", "切分类"}, {"Esc", "返回"}}},
		{"本地", func(a *App) { a.sourceTab = 1 },
			[]helpItem{{"Enter", "打开"}, {"C-j/C-k", "移动"}, {"Backspace", "删词/上级"}, {"C-h", "上级"}, {"Tab", "切分类"}, {"Esc", "关闭"}}},
		{"SSH 主机过滤态", func(a *App) { a.sourceTab = 2 },
			[]helpItem{{"Enter", "连接"}, {"C-j/C-k", "移动"}, {"C-a", "新增主机"}, {"C-d", "删除"}, {"Tab", "切分类"}, {"Esc", "关闭"}}},
		{"SSH 新增表单", func(a *App) { a.sourceTab = 2; a.pickerSSHInputMode = true },
			// keybind-unify-r2 A3：单字段保存表单（C-l 双框退役）
			[]helpItem{{"Enter", "保存"}, {"Esc", "取消"}, {"Tab", "切分类"}}},
		{"SSH 目录层", func(a *App) { a.sourceTab = 2; a.pickerSSHHost = "srv1" },
			[]helpItem{{"Enter", "打开"}, {"C-j/C-k", "移动"}, {"Backspace", "删词/上级"}, {"C-h", "上级"}, {"Tab", "切分类"}, {"Esc", "返回"}}},
		{"FRP 连接列表", func(a *App) { a.sourceTab = 3; a.pickerFRPLevel = 0 },
			[]helpItem{{"Enter", "连接"}, {"C-j/C-k", "移动"}, {"C-a", "新增"}, {"C-e", "编辑"}, {"C-d", "删除"}, {"Tab", "切分类"}, {"Esc", "关闭"}}},
		{"FRP 新建表单", func(a *App) { a.sourceTab = 3; a.pickerFRPLevel = 1 },
			[]helpItem{{"Enter", "下一步"}, {"C-u", "清空"}, {"Esc", "回退"}, {"Tab", "切分类"}}},
		{"FRP 远程目录", func(a *App) { a.sourceTab = 3; a.pickerFRPLevel = 2 },
			[]helpItem{{"Enter", "打开"}, {"C-j/C-k", "移动"}, {"Backspace", "删词/上级"}, {"C-h", "上级"}, {"Tab", "切分类"}, {"Esc", "返回"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			app := newTestApp()
			app.sourcePickerMode = true
			c.setup(app)
			got := app.pickerShortcutItems()
			if len(got) != len(c.want) {
				t.Fatalf("items 数 = %d, want %d: %+v", len(got), len(c.want), got)
			}
			for i, w := range c.want {
				if got[i] != w {
					t.Errorf("[%d] = {%s %s}, want {%s %s}", i, got[i].key, got[i].desc, w.key, w.desc)
				}
			}
		})
	}
}

// 渲染接线：FRP L0 底部栏出现键位说明；主日志界面快捷键栏不受影响。
func TestPickerFooterHints(t *testing.T) {
	app := newTestApp()
	app.sourcePickerMode = true
	app.sourceTab = 3
	footer := app.renderFooter()
	for _, kw := range []string{"新增", "编辑", "删除"} {
		if !strings.Contains(footer, kw) {
			t.Errorf("FRP L0 底部栏应含 %q:\n%s", kw, footer)
		}
	}

	app2 := newTestApp()
	if f := app2.renderFooter(); !strings.Contains(f, "搜索") {
		t.Errorf("主界面底部栏应不受影响(含 /搜索):\n%s", f)
	}
}

// keybind-unify-r2 T8：帮助弹窗文案与实际 handler 对齐——假承诺/陈旧语义清零。
func TestHelpPopupContent(t *testing.T) {
	var all strings.Builder
	for _, sec := range helpSections() {
		all.WriteString(sec.title + "\n")
		for _, it := range sec.items {
			all.WriteString(it.key + " " + it.desc + "\n")
		}
	}
	s := all.String()

	// 陈旧假承诺不得复现（历史漂移项；"D 全部级别"=旧错档位描述，新 D=仅DEBUG）
	for _, bad := range []string{"切换分区/字段", "上下字段", "D 全部级别"} {
		if strings.Contains(s, bad) {
			t.Errorf("help 不应再含陈旧文案 %q:\n%s", bad, s)
		}
	}
	// 新语义条目（keybind-unify-r2 各任务落地后的正本）
	for _, want := range []string{
		"仅DEBUG",             // D=DEBUG 档（app.go toggleLevelFilter("DEBUG")）
		"需已选 ctx",          // C-n 直达 ns 层的生效条件（A4 豁免注记）
		"上下文混入",          // +/- 混入键此前 help 全缺
		"路径直达",            // 目录层 Enter /path 直达
		"删词",                // B2 C-d 词表删词
		"回上级",              // B1 C-h 目录层回上级
		"标记列表",            // A2 ' 键新语义
	} {
		if !strings.Contains(s, want) {
			t.Errorf("help 应含 %q:\n%s", want, s)
		}
	}
}
