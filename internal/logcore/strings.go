package logcore

import "strings"

// ContainsIgnoreCase 子串忽略大小写包含判断。
// ASCII 走零分配字节比较(日志场景压倒性路径,替代每行两次 ToLower 的全行分配);
// 含非 ASCII 字节回落 Unicode 语义(strings.ToLower 折叠),行为与原实现一致。
// （自 internal/tui/app.go 迁入：searchquery 依赖，fview 桌面端共享单源）
func ContainsIgnoreCase(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(s) < len(sub) {
		return false
	}
	if hasNonASCII(s) || hasNonASCII(sub) {
		return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
	}
	n := len(s) - len(sub)
	f0 := lowerByte(sub[0])
	for i := 0; i <= n; i++ {
		if lowerByte(s[i]) != f0 {
			continue // 首字节折叠不匹配,滑窗剪枝
		}
		j := 1
		for ; j < len(sub); j++ {
			if lowerByte(s[i+j]) != lowerByte(sub[j]) {
				break
			}
		}
		if j == len(sub) {
			return true
		}
	}
	return false
}

func hasNonASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return true
		}
	}
	return false
}

func lowerByte(c byte) byte {
	if 'A' <= c && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}
