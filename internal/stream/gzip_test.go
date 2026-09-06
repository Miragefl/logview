package stream

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// writeGzip 在 path 写入 content 的 gzip 压缩,返回错误。
func writeGzip(t *testing.T, path, content string) {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
}

func readAll(t *testing.T, r io.Reader) string {
	t.Helper()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return string(data)
}

func TestOpenMaybeGzip(t *testing.T) {
	dir := t.TempDir()

	gzPath := filepath.Join(dir, "a.log.gz")
	writeGzip(t, gzPath, "hello\nworld\n")
	r, closer, isGz, err := openMaybeGzip(gzPath)
	if err != nil || !isGz {
		t.Fatalf("gz 文件应嗅探成功: isGz=%v err=%v", isGz, err)
	}
	if got := readAll(t, r); got != "hello\nworld\n" {
		t.Fatalf("解压内容 = %q", got)
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// 非 gz 后缀但内容是 gzip:仍解压(magic 与文件名无关)
	renamed := filepath.Join(dir, "plain-name.log")
	os.Rename(gzPath, renamed)
	r, closer, isGz, err = openMaybeGzip(renamed)
	if err != nil || !isGz {
		t.Fatalf("改名归档应嗅探成功: isGz=%v err=%v", isGz, err)
	}
	readAll(t, r)
	closer.Close()

	// 普通文本:原样
	plain := filepath.Join(dir, "b.log")
	os.WriteFile(plain, []byte("x\n"), 0644)
	r, closer, isGz, err = openMaybeGzip(plain)
	if err != nil || isGz {
		t.Fatalf("普通文件不应判为 gz: isGz=%v err=%v", isGz, err)
	}
	if got := readAll(t, r); got != "x\n" {
		t.Fatalf("普通内容 = %q", got)
	}
	closer.Close()

	// 空文件:Peek 不足,按普通文本处理
	empty := filepath.Join(dir, "c.log")
	os.WriteFile(empty, nil, 0644)
	_, closer, isGz, err = openMaybeGzip(empty)
	if err != nil || isGz {
		t.Fatalf("空文件应按普通处理: isGz=%v err=%v", isGz, err)
	}
	closer.Close()
}
