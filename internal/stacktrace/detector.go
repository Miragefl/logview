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
		if isExceptionLine(lines[i].Message) {
			start := i
			leader := lines[i].Message
			i++
			for i < len(lines) && isStackFrame(lines[i].Message) {
				i++
			}
			for i < len(lines) && isCausedBy(lines[i].Message) {
				i++
				for i < len(lines) && isStackFrame(lines[i].Message) {
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