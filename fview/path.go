package main

import (
	"os"
	"path/filepath"
	"strings"
)

// augmentPATH 补齐 GUI 启动环境缺失的可执行目录。
// macOS 从 Finder/Dock 启动时 launchd 只给 /usr/bin:/bin:/usr/sbin:/sbin,
// brew 安装的 frpc/kubectl(/opt/homebrew/bin、/usr/local/bin)全找不到,
// 建隧道报"未找到 frpc"。追加到 PATH 尾部,不抢占系统命令优先级
// (终端启动时 PATH 已含这些目录,重复调用无副作用)。
func augmentPATH() {
	augmentPATHWith([]string{"/opt/homebrew/bin", "/usr/local/bin"})
}

func augmentPATHWith(candidates []string) {
	parts := filepath.SplitList(os.Getenv("PATH"))
	seen := make(map[string]bool, len(parts))
	for _, p := range parts {
		seen[p] = true
	}
	var add []string
	for _, d := range candidates {
		if !seen[d] {
			if fi, err := os.Stat(d); err == nil && fi.IsDir() {
				add = append(add, d)
			}
		}
	}
	if len(add) == 0 {
		return
	}
	os.Setenv("PATH", strings.Join(append(parts, add...), string(filepath.ListSeparator)))
}
