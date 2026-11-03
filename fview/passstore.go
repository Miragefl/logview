package main

import (
	"github.com/99designs/keyring"
)

// passStore 密码仓抽象：系统钥匙串存取接口（生产走 keyring，测试注入 fake）。
type passStore interface {
	Get(key string) string
	Set(key, pass string) error
}

// keyringPassStore 99designs/keyring 实现：mac=Keychain / win=凭据库 / linux=SecretService，
// ServiceName "fview" 即钥匙串条目归属（系统钥匙串访问.app 可管理）。
type keyringPassStore struct {
	ring keyring.Keyring
}

// newKeyringPassStore 打开系统钥匙串；打开失败返回降级实现（Get 恒空=回落每次输入现状）。
func newKeyringPassStore() passStore {
	ring, err := keyring.Open(keyring.Config{ServiceName: "fview"})
	if err != nil {
		return &nullPassStore{}
	}
	return &keyringPassStore{ring: ring}
}

// Get 取密码；不存在（keyring.ErrKeyNotFound 等）一律空串（=免密/首访，前端走 needPwd 流）。
func (k *keyringPassStore) Get(key string) string {
	item, err := k.ring.Get(key)
	if err != nil {
		return ""
	}
	return string(item.Data)
}

// Set 存密码（幂等覆盖由钥匙串保证，改密后重写同 key）。
func (k *keyringPassStore) Set(key, pass string) error {
	return k.ring.Set(keyring.Item{Key: key, Data: []byte(pass)})
}

// nullPassStore 降级实现：钥匙串不可用时密码功能整体回落现状（恒空读/写丢弃，不崩）。
type nullPassStore struct{}

func (n *nullPassStore) Get(string) string         { return "" }
func (n *nullPassStore) Set(string, string) error  { return nil }
