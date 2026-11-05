package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// GUI 启动环境模拟:launchd 只给系统目录,brew 目录缺失,
// augment 后必须能在补入目录里找到可执行文件(修"未找到 frpc"的回归测试)。
func TestAugmentPATHFindsBrewBin(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "frpc")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")
	augmentPATHWith([]string{dir})

	p, err := exec.LookPath("frpc")
	if err != nil {
		t.Fatalf("augment 后仍找不到 frpc: %v", err)
	}
	if p != bin {
		t.Fatalf("找到的不是补入目录里的 frpc: %s", p)
	}
	// 尾部追加,不抢占已有 PATH 优先级(系统命令仍先于补入目录)
	if !strings.HasSuffix(os.Getenv("PATH"), dir) {
		t.Fatalf("补入目录应在 PATH 尾部,当前: %s", os.Getenv("PATH"))
	}
}

// 已在 PATH 的目录不得重复追加;不存在的目录直接跳过。
func TestAugmentPATHSkipsExistingAndMissing(t *testing.T) {
	dir := t.TempDir()
	orig := "/usr/bin:/bin:" + dir
	t.Setenv("PATH", orig)
	augmentPATHWith([]string{dir, "/definitely/not/exist/bin"})
	if got := os.Getenv("PATH"); got != orig {
		t.Fatalf("PATH 不应变化,期望 %q 实际 %q", orig, got)
	}
}
