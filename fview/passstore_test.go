package main

import "testing"

// fakePassStore 内存版密码仓（绕过系统钥匙串，单测注入用）。
type fakePassStore struct {
	m map[string]string
}

func (f *fakePassStore) Get(key string) string { return f.m[key] }
func (f *fakePassStore) Set(key, pass string) error {
	f.m[key] = pass
	return nil
}

// newTestAppPass 注入 fake 密码仓的 App（绑定层透传验证）。
func newTestAppPass() *App {
	a := NewApp()
	a.passes = &fakePassStore{m: map[string]string{}}
	return a
}

// TestGetSavedPassEmpty 未存过的 key 返回空串（=免密/首访，前端走 needPwd 流）。
func TestGetSavedPassEmpty(t *testing.T) {
	a := newTestAppPass()
	if got := a.GetSavedPass("ssh:web1"); got != "" {
		t.Fatalf("未存应空串,got %q", got)
	}
}

// TestSavePassRoundtrip 存后取回：Keychain 语义的最小闭环。
func TestSavePassRoundtrip(t *testing.T) {
	a := newTestAppPass()
	if err := a.SavePass("ssh:web1", "p1"); err != nil {
		t.Fatalf("SavePass err: %v", err)
	}
	if got := a.GetSavedPass("ssh:web1"); got != "p1" {
		t.Fatalf("应取回 p1,got %q", got)
	}
}

// TestSavePassEmptyGuard 空密码不入库（密钥认证/免密场景不产生垃圾条目）。
func TestSavePassEmptyGuard(t *testing.T) {
	a := newTestAppPass()
	if err := a.SavePass("ssh:web1", ""); err != nil {
		t.Fatalf("空密码应静默成功,got %v", err)
	}
	if got := a.GetSavedPass("ssh:web1"); got != "" {
		t.Fatalf("空密码不应入库,got %q", got)
	}
}

// TestSavePassOverwrite 改密后覆盖旧值（幂等覆盖，Review Focus 1）。
func TestSavePassOverwrite(t *testing.T) {
	a := newTestAppPass()
	_ = a.SavePass("ssh:web1", "old")
	_ = a.SavePass("ssh:web1", "new")
	if got := a.GetSavedPass("ssh:web1"); got != "new" {
		t.Fatalf("应覆盖为 new,got %q", got)
	}
}
