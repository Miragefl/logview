package cmd

import (
	"fmt"
	"strings"
)

// splitPipeSegments 把管道 DSL 参数按裸 | 分段:单/双引号内的 | 不拆,\ 转义。
// 段保留原文(含引号与空白),供首段 token 化或后续段原样拼 sh -c。
func splitPipeSegments(s string) ([]string, error) {
	var segs []string
	var cur strings.Builder
	var quote rune
	escaped := false
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			cur.WriteRune(r)
			escaped = true
		case quote != 0:
			cur.WriteRune(r)
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			cur.WriteRune(r)
			quote = r
		case r == '|':
			segs = append(segs, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("管道表达式引号未闭合: %s", s)
	}
	if escaped {
		return nil, fmt.Errorf("管道表达式以转义符结尾: %s", s)
	}
	return append(segs, cur.String()), nil
}

// splitShellArgs shell 风格 token 化(空格分隔,单/双引号成组并剥壳,\ 转义下一字符)。
func splitShellArgs(s string) ([]string, error) {
	var args []string
	var cur strings.Builder
	var quote rune
	inToken := false
	escaped := false
	flush := func() {
		if inToken {
			args = append(args, cur.String())
			cur.Reset()
			inToken = false
		}
	}
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
			inToken = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			inToken = true
		case r == ' ' || r == '\t':
			flush()
		default:
			cur.WriteRune(r)
			inToken = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("参数引号未闭合: %s", s)
	}
	if escaped {
		return nil, fmt.Errorf("参数以转义符结尾: %s", s)
	}
	flush()
	return args, nil
}
