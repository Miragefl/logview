package parser

import (
	"regexp"
	"time"

	"github.com/justfun/logview/internal/model"
)

type RegexParser struct {
	name   string
	re     *regexp.Regexp
	groups []string
}

func NewRegexParser(name, pattern string) (*RegexParser, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &RegexParser{
		name:   name,
		re:     re,
		groups: re.SubexpNames()[1:],
	}, nil
}

func (p *RegexParser) Name() string { return p.name }

func (p *RegexParser) Parse(raw model.RawLine) *model.ParsedLine {
	cleaned := model.StripANSI(raw.Text)
	idx := p.re.FindStringSubmatchIndex(cleaned)
	if idx == nil {
		return nil
	}

	// 列存:字段以 (off,ln) 区间引用 base 串,不驻留独立 string。
	// 无 ANSI 时 StripANSI 返回原串,base=Raw.Text 零成本共享;
	// 含 ANSI 行 cleaned 为独立串,由 base 持有(退化驻留,正确性优先)
	result := &model.ParsedLine{Raw: raw}
	base := raw.Text
	if cleaned != raw.Text {
		base = cleaned
	}
	result.InitBase(base)

	for i, name := range p.groups {
		if 2*(i+1)+1 >= len(idx) {
			break
		}
		off, end := idx[2*(i+1)], idx[2*(i+1)+1]
		if off < 0 { // 可选组未参与匹配
			continue
		}
		f := model.Field(name)
		if f == model.FieldTime {
			val := base[off:end]
			// 布局按精度降级尝试；无日期格式落到 0000-01-01（匹配层按时分窗口比较）
			for _, layout := range []string{
				"2006-01-02 15:04:05.000",
				"2006-01-02 15:04:05",
				"2006-01-02T15:04:05",
				"15:04:05.000",
				"15:04:05",
				"15:04:05,000",
				"15:04",
			} {
				if t, err := time.ParseInLocation(layout, val, time.Local); err == nil {
					result.SetUnixMs(t.UnixMilli())
					break
				}
			}
		}
		// 标准字段走 span 区间;仅自定义字段进 Fields map(懒建)
		if model.HasStdSpan(f) {
			result.SetStdSpan(f, off, end-off)
			continue
		}
		if result.Fields == nil {
			result.Fields = make(map[model.Field]string, 1)
		}
		result.Fields[f] = base[off:end]
	}

	return result
}