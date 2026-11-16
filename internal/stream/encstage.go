package stream

import (
	"github.com/Miragefl/logview/internal/encoding"
)

// encStager 数据行的编码检测暂存包装:每文件/每 pod/每 SSH 源一个。
// 检测未定时行暂存不外发;结案后按结论转码打标直通。程序生成行
// ([logview] 提示)不走本口,直接 Batcher.Send(Enc 零值旁路)。
type encStager struct {
	d *encoding.Detector
}

func newEncStager() *encStager {
	return &encStager{d: encoding.NewDetector()}
}

// send 数据行过检测器再入批(结案输出的每一行都带 Enc 标记);
// ctx 取消返回 false(调用方停止读取)。
func (e *encStager) send(b *Batcher, text, source string) bool {
	for _, sl := range e.d.Stage(text) {
		if !b.SendEncoded(sl.Text, source, sl.Enc) {
			return false
		}
	}
	return true
}
