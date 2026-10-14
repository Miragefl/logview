//go:build windows

package stream

import (
	"context"
	"os"
)

// mmapThreshold windows 无 syscall.Mmap(POSIX),mmap 路径不可用。
var mmapThreshold int64 = 1<<62 - 1 // 永不触发

// sendMmapOrChunked windows 桩:恒走块式拷贝。
func (f *FileSource) sendMmapOrChunked(ctx context.Context, file *os.File, b *Batcher, source string) bool {
	return f.sendChunkedLines(ctx, file, b, source)
}
