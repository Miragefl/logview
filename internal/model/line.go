package model

import "time"

// RawLine is an unparsed log line from any data source.
type RawLine struct {
	Text   string    // raw text
	Source string    // origin label, e.g. "pod/api-7d8f6-x8k2j"
	Seq    uint64    // monotonic sequence number
}

// Field represents a parsed log field name.
type Field string

const (
	FieldTime    Field = "time"
	FieldLevel   Field = "level"
	FieldThread  Field = "thread"
	FieldTraceID Field = "traceId"
	FieldLogger  Field = "logger"
	FieldMessage Field = "message"
	FieldSource  Field = "source"
)

// AllFields lists all possible parsed fields in display order.
// Can be overridden via config.
var AllFields = []Field{FieldTime, FieldSource, FieldLevel, FieldThread, FieldTraceID, FieldLogger, FieldMessage}

// SetAllFields overrides the global field list from config.
func SetAllFields(fields []Field) {
	if len(fields) > 0 {
		AllFields = fields
	}
}

// 标准字段在 f 区间数组中的槽位序号
const (
	fTime = iota
	fLevel
	fThread
	fTraceID
	fLogger
	fMessage
	nStdFields
)

// stdFieldIdx 标准字段名 → 槽位;非标准返回 -1
func stdFieldIdx(f Field) int {
	switch f {
	case FieldTime:
		return fTime
	case FieldLevel:
		return fLevel
	case FieldThread:
		return fThread
	case FieldTraceID:
		return fTraceID
	case FieldLogger:
		return fLogger
	case FieldMessage:
		return fMessage
	}
	return -1
}

// HasStdSpan 字段是否为可用 span 区间表示的标准字段(parser 构造分流用)。
func HasStdSpan(f Field) bool { return stdFieldIdx(f) >= 0 }

// span 标准字段在 base 串中的字节区间(off+ln<=uint32 上限,单行日志远不及)
type span struct{ off, ln uint32 }

// ParsedLine 列存结构:标准字段不驻留独立 string(7×16B header + map 桶),
// 而是 (off,ln) 区间指向 base 串,取值时零拷贝切片构造——
// 每行驻留从 map 时代 ~920B / string 时代 529B 压至 ~350B(含原文块)。
// base:无 ANSI 行即 Raw.Text(StripANSI 无匹配返回原串,零成本共享);
// 含 ANSI 行经清洗产生独立串,由 base 持有驻留(正确性优先,退化场景)。
type ParsedLine struct {
	Raw    RawLine
	base   string
	unixMs int64           // epoch 毫秒;0=未解析出时间(1970-01-01 理论歧义,日志场景不存在)
	f      [nStdFields]span // 标准字段区间:time/level/thread/traceId/logger/message
	Fields map[Field]string // 自定义字段(json 行的标准字段也走此 map——json 值不在原文中,无法引用)
}

// SetStd 记录标准字段区间(parser 构造用)。base 由 InitBase 先行设置。
func (p *ParsedLine) InitBase(base string) { p.base = base }

// SetStdSpan 记录标准字段在 base 中的字节区间。
func (p *ParsedLine) SetStdSpan(f Field, off, ln int) {
	i := stdFieldIdx(f)
	if i < 0 || off < 0 || ln < 0 || off+ln > len(p.base) {
		return
	}
	p.f[i] = span{uint32(off), uint32(ln)}
}

// SetUnixMs 记录解析出的绝对时刻(毫秒)。
func (p *ParsedLine) SetUnixMs(ms int64) { p.unixMs = ms }

// std 取标准字段值(零拷贝切片)
func (p *ParsedLine) std(i int) string {
	s := p.f[i]
	if s.ln == 0 {
		return ""
	}
	return p.base[s.off : s.off+s.ln]
}

// stdOrMap 标准字段取值统一入口:Fields map 优先(json 行/测试构造),
// 否则走 span 切片(regex 行零拷贝)——与 Get 语义一致。
func (p *ParsedLine) stdOrMap(f Field, i int) string {
	if p.Fields != nil {
		if v, ok := p.Fields[f]; ok {
			return v
		}
	}
	return p.std(i)
}

// Message/Level/Thread/TraceID/Logger 与原具名字段同名的方法形式,
// 调用点从 .Message 改 .Message() 即可。Time 返回绝对时刻(零值=未解析)。
func (p *ParsedLine) Message() string { return p.stdOrMap(FieldMessage, fMessage) }
func (p *ParsedLine) Level() string   { return p.stdOrMap(FieldLevel, fLevel) }
func (p *ParsedLine) Thread() string  { return p.stdOrMap(FieldThread, fThread) }
func (p *ParsedLine) TraceID() string { return p.stdOrMap(FieldTraceID, fTraceID) }
func (p *ParsedLine) Logger() string  { return p.stdOrMap(FieldLogger, fLogger) }
func (p *ParsedLine) Time() time.Time {
	if p.unixMs == 0 {
		return time.Time{}
	}
	return time.UnixMilli(p.unixMs)
}

// UnixMs 返回绝对时刻毫秒(0=未解析);时间比较运算直接用它,避免构造 time.Time。
func (p *ParsedLine) UnixMs() int64 { return p.unixMs }

// Get returns the value for a given field.
// Checks Fields map first, then falls back to std spans / Raw.
func (p *ParsedLine) Get(f Field) string {
	if p.Fields != nil {
		if v, ok := p.Fields[f]; ok {
			return v
		}
	}
	if i := stdFieldIdx(f); i >= 0 {
		return p.std(i)
	}
	if f == FieldSource {
		return p.Raw.Source
	}
	return ""
}

// TestLine 便捷构造(测试用):Message 独立 string 进 Fields map,驻留非最优,生产路径勿用。
func TestLine(msg string) *ParsedLine {
	return &ParsedLine{Fields: map[Field]string{FieldMessage: msg}}
}

// FieldMask controls which fields are visible in the TUI.
type FieldMask map[Field]bool

// DefaultFieldMask returns the default visible fields.
// Uses AllFields to include any custom fields.
func DefaultFieldMask() FieldMask {
	mask := FieldMask{
		FieldTime:    true,
		FieldLevel:   true,
		FieldThread:  false,
		FieldTraceID: false,
		FieldLogger:  false,
		FieldMessage: true,
		FieldSource:  true,
	}
	// custom fields default to visible
	for _, f := range AllFields {
		if _, ok := mask[f]; !ok {
			mask[f] = true
		}
	}
	return mask
}

// NewFieldMaskFromConfig creates a FieldMask from explicit config.
func NewFieldMaskFromConfig(fields []FieldConfigEntry) FieldMask {
	mask := make(FieldMask)
	for _, f := range fields {
		mask[Field(f.Name)] = f.Visible
	}
	return mask
}

// FieldConfigEntry represents a field entry from config.
type FieldConfigEntry struct {
	Name    string
	Visible bool
}

// IsVisible returns whether a field should be displayed.
func (fm FieldMask) IsVisible(f Field) bool {
	visible, ok := fm[f]
	return ok && visible
}

// Toggle flips the visibility of a field.
func (fm FieldMask) Toggle(f Field) {
	fm[f] = !fm[f]
}
