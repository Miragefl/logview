package usage

import (
	"sync"
	"testing"
)

// TestConcurrentBumpAndRecent 并发 Bump + RecentSourceKeys 不炸（-race 下验证；
// Wails 绑定在不同 goroutine 并发执行，锁外遍历内部 map 会 fatal）。
func TestConcurrentBumpAndRecent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ResetForTest()

	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				Bump(KeyFile + "/tmp/f")
			}
		}(w)
		go func() {
			defer wg.Done()
			_ = RecentSourceKeys(50)
		}()
	}
	wg.Wait()
}
