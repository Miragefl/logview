package buffer

import "github.com/justfun/logview/internal/model"

type RingBuffer struct {
	buf   []*model.ParsedLine
	cap   int
	head  int
	len   int
	total uint64
}

func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		buf: make([]*model.ParsedLine, capacity),
		cap: capacity,
	}
}

func (rb *RingBuffer) Push(line *model.ParsedLine) {
	idx := (rb.head + rb.len) % rb.cap
	rb.buf[idx] = line
	if rb.len < rb.cap {
		rb.len++
	} else {
		rb.head = (rb.head + 1) % rb.cap
	}
	rb.total++
}

func (rb *RingBuffer) Len() int { return rb.len }
func (rb *RingBuffer) TotalReceived() uint64 { return rb.total }

func (rb *RingBuffer) Get(i int) *model.ParsedLine {
	if i < 0 || i >= rb.len {
		return nil
	}
	idx := (rb.head + i) % rb.cap
	return rb.buf[idx]
}

func (rb *RingBuffer) Slice(start, end int) []*model.ParsedLine {
	if start < 0 {
		start = 0
	}
	if end > rb.len {
		end = rb.len
	}
	if start >= end {
		return nil
	}
	result := make([]*model.ParsedLine, end-start)
	for i := range result {
		result[i] = rb.Get(start + i)
	}
	return result
}