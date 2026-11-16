package stream

// 编码检测接线测试:各源读行过 Detector(stream 层 auto 检测)后行级 Enc 打标。
// 用例集中在独立文件(plan 原定追加 filesource/pipe 两处,主题内聚一起放)。

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Miragefl/logview/internal/encoding"
	"github.com/Miragefl/logview/internal/model"
)

func collectRaws(t *testing.T, ch <-chan []model.RawLine) []model.RawLine {
	t.Helper()
	var raws []model.RawLine
	timeout := time.After(2 * time.Second)
	for {
		select {
		case batch, ok := <-ch:
			if !ok {
				return raws
			}
			raws = append(raws, batch...)
		case <-timeout:
			t.Fatalf("timed out, got %d raws", len(raws))
			return nil
		}
	}
}

func startCollect(t *testing.T, src interface {
	Start(context.Context) (<-chan []model.RawLine, error)
}) []model.RawLine {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	return collectRaws(t, ch)
}

func TestFileSourceGBKDetected(t *testing.T) {
	fpath := filepath.Join(t.TempDir(), "gbk.log")
	// "中文" GBK = d6d0cec4
	if err := os.WriteFile(fpath, []byte("\xd6\xd0\xce\xc4 hi\n\xd6\xd0\xce\xc4 again\n"), 0644); err != nil {
		t.Fatal(err)
	}
	raws := startCollect(t, NewFileSource([]string{fpath}))
	if len(raws) != 2 {
		t.Fatalf("got %d raws, want 2", len(raws))
	}
	if raws[0].Text != "中文 hi" || raws[0].Enc != encoding.EncGB18030 {
		t.Errorf("raw[0] = %q,%v want 中文 hi,gb18030", raws[0].Text, raws[0].Enc)
	}
	if raws[1].Text != "中文 again" || raws[1].Enc != encoding.EncGB18030 {
		t.Errorf("raw[1] = %q,%v want 中文 again,gb18030", raws[1].Text, raws[1].Enc)
	}
}

func TestFileSourceUTF8ChineseUntouched(t *testing.T) {
	fpath := filepath.Join(t.TempDir(), "utf8.log")
	if err := os.WriteFile(fpath, []byte("中文 logback\n第二行\n"), 0644); err != nil {
		t.Fatal(err)
	}
	raws := startCollect(t, NewFileSource([]string{fpath}))
	if len(raws) != 2 {
		t.Fatalf("got %d raws, want 2", len(raws))
	}
	for i, want := range []string{"中文 logback", "第二行"} {
		if raws[i].Text != want || raws[i].Enc != encoding.EncUTF8 {
			t.Errorf("raw[%d] = %q,%v want %q,utf-8", i, raws[i].Text, raws[i].Enc, want)
		}
	}
}

func TestFileSourcePureASCIIUntouched(t *testing.T) {
	fpath := filepath.Join(t.TempDir(), "ascii.log")
	content := "2026-11-16 10:00:00 INFO [main] boot ok\nshutdown\n"
	if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	raws := startCollect(t, NewFileSource([]string{fpath}))
	if len(raws) != 2 {
		t.Fatalf("got %d raws, want 2", len(raws))
	}
	if raws[0].Text != "2026-11-16 10:00:00 INFO [main] boot ok" || raws[0].Enc != encoding.EncUTF8 {
		t.Errorf("ASCII 行被改动: %q,%v", raws[0].Text, raws[0].Enc)
	}
}

func TestFileSourceProgramLineBypassed(t *testing.T) {
	// 不存在路径:错误提示行是程序生成 UTF-8,Enc 必须保持零值旁路
	raws := startCollect(t, NewFileSource([]string{filepath.Join(t.TempDir(), "nope.log")}))
	if len(raws) != 1 {
		t.Fatalf("got %d raws, want 1 错误提示行", len(raws))
	}
	if !strings.HasPrefix(raws[0].Text, "[logview]") {
		t.Errorf("错误行 = %q, want [logview] 前缀", raws[0].Text)
	}
	if raws[0].Enc != encoding.EncAuto {
		t.Errorf("程序行 Enc = %v, want auto(零值旁路)", raws[0].Enc)
	}
}

func TestPipeSourceGBK(t *testing.T) {
	r := strings.NewReader("\xd6\xd0\xce\xc4 from pipe\n\xc8\xd5\xd6\xbe line2\n")
	raws := startCollect(t, NewPipeSource(r))
	if len(raws) != 2 {
		t.Fatalf("got %d raws, want 2", len(raws))
	}
	if raws[0].Text != "中文 from pipe" || raws[0].Enc != encoding.EncGB18030 {
		t.Errorf("raw[0] = %q,%v want 中文 from pipe,gb18030", raws[0].Text, raws[0].Enc)
	}
	if raws[1].Text != "日志 line2" || raws[1].Enc != encoding.EncGB18030 {
		t.Errorf("raw[1] = %q,%v want 日志 line2,gb18030", raws[1].Text, raws[1].Enc)
	}
}
