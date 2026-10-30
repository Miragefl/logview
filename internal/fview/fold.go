// Java 堆栈折叠检测(验收反馈④ e 键):复用 internal/stacktrace 的 Detect,
// 在 tab 缓冲尾窗上重扫,组以 seq 域表达(不受缓冲裁头/索引漂移影响)。
// 全量语义与 TUI"过滤后重检测"有微小差异(后端不知道过滤态),由前端按
// "组内可见行>1 且锚点可见才折叠"近似——设计既定决策。
package fview

import (
	"github.com/justfun/logview/internal/buffer"
	"github.com/justfun/logview/internal/stacktrace"
)

// foldWindow 尾窗行数:每轮 flush 间隔增量 ≤1000 行,窗口远大于增量保证连续;
// 未终局长组由"窗口起点回退到上轮末组 leader"兜住(跨窗不断裂)。
const foldWindow = 16384

// FoldRange 折叠组(seq 域,含端点;Leader 为异常首行)。
type FoldRange struct {
	StartSeq uint64 `json:"startSeq"`
	EndSeq   uint64 `json:"endSeq"`
	Leader   string `json:"leader"`
}

// foldDetector 尾窗堆栈检测器:滑出窗口的已终结组进 finals 原样保留;
// 窗口内全量重扫(含回退区),与 TUI DetectAppend 的回退思想同源。
type foldDetector struct {
	finals []FoldRange // 终局组(EndSeq 已滑出窗口,不再变化)
	window []FoldRange // 上轮窗口内组(本轮重扫替代)
	win    int         // 窗口行数(0 = 默认 foldWindow;测试注入小窗)
}

// update 取缓冲尾窗重扫,返回全量折叠组(finals + 本轮窗口组)。
// 必须持 tab 锁调用(RingBuffer 无锁)。
func (d *foldDetector) update(buf *buffer.RingBuffer) []FoldRange {
	n := buf.Len()
	if n == 0 {
		d.window = nil
		return append([]FoldRange(nil), d.finals...)
	}
	w := d.win
	if w <= 0 {
		w = foldWindow
	}
	start := n - w
	if start < 0 {
		start = 0
	}
	// 回退:上轮末组 leader 仍在缓冲但已落标准窗口之前 → 窗口扩到它,跨窗不断裂
	if len(d.window) > 0 {
		if idx := seqIndex(buf, d.window[len(d.window)-1].StartSeq); idx >= 0 && idx < start {
			start = idx
		}
	}
	lines := buf.Slice(start, n)
	firstSeq := lines[0].Raw.Seq
	// 终局:上轮窗口组完全落在本轮窗口前 → 固化,再快照全量
	for _, g := range d.window {
		if g.EndSeq < firstSeq {
			d.finals = append(d.finals, g)
		}
	}
	out := append([]FoldRange(nil), d.finals...)
	var win []FoldRange
	for _, g := range stacktrace.Detect(lines) {
		win = append(win, FoldRange{
			StartSeq: lines[g.Start].Raw.Seq,
			EndSeq:   lines[g.End].Raw.Seq,
			Leader:   g.Leader,
		})
	}
	d.window = win
	return append(out, win...)
}

// seqIndex 有序 seq 缓冲二分定位;不存在返回 -1。
func seqIndex(buf *buffer.RingBuffer, seq uint64) int {
	lo, hi := 0, buf.Len()-1
	for lo <= hi {
		mid := int(uint(lo+hi) >> 1)
		s := buf.Get(mid).Raw.Seq
		if s == seq {
			return mid
		}
		if s < seq {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return -1
}
