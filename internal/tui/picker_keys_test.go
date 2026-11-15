// picker 键迁移测试(tui-picker-unify):Esc 逐级/C-d 删/C-e 编辑回填/C-a 新增/
// K8s C-cC-n 直达/SSH 主机层过滤·输入态分离——fview 面板键系对齐。
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Miragefl/logview/internal/frp"
)

func TestPickerKeysEsc(t *testing.T) {
	t.Run("Esc 逐级:词→层→关", func(t *testing.T) {
		app := newFilterApp(3)
		app.pickerFRPLevel = 2
		app.pickerDirFilter = "xy" // 词非空
		app.Update(tea.KeyMsg{Type: tea.KeyEscape})
		if app.pickerDirFilter != "" || app.pickerFRPLevel != 2 || !app.sourcePickerMode {
			t.Fatalf("Esc1 应只清词:词=%q L=%d mode=%v", app.pickerDirFilter, app.pickerFRPLevel, app.sourcePickerMode)
		}
		app.Update(tea.KeyMsg{Type: tea.KeyEscape})
		if app.pickerFRPLevel != 0 || !app.sourcePickerMode {
			t.Fatalf("Esc2 应回 L0: L=%d mode=%v", app.pickerFRPLevel, app.sourcePickerMode)
		}
		app.Update(tea.KeyMsg{Type: tea.KeyEscape})
		if app.sourcePickerMode {
			t.Fatalf("Esc3 应关 picker")
		}
	})
}

func TestPickerKeysDCtrl(t *testing.T) {
	t.Run("C-d 删 FRP 连接(C-x 退役)", func(t *testing.T) {
		app := newFilterApp(3)
		setupFRPStore(t, frp.Conn{Name: "del-me", Type: frp.TypeTCP, ServerAddr: "a.com", ServerPort: 7000, RemotePort: 6000})
		app.openSourcePicker(3)
		// 光标落在 +new(0),C-j 到 del-me
		app.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
		app.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
		if _, ok := frpStore().FindConn("del-me"); ok {
			t.Fatalf("C-d 应删除选中连接")
		}
	})
}

func TestPickerKeysECtrl(t *testing.T) {
	t.Run("C-e 编辑 FRP 回填表单", func(t *testing.T) {
		app := newFilterApp(3)
		setupFRPStore(t, frp.Conn{Name: "edit-me", Type: frp.TypeTCP, ServerAddr: "srv.io", ServerPort: 7000, RemotePort: 6100})
		app.openSourcePicker(3)
		app.Update(tea.KeyMsg{Type: tea.KeyCtrlJ}) // 光标→edit-me
		app.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
		if app.pickerFRPLevel != 1 {
			t.Fatalf("C-e 应进表单层, L=%d", app.pickerFRPLevel)
		}
		if app.pickerFRPServerAddr != "srv.io" || app.pickerFRPRemotePort != 6100 {
			t.Fatalf("表单应回填现值: addr=%q port=%d", app.pickerFRPServerAddr, app.pickerFRPRemotePort)
		}
	})
}

func TestPickerKeysACtrl(t *testing.T) {
	t.Run("C-a FRP→新建表单", func(t *testing.T) {
		app := newFilterApp(3)
		app.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
		if app.pickerFRPLevel != 1 || app.pickerFRPFormType != "" {
			t.Fatalf("C-a 应进空表单: L=%d type=%q", app.pickerFRPLevel, app.pickerFRPFormType)
		}
	})
	t.Run("C-a SSH→新增主机表单", func(t *testing.T) {
		app := newFilterApp(2)
		app.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
		if !app.pickerSSHInputMode {
			t.Fatalf("C-a 应进 SSH 新增主机表单")
		}
	})
	t.Run("C-a SSH 目录层不进表单态(Final Review I3)", func(t *testing.T) {
		app := newFilterApp(2)
		app.pickerSSHHost = "srv1" // 远程目录浏览层：表单无渲染路径，进态=隐形错位
		app.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
		if app.pickerSSHInputMode {
			t.Fatalf("目录层 C-a 不应进表单态（渲染/helpbar/Enter 全错位）")
		}
	})
	t.Run("SSH 默认过滤态:打字过滤主机列表", func(t *testing.T) {
		app := newFilterApp(2)
		app.Update(keyMsg("w"))
		if app.pickerDirFilter != "w" {
			t.Fatalf("SSH 层打字应进过滤词,got hostInput=%q dirFilter=%q", app.pickerHostInput, app.pickerDirFilter)
		}
	})
}

