package stacktrace

import (
	"testing"

	"github.com/justfun/logview/internal/model"
)

func TestDetectStackTrace(t *testing.T) {
	lines := []*model.ParsedLine{
		model.TestLine("something happened"),
		model.TestLine("java.lang.NullPointerException"),
		model.TestLine("  at com.example.App.doThing(App.java:42)"),
		model.TestLine("  at com.example.App.run(App.java:10)"),
		model.TestLine("Caused by: java.lang.IllegalArgumentException"),
		model.TestLine("  at com.example.Util.check(Util.java:5)"),
		model.TestLine("normal log again"),
	}

	groups := Detect(lines)
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	g := groups[0]
	if g.Start != 1 {
		t.Errorf("Start = %d, want 1", g.Start)
	}
	if g.End != 5 {
		t.Errorf("End = %d, want 5", g.End)
	}
	if g.Leader != "java.lang.NullPointerException" {
		t.Errorf("Leader = %q", g.Leader)
	}
}

func TestNoStackTrace(t *testing.T) {
	lines := []*model.ParsedLine{
		model.TestLine("hello"),
		model.TestLine("world"),
	}
	groups := Detect(lines)
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
}

// 分批增量维护(DetectAppend)与全量一次 Detect 结果必须一致:
// 覆盖跨批延续组、Caused by 段、终结组、批尾截断等路径。overlap > batch 满足契约。
func TestDetectAppendMatchesDetect(t *testing.T) {
	all := []*model.ParsedLine{
		model.TestLine("plain 1"),
		model.TestLine("java.lang.NullPointerException"),
		model.TestLine("  at com.example.App.doThing(App.java:42)"),
		model.TestLine("  at com.example.App.run(App.java:10)"),
		model.TestLine("Caused by: java.lang.IllegalArgumentException"),
		model.TestLine("  at com.example.Util.check(Util.java:5)"),
		model.TestLine("plain 2"),
		model.TestLine("java.lang.IllegalStateException: boom"),
		model.TestLine("  at com.example.Other.main(Other.java:7)"),
		model.TestLine("plain 3"),
	}
	const batch, overlap = 2, 6
	var groups []Group
	for i := 0; i < len(all); i += batch {
		end := min(i+batch, len(all))
		groups = DetectAppend(all[:end], groups, overlap)
	}
	want := Detect(all)
	if len(groups) != len(want) {
		t.Fatalf("got %d groups, want %d", len(groups), len(want))
	}
	for i := range want {
		if groups[i] != want[i] {
			t.Errorf("group[%d] = %+v, want %+v", i, groups[i], want[i])
		}
	}
}