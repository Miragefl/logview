package model

import "time"

// RawLine is an unparsed log line from any data source.
type RawLine struct {
	Text   string    // raw text
	Source string    // origin label, e.g. "pod/api-7d8f6-x9k2j"
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
var AllFields = []Field{FieldTime, FieldLevel, FieldThread, FieldTraceID, FieldLogger, FieldMessage, FieldSource}

// ParsedLine is a structured log line produced by a parser.
type ParsedLine struct {
	Raw      RawLine
	Time     time.Time
	Level    string
	Thread   string
	TraceID  string
	Logger   string
	Message  string
	Fields   map[Field]string // all extracted fields
}

// Get returns the value for a given field.
func (p *ParsedLine) Get(f Field) string {
	if p.Fields != nil {
		return p.Fields[f]
	}
	switch f {
	case FieldTime:
		return p.Time.Format("15:04:05.000")
	case FieldLevel:
		return p.Level
	case FieldThread:
		return p.Thread
	case FieldTraceID:
		return p.TraceID
	case FieldLogger:
		return p.Logger
	case FieldMessage:
		return p.Message
	case FieldSource:
		return p.Raw.Source
	}
	return ""
}

// FieldMask controls which fields are visible in the TUI.
type FieldMask map[Field]bool

// DefaultFieldMask returns the default visible fields.
func DefaultFieldMask() FieldMask {
	return FieldMask{
		FieldTime:    true,
		FieldLevel:   true,
		FieldThread:  false,
		FieldTraceID: false,
		FieldLogger:  false,
		FieldMessage: true,
		FieldSource:  true,
	}
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

// Level represents a log severity level.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// ParseLevel parses a level string (case-insensitive).
func ParseLevel(s string) Level {
	switch s {
	case "DEBUG", "debug", "DBG":
		return LevelDebug
	case "INFO", "info":
		return LevelInfo
	case "WARN", "warn", "WARNING", "warning":
		return LevelWarn
	case "ERROR", "error", "ERR", "FATAL", "fatal":
		return LevelError
	}
	return LevelInfo
}