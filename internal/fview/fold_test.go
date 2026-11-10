package fview

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Miragefl/logview/internal/buffer"
	"github.com/Miragefl/logview/internal/model"
)

// mkLine 构造单行 ParsedLine(Message span 覆盖全文,与 session 兜底行同构)。
func mkLine(seq uint64, text string) *model.ParsedLine {
	pl := &model.ParsedLine{Raw: model.RawLine{Seq: seq, Text: text}}
	pl.InitBase(text)
	pl.SetStdSpan(model.FieldMessage, 0, len(text))
	return pl
}

func push(buf *buffer.RingBuffer, startSeq uint64, texts []string) {
	for i, s := range texts {
		buf.Push(mkLine(startSeq+uint64(i), s))
	}
}

// TestFoldDetectBasic 基本检测:异常行+帧+Caused by 段成组,seq 域正确。
func TestFoldDetectBasic(t *testing.T) {
	d := &foldDetector{}
	buf := buffer.NewRingBuffer(1000)
	push(buf, 1, []string{
		"INFO before",
		"java.lang.NullException: boom",
		"\tat com.a.Foo(Foo.java:1)",
		"\tat com.a.Bar(Bar.java:2)",
		"Caused by: java.lang.IllegalStateException: inner",
		"\tat com.a.Baz(Baz.java:3)",
		"INFO after",
	})
	rs := d.update(buf)
	if len(rs) != 1 {
		t.Fatalf("检出 %d 组,期望 1: %+v", len(rs), rs)
	}
	r := rs[0]
	if r.StartSeq != 2 || r.EndSeq != 6 {
		t.Errorf("组范围 [%d,%d],期望 [2,6](leader+帧+Caused by 段)", r.StartSeq, r.EndSeq)
	}
	if !strings.Contains(r.Leader, "NullException") {
		t.Errorf("Leader = %q,应含首行异常", r.Leader)
	}
}

// TestFoldUpdateNoDup 跨轮更新组不重复、不漂移。
func TestFoldUpdateNoDup(t *testing.T) {
	d := &foldDetector{}
	buf := buffer.NewRingBuffer(1000)
	push(buf, 1, []string{"INFO a", "java.io.IOException: x", "\tat c.D(E.java:1)"})
	if rs := d.update(buf); len(rs) != 1 {
		t.Fatalf("首轮检出 %d 组", len(rs))
	}
	push(buf, 4, []string{"INFO tail"})
	rs := d.update(buf)
	if len(rs) != 1 || rs[0].StartSeq != 2 || rs[0].EndSeq != 3 {
		t.Fatalf("二轮组漂移/重复: %+v", rs)
	}
}

// TestFoldEvictionKeepsFinalGroups 缓冲裁头后终局组保留(seq 域不受索引漂移影响)。
func TestFoldEvictionKeepsFinalGroups(t *testing.T) {
	d := &foldDetector{}
	buf := buffer.NewRingBuffer(8) // 小缓冲逼裁头
	push(buf, 1, []string{"INFO 1", "java.lang.Exception: a", "\tat a.B(B.java:1)"})
	if rs := d.update(buf); len(rs) != 1 {
		t.Fatalf("初检失败: %+v", rs)
	}
	more := make([]string, 10)
	for i := range more {
		more[i] = "plain " + strconv.Itoa(i)
	}
	push(buf, 4, more) // seq 4..13,容量 8 → 组(seq 2-3)被裁出缓冲
	rs := d.update(buf)
	if len(rs) != 1 || rs[0].StartSeq != 2 || rs[0].EndSeq != 3 {
		t.Fatalf("裁头后终局组丢失: %+v", rs)
	}
}

// TestFoldGiantGroupAcrossWindow 巨组跨检测窗口不断裂(小窗注入+分批推进,
// 模拟 flush 节奏下回退重扫保证连续性)。
func TestFoldGiantGroupAcrossWindow(t *testing.T) {
	d := &foldDetector{win: 6}
	buf := buffer.NewRingBuffer(1000)
	texts := []string{"java.lang.Exception: giant"}
	for i := 0; i < 10; i++ {
		texts = append(texts, "\tat x.Y(Z.java:"+strconv.Itoa(i)+")")
	}
	// 组共 12 行 > 窗 6;分批推送逐轮 update(模拟每批 flush)
	var seq uint64 = 1
	for i := 0; i < len(texts); i += 3 {
		end := min(i+3, len(texts))
		push(buf, seq, texts[i:end])
		seq += uint64(end - i)
		d.update(buf)
	}
	rs := d.update(buf)
	if len(rs) != 1 || rs[0].StartSeq != 1 || rs[0].EndSeq != 11 {
		t.Fatalf("巨组断裂: %+v", rs)
	}
}

// TestShouldFoldScan 节流判定(Final #4):满 500ms 或增量 ≥8192 才重扫。
func TestShouldFoldScan(t *testing.T) {
	now := time.Now()
	if !shouldFoldScan(now.Add(-time.Second), now, 0) {
		t.Error("间隔 ≥500ms 应触发扫描")
	}
	if shouldFoldScan(now, now, 100) {
		t.Error("间隔 0 且增量 100 不应扫描")
	}
	if !shouldFoldScan(now, now, 8192) {
		t.Error("增量 ≥8192 应立即扫描")
	}
	if shouldFoldScan(now.Add(-time.Millisecond*100), now, 100) {
		t.Error("间隔 100ms 且增量 100 不应扫描")
	}
}

// TestFoldTwoGroupsAcrossWindow 双组场景(Final Review #2):G1 跨标准窗口起点、
// G2 完整落窗内——旧实现只回退最后一组(G2),G1 leader 出窗整组丢失。
func TestFoldTwoGroupsAcrossWindow(t *testing.T) {
	d := &foldDetector{win: 6}
	buf := buffer.NewRingBuffer(1000)
	// G1 = seq1 leader + seq2-8 帧;G2 = seq9 leader + seq10 帧;seq11-15 普通行
	texts := []string{"java.lang.Exception: first"}
	for i := 0; i < 7; i++ {
		texts = append(texts, "\tat a.B"+strconv.Itoa(i)+"(C.java:1)")
	}
	texts = append(texts, "java.lang.Exception: second", "\tat d.E(F.java:1)")
	for i := 0; i < 5; i++ {
		texts = append(texts, "plain "+strconv.Itoa(i))
	}
	var seq uint64 = 1
	for i := 0; i < len(texts); i += 3 {
		end := min(i+3, len(texts))
		push(buf, seq, texts[i:end])
		seq += uint64(end - i)
		d.update(buf)
	}
	rs := d.update(buf)
	if len(rs) != 2 {
		t.Fatalf("应检出 2 组,得到 %d 组: %+v", len(rs), rs)
	}
	if rs[0].StartSeq != 1 || rs[0].EndSeq != 8 {
		t.Errorf("G1 = [%d,%d],期望 [1,8]", rs[0].StartSeq, rs[0].EndSeq)
	}
	if rs[1].StartSeq != 9 || rs[1].EndSeq != 10 {
		t.Errorf("G2 = [%d,%d],期望 [9,10]", rs[1].StartSeq, rs[1].EndSeq)
	}
}
