package stream

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"os"
)

// gzipMagic gzip 流头两字节(RFC 1952),本地嗅探依据(与文件名无关)。
var gzipMagic = []byte{0x1f, 0x8b}

// isGzipMagic br 头两字节是否 gzip magic(Peek 不消费 bufio 的逻辑读位,
// 但可能触发预读推进底层 fd 偏移——嗅探后请复用同一 br,勿重建 reader)。
func isGzipMagic(br *bufio.Reader) bool {
	magic, err := br.Peek(2)
	return err == nil && bytes.Equal(magic, gzipMagic)
}

// multiCloser 逆序关闭多个 Closer,返回首个错误(其余仍尽力关)。
type multiCloser []io.Closer

func (m multiCloser) Close() error {
	var firstErr error
	for i := len(m) - 1; i >= 0; i-- {
		if err := m[i].Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// openMaybeGzip 打开文件并按 gzip magic 嗅探:内容是 gzip 流则返回解压 reader。
// isGz 标记是否走了解压;返回的 closer 负责关闭 gzip 流与底层文件,调用方必须 defer。
func openMaybeGzip(path string) (*bufio.Reader, io.Closer, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, false, err
	}
	br := bufio.NewReader(f)
	if isGzipMagic(br) {
		gz, err := gzip.NewReader(br)
		if err != nil {
			f.Close()
			return nil, nil, false, err
		}
		return bufio.NewReader(gz), multiCloser{gz, f}, true, nil
	}
	return br, f, false, nil
}
