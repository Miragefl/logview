package stream

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CountLines:常规/末行无换行/空文件/gz 报错/跨 1MB 块边界
func TestCountLines(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.log")

	if err := os.WriteFile(p, []byte("a\nb\nc\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if n, err := CountLines(p); err != nil || n != 3 {
		t.Fatalf("CountLines = %d, %v; want 3", n, err)
	}

	if err := os.WriteFile(p, []byte("a\nb\nc"), 0644); err != nil { // 末行无换行符
		t.Fatal(err)
	}
	if n, err := CountLines(p); err != nil || n != 3 {
		t.Fatalf("CountLines(末行无换行) = %d, %v; want 3", n, err)
	}

	if err := os.WriteFile(p, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if n, err := CountLines(p); err != nil || n != 0 {
		t.Fatalf("CountLines(空文件) = %d, %v; want 0", n, err)
	}

	gz := filepath.Join(dir, "a.log.gz")
	writeGzip(t, gz, "x\ny\n")
	if _, err := CountLines(gz); err == nil {
		t.Fatal("gz 归档应报错(不支持 --all)")
	}
}

// 跨 1MB 读取块的行数统计正确性
func TestCountLinesAcrossBlocks(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "big.log")
	// 每行 2 字节("x\n"),70 万行 = 1.4MB,跨 1MB 块边界
	const rows = 700000
	if err := os.WriteFile(p, []byte(strings.Repeat("x\n", rows)), 0644); err != nil {
		t.Fatal(err)
	}
	if n, err := CountLines(p); err != nil || n != rows {
		t.Fatalf("CountLines = %d, %v; want %d", n, err, rows)
	}
}

// seekTailLines:定位最后 N 行起点(含末行无换行、行数不足回落 0)
func TestSeekTailLines(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.log")
	if err := os.WriteFile(p, []byte("aa\nbb\ncc\ndd\n"), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// 最后 2 行 "cc\ndd\n" 起点在第 6 字节
	if got := seekTailLines(f, 2); got != 6 {
		t.Fatalf("seekTailLines(2) = %d, want 6", got)
	}
	// 只要最后 1 行
	if got := seekTailLines(f, 1); got != 9 {
		t.Fatalf("seekTailLines(1) = %d, want 9", got)
	}
	// 行数不足:从头读
	if got := seekTailLines(f, 10); got != 0 {
		t.Fatalf("seekTailLines(10) = %d, want 0(行数不足回落)", got)
	}
}
