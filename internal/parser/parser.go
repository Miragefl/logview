package parser

import "github.com/justfun/logview/internal/model"

// Parser converts raw text into structured log lines.
type Parser interface {
	Parse(raw model.RawLine) *model.ParsedLine
	Name() string
}