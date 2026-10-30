// 系统字体枚举：按平台扫字体目录树，文件名（去扩展）即字体显示名。
// 不解析 name table——文件名近似足够（KISS），跨平台 darwin/windows/linux 三套目录。
package fview

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// fontExts 可识别的字体文件扩展名。
var fontExts = map[string]bool{".ttf": true, ".otf": true, ".ttc": true}

// SystemFonts 返回当前系统可用字体名列表（去重、字母序）。
func SystemFonts() []string {
	return fontsInDirs(platformFontDirs())
}

// platformFontDirs 各平台字体目录（含用户级）。
func platformFontDirs() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/System/Library/Fonts",
			"/Library/Fonts",
			filepath.Join(home, "Library", "Fonts"),
		}
	case "windows":
		return []string{`C:\Windows\Fonts`}
	default: // linux
		return []string{
			filepath.Join(home, ".fonts"),
			"/usr/share/fonts",
			"/usr/local/share/fonts",
		}
	}
}

// fontsInDirs 递归扫描目录树收集字体名（去扩展、去重、升序；目录缺失静默跳过）。
func fontsInDirs(dirs []string) []string {
	set := make(map[string]bool)
	for _, dir := range dirs {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil // 缺失/不可读跳过
			}
			if fontExts[strings.ToLower(filepath.Ext(path))] {
				set[strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))] = true
			}
			return nil
		})
	}
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
