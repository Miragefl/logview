package stream

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

// seekTailLines 定位文件"最后 lines 行"的起点偏移(倒数第 lines+1 个换行之后;
// 文件换行总数不足 lines 时返回 0,即从头读)。从尾部 1MB 分块倒扫,
// 数够换行即停——10 万行量级只扫几十 MB,内存 O(1MB)。
func seekTailLines(f *os.File, lines int) int64 {
	st, err := f.Stat()
	if err != nil || st.Size() == 0 {
		return 0
	}
	const blk = 1 << 20
	size := st.Size()
	pos := size
	nl := 0
	buf := make([]byte, blk)
	for pos > 0 {
		n := min(int64(blk), pos)
		pos -= n
		if _, err := f.ReadAt(buf[:n], pos); err != nil {
			return 0 // 定位失败回落从头读
		}
		for i := n - 1; i >= 0; i-- {
			if buf[i] == '\n' {
				nl++
				if nl > lines {
					return pos + i + 1
				}
			}
		}
	}
	return 0
}

// isGzipFile 探测文件头两字节是否 gzip magic(探测后 seek 归零,不影响后续读取)。
func isGzipFile(f *os.File) bool {
	var magic [2]byte
	n, _ := io.ReadFull(f, magic[:])
	f.Seek(0, io.SeekStart)
	return n == 2 && bytes.Equal(magic[:], gzipMagic)
}

// tailNoticeLine 尾读提示行:告知当前是尾部窗口,更早历史不在缓冲内,--buffer-size 可扩。
func tailNoticeLine(tailLines int, size int64) string {
	return fmt.Sprintf("[logview] 大文件尾读:加载末尾 ≤%d 行(共 %.1f MB,--buffer-size 可调)", tailLines, float64(size)/1e6)
}

// CountLines 统计普通文件总行数(按换行符计;末行无换行符也计入)。
// 供 --all 全量加载前确定缓冲容量——ring buffer 预分配需要精确值。
// gz 流需顺序解压无法廉价数行,直接报错由调用方提示用户。
func CountLines(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if isGzipFile(f) {
		return 0, fmt.Errorf("%s: gzip archives do not support --all", path)
	}
	var n int64
	var lastByte byte
	readAny := false
	buf := make([]byte, 1<<20)
	for {
		c, err := f.Read(buf)
		for i := range c {
			if buf[i] == '\n' {
				n++
			}
			lastByte = buf[i]
		}
		readAny = readAny || c > 0
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
	}
	// 文件非空且末字节非换行:末行无换行符,补计 1 行
	if readAny && lastByte != '\n' {
		n++
	}
	return n, nil
}
