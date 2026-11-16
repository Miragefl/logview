package encoding

import (
	"strings"
	"unicode/utf8"
)

// StageLine 检测器输出行:Text 为按结论转码后的 UTF-8 文本,Enc 恒为
// EncUTF8(未转)或 EncGB18030(已转),不出现 auto/app。
type StageLine struct {
	Text string
	Enc  Encoding
}

// Detector 流式字符集检测器(每文件/每 pod/每 SSH 源一个)。
// 判定规则(spec §2,2026-11-16 实现修订):逐行即时判定,零暂存——
// ASCII 行两种编码字节一致,直接直通(UTF-8);非 ASCII 行一次定夺:
// 非合法 UTF-8 且合法 GB18030 → GB18030 转码;合法 UTF-8(含两可)→ UTF-8
// 优先;两者皆非法 → 放弃(原样 UTF-8,用户可手切)。
// 采样窗口(64KB/1000 行)在实现中删去:判定只依赖当前行、不依赖历史行,
// 窗口暂存只吞 ASCII 行(任何结论下都原样输出),除引入"流结束必须 flush"
// 义务外毫无收益;单行 GBK 恰好全行合法 UTF-8 的误判由手切兜底(spec §7)。
// GB18030 合法性 = 解码输出不含 U+FFFD(x/text 解码器遇非法字节自动替换且不报 err)。
type Detector struct {
	concluded bool
	result    Encoding // NewDetector 起默认 EncUTF8,结案时改写

	one [1]StageLine // 直通路径复用槽,避免每行分配
}

func NewDetector() *Detector {
	return &Detector{result: EncUTF8}
}

// Stage 喂一行(无换行原文),立即返回该行的检测/转码结果:
// 结案前 ASCII 行直通;非 ASCII 行一次定夺;结案后按结论直通。
// 返回切片复用内部槽,消费有效期到下一次 Stage 调用。
func (d *Detector) Stage(line string) []StageLine {
	if d.concluded {
		d.one[0] = d.emit(line)
		return d.one[:]
	}
	if isASCII(line) {
		// ASCII 是 GBK/GB18030/UTF-8 公共子集,无需转码,直通
		d.one[0] = StageLine{Text: line, Enc: EncUTF8}
		return d.one[:]
	}
	// 非 ASCII 行:两编码字节不一致,一次定夺
	switch isUTF8, isGB := utf8.Valid([]byte(line)), validGB18030(line); {
	case !isUTF8 && isGB:
		d.concluded = true
		d.result = EncGB18030
	default: // 合法 UTF-8(含两可,UTF-8 优先)或两者皆非法(放弃)
		d.concluded = true
		d.result = EncUTF8
	}
	d.one[0] = d.emit(line)
	return d.one[:]
}

func (d *Detector) Concluded() bool { return d.concluded }

// Result 检测结论:未结案/放弃/UTF-8 均为 EncUTF8,检出为 EncGB18030。
func (d *Detector) Result() Encoding { return d.result }

// emit 按结案结论输出一行。
func (d *Detector) emit(line string) StageLine {
	if d.result == EncGB18030 {
		return StageLine{Text: Decode(line, EncGB18030), Enc: EncGB18030}
	}
	return StageLine{Text: line, Enc: EncUTF8}
}

// isASCII 全部字节 < 0x80(GBK/GB18030/UTF-8 公共子集,无需转码)。
func isASCII(s string) bool {
	return !strings.ContainsFunc(s, func(r rune) bool { return r >= utf8.RuneSelf })
}

// validGB18030 整行合法 GB18030:解码输出不含 U+FFFD。
func validGB18030(s string) bool {
	return !strings.ContainsRune(Decode(s, EncGB18030), '�')
}
