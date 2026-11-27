//go:build with_clash_api

package host

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"
)

func TestLifecycleModeSelectionAndProxyEgress(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "ferry-real-egress") }))
	defer origin.Close()
	upstream := newSOCKS(t)
	s := hostFixture(t, upstream.Addr().(*net.TCPAddr).Port)
	h := New()
	t.Cleanup(func() { _ = h.Stop(context.Background()) })
	if err := h.Start(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if err := h.Start(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if state := h.Status(); state.Status != "running" || state.CoreVersion != "1.14.4" {
		t.Fatalf("错误运行状态: %+v", state)
	}
	if err := h.SetMode(context.Background(), "global"); err != nil {
		t.Fatal(err)
	}
	if h.Status().Mode != "global" {
		t.Fatal("模式状态未更新")
	}
	if err := h.SetMode(context.Background(), "unsupported"); err == nil {
		t.Fatal("未知模式未拒绝")
	}
	if err := h.SelectNode(context.Background(), "proxy", "n1"); err != nil {
		t.Fatal(err)
	}
	if err := h.SelectNode(context.Background(), "proxy", "absent"); err == nil {
		t.Fatal("不存在的节点未拒绝")
	}
	body := requestThroughProxy(t, s.Inbound.MixedPort, origin.URL)
	if upstream.requests.Load() == 0 {
		t.Fatal("请求绕过了SOCKS节点")
	}
	traffic, err := h.Snapshot(context.Background())
	if err != nil || traffic.TotalDown < int64(len(body)) || traffic.TotalUp == 0 {
		t.Fatalf("无真实统计: %+v %v", traffic, err)
	}
	assertAPISecurity(t, h)
	assertStopped(t, h, s.Inbound.MixedPort)
}

func assertStopped(t *testing.T, h *Host, port uint16) {
	t.Helper()
	if err := h.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := h.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if h.Status().Status != "stopped" {
		t.Fatal("未停止")
	}
	if conn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(int(port))); err == nil {
		conn.Close()
		t.Fatal("停止未释放端口")
	}
}

func requestThroughProxy(t *testing.T, port uint16, target string) []byte {
	t.Helper()
	proxyURL, _ := url.Parse("http://127.0.0.1:" + strconv.Itoa(int(port)))
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	response, err := (&http.Client{Transport: transport}).Get(target)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || string(body) != "ferry-real-egress" {
		t.Fatalf("实际出站失败: %s %v", body, err)
	}
	return body
}

func TestStartFailureRecoveryAndConcurrentLifecycle(t *testing.T) {
	upstream := newSOCKS(t)
	s := hostFixture(t, upstream.Addr().(*net.TCPAddr).Port)
	occupied, err := net.Listen("tcp4", "127.0.0.1:"+strconv.Itoa(int(s.Inbound.MixedPort)))
	if err != nil {
		t.Fatal(err)
	}
	h := New()
	t.Cleanup(func() { _ = h.Stop(context.Background()) })
	if err := h.Start(context.Background(), s); err == nil {
		t.Fatal("占用端口启动成功")
	}
	if h.Status().Status != "error" {
		t.Fatal("失败状态未报告")
	}
	occupied.Close()
	if err := h.Start(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() { _ = h.Status(); _, _ = h.Snapshot(context.Background()); _ = h.Stop(context.Background()) })
	}
	wg.Wait()
	if err := h.Start(context.Background(), s); err != nil {
		t.Fatal(err)
	}
}

func assertAPISecurity(t *testing.T, h *Host) {
	t.Helper()
	h.mu.RLock()
	runtime := h.current
	h.mu.RUnlock()
	response, err := http.Get("http://" + runtime.address + "/configs")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatal("Clash API没有鉴权")
	}
	data, _ := json.Marshal(h.Status())
	if len(runtime.secret) < 43 || containsSecret(data, runtime.secret) {
		t.Fatal("密钥不安全或泄漏")
	}
}
