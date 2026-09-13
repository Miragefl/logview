package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/justfun/logview/internal/stream"
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

// pipeDSLArg:恰好一个含 | 的位置参数(非 flag)且无子命令时返回之。
func TestPipeDSLArg(t *testing.T) {
	if got := pipeDSLArg([]string{"./a.log | grep 123"}); got != "./a.log | grep 123" {
		t.Fatalf("DSL 参数 = %q", got)
	}
	if got := pipeDSLArg([]string{"./a.log", "|", "grep"}); got != "" {
		t.Fatalf("拆成多个参数不应触发, got %q", got)
	}
	if got := pipeDSLArg([]string{"./plain.log"}); got != "" {
		t.Fatalf("无 | 不触发, got %q", got)
	}
}

// 首段源构造:裸路径→tail;tail -100f x→TailSource 行数;file x→FileSource。
func TestPipeDSLFirstSegment(t *testing.T) {
	src, err := firstSegmentSource([]string{"./a.log"}, 5000)
	if err != nil || src == nil {
		t.Fatalf("裸路径应构造源: %v", err)
	}
	src, err = firstSegmentSource([]string{"tail", "-100f", "./a.log"}, 5000)
	if err != nil {
		t.Fatalf("tail 子命令形式: %v", err)
	}
	if _, err = firstSegmentSource([]string{"k8s", "deploy/x"}, 5000); err == nil {
		t.Fatal("不支持的首段子命令应报错")
	}
}

// 集成:真 sh + grep 过滤 fixture,pipeCmdSource 收过滤行;命令退出(grep 正常 EOF)后通道关闭。
func TestPipeCmdSource(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.log")
	os.WriteFile(f, []byte("keep-1\nskip\nkeep-2\n"), 0644)

	src := newPipeCmdSourceCmd(stream.NewFileSource([]string{f}), "grep keep")
	ch, err := src.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	timeout := time.After(5 * time.Second)
	for {
		select {
		case raw, ok := <-ch:
			if !ok {
				goto done
			}
			texts = append(texts, raw.Text)
		case <-timeout:
			t.Fatalf("超时, got %v", texts)
		}
	}
done:
	if len(texts) != 2 || texts[0] != "keep-1" || texts[1] != "keep-2" {
		t.Fatalf("过滤行 = %v, want [keep-1 keep-2]", texts)
	}
	src.Cleanup()
}

// 命令非零退出:收退出提示行。
func TestPipeCmdSourceExitLine(t *testing.T) {
	src := newPipeCmdSourceCmd(stream.NewFileSource(nil), "exit 3")
	ch, err := src.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	timeout := time.After(5 * time.Second)
	for {
		select {
		case raw, ok := <-ch:
			if !ok {
				goto done
			}
			texts = append(texts, raw.Text)
		case <-timeout:
			t.Fatalf("超时, got %v", texts)
		}
	}
done:
	if len(texts) == 0 || !strings.Contains(texts[len(texts)-1], "管道命令退出") {
		t.Fatalf("非零退出应收提示行, got %v", texts)
	}
	src.Cleanup()
}

// 实时流证明(pty 桥修复目标):tail -f 首段 + grep(不带 --line-buffered)。
// pty 下 grep 判 stdout 为 TTY → 行缓冲,追加行 ~0.5s 实时到达;
// 若 stdout 是普通管道(全缓冲),match-live 须攒满 4-64KB 才出,3s 必超时。
func TestPipeCmdSourceLiveStream(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "live.log")
	if err := os.WriteFile(f, []byte("match-0\nnomatch\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	first := stream.NewTailSource([]string{f}, 10) // follow 语义:取尾 10 行 + 100ms 轮询追加
	src := newPipeCmdSourceCmd(first, "grep match")
	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}

	go func() { // 0.5s 后追加命中行,模拟 live tail 新日志
		time.Sleep(500 * time.Millisecond)
		af, err := os.OpenFile(f, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		defer af.Close()
		fmt.Fprintln(af, "match-live")
	}()

	deadline := time.After(3 * time.Second)
	for {
		select {
		case raw, ok := <-ch:
			if !ok {
				t.Fatal("通道提前关闭:tail -f 不 EOF,grep 应持续运行(全缓冲未流出即 EOF 退出也是病态)")
			}
			if raw.Text == "match-live" {
				return // 追加行实时到达,通过
			}
		case <-deadline:
			t.Fatal("3s 内未见 match-live:grep stdout 疑似全缓冲,pty 桥未生效")
		}
	}
}
