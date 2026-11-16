package encoding

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func storePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "encoding.json")
}

// entriesForTest 暴露盘上条目计数(LRU 断言用)。
func (s *Store) entriesForTest() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	return len(s.entries)
}

func TestStoreRememberAndLoad(t *testing.T) {
	p := storePath(t)
	s := NewStore(p)
	s.Remember("ssh:web1/app/x.log", EncGBK)
	if got := s.LoadFor("ssh:web1/app/x.log"); got != EncGBK {
		t.Errorf("LoadFor = %v, want gbk", got)
	}
	// 落盘可持久:新实例读同路径
	s2 := NewStore(p)
	if got := s2.LoadFor("ssh:web1/app/x.log"); got != EncGBK {
		t.Errorf("新实例 LoadFor = %v, want gbk(落盘失效)", got)
	}
}

func TestStoreOnlyNonUTF8Persisted(t *testing.T) {
	s := NewStore(storePath(t))
	s.Remember("k8sres:pod/x", EncUTF8)
	if got := s.LoadFor("k8sres:pod/x"); got != EncAuto {
		t.Errorf("UTF-8 不落盘, LoadFor = %v want auto", got)
	}
	// 覆盖删除:GBK 记忆被切回 UTF-8 抹掉
	s.Remember("k8sres:pod/x", EncGBK)
	s.Remember("k8sres:pod/x", EncUTF8)
	if got := s.LoadFor("k8sres:pod/x"); got != EncAuto {
		t.Errorf("切回 UTF-8 应删条目, LoadFor = %v want auto", got)
	}
}

func TestStoreLRU50(t *testing.T) {
	p := storePath(t)
	s := NewStore(p)
	for i := range 55 {
		s.Remember(fmt.Sprintf("ssh:h%02d/p/x.log", i), EncGBK)
	}
	s2 := NewStore(p)
	// 直接数盘上条目:55 记 50 存,最早的被淘汰(同纳秒 TS 时按 key 字典序兜底)
	if n := s2.entriesForTest(); n != 50 {
		t.Errorf("盘上条目 = %d, want 50(LRU 截断失效)", n)
	}
}

func TestStoreCorruptFileTolerated(t *testing.T) {
	p := storePath(t)
	os.WriteFile(p, []byte("{not json"), 0644)
	s := NewStore(p)
	if got := s.LoadFor("ssh:h/p"); got != EncAuto {
		t.Errorf("坏 JSON 应容错回 auto, got %v", got)
	}
	// 坏文件上 Remember 应能覆盖写恢复
	s.Remember("ssh:h/p", EncGB18030)
	if got := NewStore(p).LoadFor("ssh:h/p"); got != EncGB18030 {
		t.Errorf("坏文件覆盖写失效, got %v", got)
	}
}

func TestStoreMissingFile(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "nope", "encoding.json"))
	if got := s.LoadFor("file:/x.log"); got != EncAuto {
		t.Errorf("文件不存在 LoadFor = %v, want auto", got)
	}
}
