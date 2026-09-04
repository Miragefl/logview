package stream

import (
	"context"
	"github.com/justfun/logview/internal/model"
)

// LogStream produces raw log lines from a data source.
type LogStream interface {
	Start(ctx context.Context) (<-chan model.RawLine, error)
	Label() string
	// Scope 返回源的词频隔离域(关键词历史/频次按此分域:FRP=连接名、SSH=主机、k8s="k8s"、本地/管道="")。
	Scope() string
	Cleanup() error
}