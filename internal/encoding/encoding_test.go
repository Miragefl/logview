package encoding

import "testing"

// fixture 硬编码字节,不用被测函数自造期望值:
// "中文" GBK/GB18030 bytes = d6 d0 ce c4;"日志" = c8 d5 d6 be。
const (
	gbkZhongWen = "\xd6\xd0\xce\xc4"
	gbkRiZhi    = "\xc8\xd5\xd6\xbe"
)

func TestDecodeGBK(t *testing.T) {
	if got := Decode(gbkZhongWen, EncGBK); got != "中文" {
		t.Errorf("Decode(GBK 中文) = %q, want 中文", got)
	}
}

func TestDecodeGB18030(t *testing.T) {
	if got := Decode(gbkZhongWen, EncGB18030); got != "中文" {
		t.Errorf("Decode(GB18030 中文) = %q, want 中文", got)
	}
}

// 非法尾字节:x/text 解码器自动替换 U+FFFD,函数必须稳定返回替换结果而非空串。
func TestDecodeIllegalBytesToReplacement(t *testing.T) {
	if got := Decode("\xd6\xd0\x81", EncGBK); got != "中�" {
		t.Errorf("Decode(非法尾字节) = %q, want 中+U+FFFD", got)
	}
}

func TestDecodePassthrough(t *testing.T) {
	for _, enc := range []Encoding{EncAuto, EncUTF8, EncApp} {
		if got := Decode(gbkZhongWen, enc); got != gbkZhongWen {
			t.Errorf("Decode(%s) = %q, want 原样", enc, got)
		}
	}
}

func TestEncodeRoundtrip(t *testing.T) {
	if got := Encode("中文", EncGBK); got != gbkZhongWen {
		t.Errorf("Encode(中文, GBK) = %x, want d6d0cec4", got)
	}
	if got := Encode("日志", EncGB18030); got != gbkRiZhi {
		t.Errorf("Encode(日志, GB18030) = %x, want c8d5d6be", got)
	}
}

// 编码器整串遇 emoji 返回空串+err:函数须逐 rune 容错,不支持字符替换 U+FFFD(spec §3)。
func TestEncodeEmojiToReplacement(t *testing.T) {
	if got := Encode("a🎉b", EncGBK); got != "a�b" {
		t.Errorf("Encode(含 emoji) = %q, want a+U+FFFD+b", got)
	}
}

func TestEncodePassthrough(t *testing.T) {
	if got := Encode("中文", EncUTF8); got != "中文" {
		t.Errorf("Encode(utf-8) = %q, want 原样", got)
	}
}

func TestReencodeUTF8ToGBK(t *testing.T) {
	if got := Reencode(gbkZhongWen, EncUTF8, EncGBK); got != "中文" {
		t.Errorf("Reencode(GBK bytes, utf8→gbk) = %q, want 中文", got)
	}
}

func TestReencodeGBKToUTF8AndBack(t *testing.T) {
	once := Reencode("中文", EncGBK, EncUTF8)
	if once != gbkZhongWen {
		t.Fatalf("Reencode(中文, gbk→utf8) = %x, want 还原为 GBK 原始字节", once)
	}
	if back := Reencode(once, EncUTF8, EncGBK); back != "中文" {
		t.Errorf("往返 = %q, want 中文", back)
	}
}

func TestReencodeSameAndAppPassthrough(t *testing.T) {
	if got := Reencode("abc", EncUTF8, EncUTF8); got != "abc" {
		t.Errorf("同编码 = %q, want 原样", got)
	}
	if got := Reencode("abc", EncApp, EncGBK); got != "abc" {
		t.Errorf("程序行 = %q, want 原样旁路", got)
	}
	// to=EncAuto 返回原始字节串(交由调用方重检测)
	if got := Reencode("中文", EncGBK, EncAuto); got != gbkZhongWen {
		t.Errorf("to=auto = %x, want 原始 GBK 字节", got)
	}
}

func TestParseEncodingRoundtrip(t *testing.T) {
	for _, s := range []string{"auto", "utf-8", "gbk", "gb18030"} {
		e, ok := ParseEncoding(s)
		if !ok || e.String() != s {
			t.Errorf("ParseEncoding(%q) = %v,%v 往返失真", s, e, ok)
		}
	}
	if _, ok := ParseEncoding("big5"); ok {
		t.Error("ParseEncoding(big5) 应不支持")
	}
	if EncApp.String() != "app" {
		t.Errorf("EncApp.String() = %q, want app", EncApp.String())
	}
}
