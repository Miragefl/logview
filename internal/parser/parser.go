package parser

import "github.com/justfun/logview/internal/model"

// Parser converts raw text into structured log lines.
type Parser interface {
	Parse(raw model.RawLine) *model.ParsedLine
	Name() string
}

// FieldProducer 可选能力:声明解析器可产出的字段名(命名捕获组/JSON key)。
// 宿主(如 fview 列裁剪)类型断言获取;未实现者(含测试桩)不受影响——
// Parser 接口保持 Parse/Name 两方法,不加连坐。
type FieldProducer interface {
	Fields() []string
}

var (
	_ FieldProducer = (*RegexParser)(nil)
	_ FieldProducer = (*JSONParser)(nil)
)