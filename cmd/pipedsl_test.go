package cmd

import (
	"reflect"
	"testing"
)

func TestSplitPipeSegments(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"./a.log | grep 123", []string{"./a.log ", " grep 123"}},
		{`./a.log | grep -E "ERROR|WARN"`, []string{"./a.log ", ` grep -E "ERROR|WARN"`}},
		{"'a|b.log' | grep x", []string{"'a|b.log' ", " grep x"}},
		{"cat a.gz | gunzip | grep 123", []string{"cat a.gz ", " gunzip ", " grep 123"}},
		{`grep \| x`, []string{`grep \| x`}}, // 转义 | 不拆
	}
	for _, c := range cases {
		got, err := splitPipeSegments(c.in)
		if err != nil {
			t.Errorf("splitPipeSegments(%q) err: %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitPipeSegments(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if _, err := splitPipeSegments(`grep "unclosed`); err == nil {
		t.Error("未闭合引号应报错")
	}
}

func TestSplitShellArgs(t *testing.T) {
	got, err := splitShellArgs(`tail -100f ./park.log`)
	if err != nil || !reflect.DeepEqual(got, []string{"tail", "-100f", "./park.log"}) {
		t.Fatalf("splitShellArgs = %v err=%v", got, err)
	}
	got, err = splitShellArgs(`file "my app.log"`)
	if err != nil || !reflect.DeepEqual(got, []string{"file", "my app.log"}) {
		t.Fatalf("引号剥离 = %v err=%v", got, err)
	}
	if _, err := splitShellArgs(`'unclosed`); err == nil {
		t.Error("未闭合引号应报错")
	}
}
