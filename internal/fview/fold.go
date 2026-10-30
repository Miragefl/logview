// Java 堆栈折叠检测(验收反馈④ e 键):复用 internal/stacktrace 的 Detect,
// 在 tab 缓冲尾窗上重扫,组以 seq 域表达(不受缓冲裁头/索引漂移影响)。
// 全量语义与 TUI"过滤后重检测"有微小差异(后端不知道过滤态),由前端按
// "组内可见行>1 且锚点可见才折叠"近似——设计既定决策。
package fview

import (
	"time"

	"github.com/justfun/logview/internal/buffer"
	"github.com/justfun/logview/internal/stacktrace"
)

// foldWindow 尾窗行数:窗口越小单轮重扫越便宜(4096 行 Detect ≈1ms);
// 连续性由"窗口起点回退到所有跨窗组 leader"兜住(跨窗不断裂)。
// 大文件加载的高频重扫由 shouldFoldScan 节流(500ms 或 8192 行增量)。
const foldWindow = 4096

// foldScanMinInterval / foldScanMinLines 节流阈值:任一满足才真正重扫。
const (
	foldScanMinInterval = 500 * time.Millisecond
	foldScanMinLines    = 8192
)

// shouldFoldScan 节流判定:距上次扫描满 500ms,或累计待扫行数 ≥8192。
// (Final Review #4:每 1000 行 flush 全量重扫 16k 窗口,664 万行加载额外
// 20-30s CPU;节流后加载期 ~2 扫/秒,follow 稳态 500ms 一扫,折叠出现延迟无感。)
func shouldFoldScan(lastScan time.Time, now time.Time, pendingLines int) bool {
	if pendingLines >= foldScanMinLines {
		return true
	}
	return now.Sub(lastScan) >= foldScanMinInterval
}

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
	// 回退:遍历上轮所有组,凡 leader 仍在缓冲但落标准窗口之前的都拉回窗口
	// (Final Review #2:只查末组时,跨窗巨组后紧跟新组会让巨组 leader 出窗整组丢失)
	for _, g := range d.window {
		if si := seqIndex(buf, g.StartSeq); si >= 0 && si < start {
			start = si
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
