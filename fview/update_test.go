package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAppVersionMatchesBuildVar AppVersion 直读 ldflags 注入的 version 变量(dev 未注入为空串)。
func TestAppVersionMatchesBuildVar(t *testing.T) {
	a := NewApp()
	if got := a.AppVersion(); got != version {
		t.Fatalf("AppVersion() = %q, want %q(version 变量)", got, version)
	}
}

// TestCheckUpdateParsesLatest 正常路径:解析 tag_name/html_url。
func TestCheckUpdateParsesLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":"v9.9.9","html_url":"https://github.com/Miragefl/logview/releases/tag/v9.9.9"}`))
	}))
	defer srv.Close()

	a := NewApp()
	a.updateAPIs = []string{srv.URL}
	info, err := a.CheckUpdate()
	if err != nil {
		t.Fatalf("CheckUpdate: %v", err)
	}
	if info.Latest != "v9.9.9" {
		t.Fatalf("Latest = %q, want v9.9.9", info.Latest)
	}
	if info.URL == "" || info.Current != version {
		t.Fatalf("info = %+v", info)
	}
}

// TestCheckUpdateFallsBack 主源 403(GitHub 限流常态)→ 回落备源成功,
// html_url 用实际命中的源(哪个源查到用哪个)。
func TestCheckUpdateFallsBack(t *testing.T) {
	fail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer fail.Close()
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":"v0.20.1","html_url":"https://gitee.com/Mtok/logview/releases/tag/v0.20.1"}`))
	}))
	defer ok.Close()

	a := NewApp()
	a.updateAPIs = []string{fail.URL, ok.URL}
	info, err := a.CheckUpdate()
	if err != nil {
		t.Fatalf("CheckUpdate: %v", err)
	}
	if info.Latest != "v0.20.1" {
		t.Fatalf("Latest = %q, want v0.20.1", info.Latest)
	}
	if info.URL != "https://gitee.com/Mtok/logview/releases/tag/v0.20.1" {
		t.Fatalf("URL = %q, want gitee html_url", info.URL)
	}
}

// TestCheckUpdateNetworkError 全部源不可达:返回 error 而非 panic,可重试。
func TestCheckUpdateNetworkError(t *testing.T) {
	a := NewApp()
	a.updateAPIs = []string{"http://127.0.0.1:1", "http://127.0.0.1:2"} // 均不可达
	if _, err := a.CheckUpdate(); err == nil {
		t.Fatal("网络不可达应返回 error")
	}
}
