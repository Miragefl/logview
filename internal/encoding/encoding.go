// Package encoding 提供日志行字符集的转码与检测(GBK/GB18030 自动检测 + 手动切换)。
// 行级编码标记(Encoding)由 stream 层检测转码时打出;tui 层按当前生效编码校正到达行
// 与 buffer 存量行(Reencode 统一"取回原始字节再按目标编码解释"模型)。
package encoding

import (
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// Encoding 行/流级字符集认知。零值 EncAuto 双语义:流级=跟随检测;行级=程序生成行
// ([logview] 提示/错误行),检测与重转全部旁路,防止中文提示被转码转坏。
type Encoding uint8

const (
	EncAuto Encoding = iota
	EncUTF8
	EncGBK
	EncGB18030
	EncApp
)

// ParseEncoding 解析用户可输入的编码名(弹窗/记忆文件),未知返回 false。
func ParseEncoding(s string) (Encoding, bool) {
	switch s {
	case "auto":
		return EncAuto, true
	case "utf-8":
		return EncUTF8, true
	case "gbk":
		return EncGBK, true
	case "gb18030":
		return EncGB18030, true
	}
	return EncAuto, false
}

func (e Encoding) String() string {
	switch e {
	case EncUTF8:
		return "utf-8"
	case EncGBK:
		return "gbk"
	case EncGB18030:
		return "gb18030"
	case EncApp:
		return "app"
	}
	return "auto"
}

// Decode 将 GBK/GB18030 原始字节串解为 UTF-8 文本。x/text 解码器对非法字节自动
// 替换 U+FFFD 且不返回错误,本函数直接采用该替换语义;utf-8/auto/app 原样返回。
func Decode(s string, enc Encoding) string {
	switch enc {
	case EncGBK, EncGB18030:
	default:
		return s
	}
	codec := simplifiedchinese.GBK
	if enc == EncGB18030 {
		codec = simplifiedchinese.GB18030
	}
	out, _ := codec.NewDecoder().String(s)
	return out
}

// Encode 将 UTF-8 文本编码回 GBK/GB18030 原始字节串(重转时"取回原始字节"用)。
// x/text 编码器整串遇编码外 rune(如 emoji)会返回空串+错误,此处逐 rune 容错,
// 不支持的字符替换 U+FFFD——代价是该字符再切回时精度丢失(spec §7 已文档化)。
func Encode(s string, enc Encoding) string {
	switch enc {
	case EncGBK, EncGB18030:
	default:
		return s
	}
	codec := simplifiedchinese.GBK
	if enc == EncGB18030 {
		codec = simplifiedchinese.GB18030
	}
	if out, err := codec.NewEncoder().String(s); err == nil {
		return out
	}
	var b strings.Builder
	for _, r := range s {
		if or, err := codec.NewEncoder().String(string(r)); err == nil {
			b.WriteString(or)
		} else {
			b.WriteRune('�')
		}
	}
	return b.String()
}

// Reencode 统一重转模型:orig = (from==utf8 ? s : Encode(s, from)),即取回原始字节;
// to 为 utf8/auto 返回 orig(auto 语义=交由调用方重检测);否则 Decode(orig, to)。
// from==EncApp(程序行)或 from==to 时原样返回。
func Reencode(s string, from, to Encoding) string {
	if from == EncApp || from == to {
		return s
	}
	orig := s
	if from != EncUTF8 {
		orig = Encode(s, from)
	}
	switch to {
	case EncUTF8, EncAuto:
		return orig
	}
	return Decode(orig, to)
}
