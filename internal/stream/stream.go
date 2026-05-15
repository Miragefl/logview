package stream

import (
	"context"
	"github.com/justfun/logview/internal/model"
)

// LogStream produces raw log lines from a data source.
type LogStream interface {
	Start(ctx context.Context) (<-chan model.RawLine, error)
	Label() string
	Cleanup() error
}