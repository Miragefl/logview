package buffer

import (
	"testing"

	"github.com/justfun/logview/internal/model"
)

func TestRingBufferAppendAndGet(t *testing.T) {
	rb := NewRingBuffer(5)
	for i := 0; i < 7; i++ {
		rb.Push(&model.ParsedLine{Message: string(rune('a' + i))})
	}

	if rb.Len() != 5 {
		t.Fatalf("Len() = %d, want 5", rb.Len())
	}

	first := rb.Get(0)
	if first.Message != "c" {
		t.Errorf("Get(0).Message = %q, want 'c'", first.Message)
	}

	last := rb.Get(4)
	if last.Message != "g" {
		t.Errorf("Get(4).Message = %q, want 'g'", last.Message)
	}
}

func TestRingBufferGetOutOfRange(t *testing.T) {
	rb := NewRingBuffer(3)
	rb.Push(&model.ParsedLine{Message: "a"})
	result := rb.Get(5)
	if result != nil {
		t.Error("expected nil for out-of-range Get")
	}
}

func TestRingBufferSlice(t *testing.T) {
	rb := NewRingBuffer(10)
	for i := 0; i < 5; i++ {
		rb.Push(&model.ParsedLine{Message: string(rune('a' + i))})
	}

	slice := rb.Slice(1, 4)
	if len(slice) != 3 {
		t.Fatalf("Slice(1,4) len = %d, want 3", len(slice))
	}
	if slice[0].Message != "b" {
		t.Errorf("slice[0].Message = %q, want 'b'", slice[0].Message)
	}
}

func TestRingBufferTotalReceived(t *testing.T) {
	rb := NewRingBuffer(3)
	rb.Push(&model.ParsedLine{Message: "a"})
	rb.Push(&model.ParsedLine{Message: "b"})
	rb.Push(&model.ParsedLine{Message: "c"})
	rb.Push(&model.ParsedLine{Message: "d"})

	if rb.TotalReceived() != 4 {
		t.Errorf("TotalReceived() = %d, want 4", rb.TotalReceived())
	}
	if rb.Len() != 3 {
		t.Errorf("Len() = %d, want 3", rb.Len())
	}
}