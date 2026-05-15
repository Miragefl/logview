package stacktrace

import (
	"testing"

	"github.com/justfun/logview/internal/model"
)

func TestDetectStackTrace(t *testing.T) {
	lines := []*model.ParsedLine{
		{Message: "something happened"},
		{Message: "java.lang.NullPointerException"},
		{Message: "  at com.example.App.doThing(App.java:42)"},
		{Message: "  at com.example.App.run(App.java:10)"},
		{Message: "Caused by: java.lang.IllegalArgumentException"},
		{Message: "  at com.example.Util.check(Util.java:5)"},
		{Message: "normal log again"},
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
		{Message: "hello"},
		{Message: "world"},
	}
	groups := Detect(lines)
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
}