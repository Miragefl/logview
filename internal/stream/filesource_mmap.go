//go:build !windows

package stream

import (
	"context"
	"os"
	"syscall"
	"unsafe"
)

// mmapThreshold 超过此大小的普通文件全量读走 mmap 零拷贝(小文件映射的
// 系统调用与页表开销不划算);测试可调小以覆盖该路径。
var mmapThreshold int64 = 64 << 20

// sendMmapOrChunked 大文件读分流:mmap 零拷贝优先,回落块式拷贝。
// ctx 取消语义与 sendChunkedLines 一致(Send 返回 false 即停)。
func (f *FileSource) sendMmapOrChunked(ctx context.Context, file *os.File, b *Batcher, source string) bool {
	if st, err := file.Stat(); err == nil && st.Size() >= mmapThreshold {
		if f.sendMmapLines(b, file, source) {
			return true
		}
		// 映射失败回落下方块式;文件偏移仍在 0(尚未读过),块式从头读
		if _, err := file.Seek(0, 0); err != nil {
			return true
		}
	}
	return f.sendChunkedLines(ctx, file, b, source)
}

// sendMmapLines mmap 零拷贝读:整个文件映射进地址空间,行文本以 unsafe
// 子串直指映射区——原文不进堆,堆驻留仅剩索引结构(--all 全量驻留场景
// 数据本体 5G→0,只剩 ~3.5G 索引);数据本体由 OS 页缓存背书,内存紧张时
// 干净页自动逐出、再访问按需缺页,进程对系统内存压力从"独占"变"让路"。
// 行扫描天然无跨块 carry(mmap 是连续全区)。
// 生命周期:映射保持到进程退出不主动 unmap——buffer 内行的 Raw.Text 与
// span 长期引用映射区,安全 unmap 需全局引用归零协调,代价不成比例;换源
// 只积累虚拟映射,物理页按访问驻留,64 位地址空间(128TB)无实质伤害。
// 风险:映射期间文件被 truncate 时访问越界页 SIGBUS(log rotate 的
// rename+新建安全,copytruncate 型危险——静态浏览语义下接受并在此注明)。
// 返回 false 表示映射不适用(空文件/映射失败),调用方回落块式。
func (f *FileSource) sendMmapLines(b *Batcher, file *os.File, source string) bool {
	st, err := file.Stat()
	if err != nil || st.Size() == 0 {
		return false
	}
	data, err := syscall.Mmap(int(file.Fd()), 0, int(st.Size()), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return false
	}
	// fd 保持打开即够;mapping 生命周期见函数注释(不 unmap)
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			if !b.Send(mmapLineString(data[start:i]), source) {
				return true
			}
			start = i + 1
		}
	}
	if start < len(data) { // 末行无换行符,同样消费
		b.Send(mmapLineString(data[start:]), source)
	}
	return true
}

// mmapLineString 映射区字节切片构造共享 backing 的 string(零拷贝)。
// 映射区非 Go 堆,GC 不追踪其存活——由"映射不 unmap"策略兜底(见上)。
func mmapLineString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}