func TestPickerKeysK8sJump(t *testing.T) {
	t.Run("C-c 直达 ctx 层/C-n 直达 ns 层", func(t *testing.T) {
		app := newFilterApp(0)
		app.pickerK8sLevel = 2
		app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		if app.pickerK8sLevel != 0 {
			t.Fatalf("C-c 应回 ctx 层, L=%d", app.pickerK8sLevel)
		}
		app.pickerKubeCtx = "prod"
		app.pickerNamespaces = []sourceCandidate{{label: "default", value: "default"}}
		app.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
		if app.pickerK8sLevel != 1 {
			t.Fatalf("C-n 应直达 ns 层, L=%d", app.pickerK8sLevel)
		}
	})
	_ = strings.TrimSpace
}

// TestPickerEscDeep Final Review C1:Esc 逐级在层内导航态(表单 step>0/嵌套目录)
// 不得一步关顶——handled 标志根治(快照键不完备的根治替代)。
func TestPickerEscDeep(t *testing.T) {
	t.Run("FRP 表单 step>0 按 Esc 回退一步不关", func(t *testing.T) {
		app := newFilterApp(3)
		app.pickerFRPLevel = 1
		app.pickerFRPStep = 2
		app.pickerFRPFormType = "stcp"
		app.Update(tea.KeyMsg{Type: tea.KeyEscape})
		if !app.sourcePickerMode || app.pickerFRPLevel != 1 || app.pickerFRPStep != 1 {
			t.Fatalf("Esc 应回退一步不关:mode=%v L=%d step=%d", app.sourcePickerMode, app.pickerFRPLevel, app.pickerFRPStep)
		}
	})
	t.Run("SSH 嵌套目录按 Esc 回主机层不关(目录上翻归 Backspace)", func(t *testing.T) {
		app := newFilterApp(2)
		app.pickerSSHHost = "web1"
		app.pickerSSHDir = "/var/log/app"
		app.Update(tea.KeyMsg{Type: tea.KeyEscape})
		if !app.sourcePickerMode || app.pickerSSHHost != "" {
			t.Fatalf("Esc 应回主机层不关:mode=%v host=%q", app.sourcePickerMode, app.pickerSSHHost)
		}
	})
	t.Run("ns 层词在 NsInput:Esc 先清词再回层", func(t *testing.T) {
		app := newFilterApp(0)
		app.pickerK8sLevel = 1
		app.pickerKubeCtx = "prod"
		app.pickerNsInput = "kube"
		app.Update(tea.KeyMsg{Type: tea.KeyEscape})
		if app.pickerNsInput != "" || app.pickerK8sLevel != 1 {
			t.Fatalf("ns 层 Esc 应清 NsInput 词:ns=%q L=%d", app.pickerNsInput, app.pickerK8sLevel)
		}
		app.Update(tea.KeyMsg{Type: tea.KeyEscape})
		if app.pickerK8sLevel != 0 {
			t.Fatalf("再 Esc 应回 ctx 层")
		}
	})
}

// TestPickerFixes Final Review I2/I4/I6:C-a 守卫/目录层 isSubseq/SSH 输入态 q。
func TestPickerFixes(t *testing.T) {
	t.Run("C-a 表单中不劫持(L1 保持字段)", func(t *testing.T) {
		app := newFilterApp(3)
		app.pickerFRPLevel = 1
		app.pickerFRPStep = 1
		app.pickerFRPServerAddr = "keep.me"
		app.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
		if app.pickerFRPServerAddr != "keep.me" || app.pickerFRPStep != 1 {
			t.Fatalf("C-a 不得在表单中清空字段:addr=%q step=%d", app.pickerFRPServerAddr, app.pickerFRPStep)
		}
	})
	t.Run("SSH 目录层子序列过滤(vl→var/log)", func(t *testing.T) {
		app := newFilterApp(2)
		app.pickerSSHHost = "web1"
		app.pickerCandidates = []sourceCandidate{
			{label: "var/log/", value: "var/log", dir: true},
			{label: "zzz.txt", value: "zzz.txt"},
		}
		app.pickerDirFilter = "vl"
		cands := app.visiblePickerCandidates()
		if len(cands) != 1 || cands[0].value != "var/log" {
			t.Fatalf("vl 应子序列命中 var/log/, got %v", cands)
		}
	})
	t.Run("SSH 输入态首字母 q 不杀 TUI", func(t *testing.T) {
		app := newFilterApp(2)
		app.pickerSSHInputMode = true
		app.Update(keyMsg("q"))
		if app.pickerHostInput != "q" || !app.sourcePickerMode {
			t.Fatalf("输入态 q 应进 hostInput:=%q mode=%v", app.pickerHostInput, app.sourcePickerMode)
		}
	})
	t.Run("三个无框层有 ⌕ 回显", func(t *testing.T) {
		app := newFilterApp(0)
		app.pickerK8sLevel = 0
		app.pickerContexts = []sourceCandidate{{label: "prod", value: "prod"}}
		app.Update(keyMsg("pr"))
		v := app.View()
		if !strings.Contains(StripANSIForTest(v), "⌕ pr") {
			t.Fatalf("ctx 层应显示过滤词回显 ⌕ pr")
		}
	})
}
