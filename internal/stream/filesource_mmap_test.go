//go:build !windows

package stream

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/justfun/logview/internal/model"
)

// drainLines 收流至通道关闭(超时兜底)。
func drainLines(t *testing.T, ch <-chan []model.RawLine) []string {
	t.Helper()
	var got []string
	timeout := time.After(10 * time.Second)
collect:
	for {
		select {
		case batch, ok := <-ch:
			if !ok {
				break collect
			}
			for _, l := range batch {
				got = append(got, l.Text)
			}
		case <-timeout:
			t.Fatal("stream drain timeout")
		}
	}
	return got
}

// mmap 零拷贝读回归:阈值调小强制触发,行为须与块式完全一致——
// 行数/内容/末行无换行/多行边界。数据本体出堆(unsafe 子串指向映射区),
// 内容正确性是本测试钉死的不变量。
func TestFileSourceMmapRead(t *testing.T) {
	old := mmapThreshold
	mmapThreshold = 16 // 字节级阈值,任何非空文件都触发
	defer func() { mmapThreshold = old }()

	dir := t.TempDir()
	p := filepath.Join(dir, "a.log")
	content := "l1\nl2\nl3\ntail-no-nl"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	src := NewFileSource([]string{p}).WithTailLines(100) // 行数不足 → 全量 → mmap
	ch, err := src.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := drainLines(t, ch)
	want := []string{"l1", "l2", "l3", "tail-no-nl"}
	if len(got) != len(want) {
		t.Fatalf("mmap 读行数 = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 行 = %q, want %q", i, got[i], want[i])
		}
	}
}

// 同一文件块式/mmap 双路径结果一致性(阈值恢复后走块式)。
func TestFileSourceMmapChunkedEquivalence(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "eq.log")
	var body []byte
	for i := 0; i < 3000; i++ { // 跨多块的行量
		body = append(body, []byte("2026-10-12 10:00:00.000 [main] INFO c.x.Svc - line content padded 0123456789\n")...)
	}
	body = append(body, []byte("final-no-nl")...)
	if err := os.WriteFile(p, body, 0o644); err != nil {
		t.Fatal(err)
	}

	readOnce := func(forceMmap bool) []string {
		old := mmapThreshold
		if forceMmap {
			mmapThreshold = 16
		} else {
			mmapThreshold = 1 << 40
		}
		defer func() { mmapThreshold = old }()
		src := NewFileSource([]string{p}).WithTailLines(1 << 30) // 行数不足 → 全量
		ch, err := src.Start(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return drainLines(t, ch)
	}

	chunked := readOnce(false)
	mmapped := readOnce(true)
	if len(chunked) == 0 || len(chunked) != len(mmapped) {
		t.Fatalf("行数不一致: chunked=%d mmap=%d", len(chunked), len(mmapped))
	}
	for i := range chunked {
		if chunked[i] != mmapped[i] {
			t.Fatalf("第 %d 行不一致:\n chunked=%q\n mmap   =%q", i, chunked[i], mmapped[i])
		}
	}
	if chunked[len(chunked)-1] != "final-no-nl" {
		t.Fatalf("末行应无换行保留: %q", chunked[len(chunked)-1])
	}
}
