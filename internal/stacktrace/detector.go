package stacktrace

import (
	"strings"

	"github.com/justfun/logview/internal/model"
)

type Group struct {
	Start  int
	End    int
	Leader string
}

func Detect(lines []*model.ParsedLine) []Group {
	var groups []Group
	i := 0
	for i < len(lines) {
		if isExceptionLine(lines[i].Message()) {
			start := i
			leader := lines[i].Message()
			i++
			for i < len(lines) && isStackFrame(lines[i].Message()) {
				i++
			}
			for i < len(lines) && isCausedBy(lines[i].Message()) {
				i++
				for i < len(lines) && isStackFrame(lines[i].Message()) {
					i++
				}
			}
			groups = append(groups, Group{
				Start:  start,
				End:    i - 1,
				Leader: leader,
			})
		} else {
			i++
		}
	}
	return groups
}

// DetectAppend 增量堆栈检测:保留完全落在重扫起点之前的旧组,尾部区间重扫重建。
// 相比每批全量 Detect,单批代价从 O(len) 降为 O(重扫区+回退组),大文件加载不再平方恶化。
//
// 契约与回退规则:
//   - overlap 必须 > 调用方单批最大行数(waitForStream 每批 ≤1000,调用处传 1256),
//     保证任何新行在被移出重扫区前至少被完整扫描一次,不留下藏组头的盲区
//   - 组跨入/落在重扫区(End >= rescan),或组未终结(End 是批尾截断,End+1 行仍可延续组)
//     时,重扫起点回退到该组 Start,整组从头重建,保证跨批延续与超长组不断裂
//   - 病态密集堆栈(巨型组)下退化为每批 O(组长+批),仍远优于全量 O(len)
func DetectAppend(lines []*model.ParsedLine, prev []Group, overlap int) []Group {
	rescan := max(0, len(lines)-overlap)
	for _, g := range prev {
		if g.End >= rescan {
			rescan = min(rescan, g.Start) // 跨/落在重扫区的组:整组从头重扫
		} else if g.End+1 >= len(lines) || continuesGroup(lines[g.End+1].Message()) {
			rescan = min(rescan, g.Start) // 未终结组:后续行仍可能扩展,从头重扫
		}
	}
	var groups []Group
	for _, g := range prev {
		if g.End < rescan {
			groups = append(groups, g) // 完全落在重扫区前的已终结旧组,原样保留
		}
	}
	for _, g := range Detect(lines[rescan:]) {
		groups = append(groups, Group{
			Start:  g.Start + rescan,
			End:    g.End + rescan,
			Leader: g.Leader,
		})
	}
	return groups
}

// continuesGroup 判断某行是否可延续其上一行所在的堆栈组(栈帧或 Caused by 段)。
func continuesGroup(msg string) bool {
	return isStackFrame(msg) || isCausedBy(msg)
}

func isExceptionLine(msg string) bool {
	trimmed := strings.TrimSpace(msg)
	if strings.Contains(trimmed, "Exception") ||
		strings.Contains(trimmed, "Error") ||
		strings.Contains(trimmed, "Throwable") {
		return !strings.HasPrefix(trimmed, "at ") &&
			!strings.HasPrefix(trimmed, "Caused by")
	}
	return false
}

func isStackFrame(msg string) bool {
	return strings.HasPrefix(strings.TrimSpace(msg), "at ")
}

func isCausedBy(msg string) bool {
	return strings.HasPrefix(strings.TrimSpace(msg), "Caused by:")
}