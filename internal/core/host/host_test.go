package host

import (
	"context"
	"encoding/json"
	"ferry/internal/spec"
	"testing"
)

func TestStoppedStateAndInvalidOperations(t *testing.T) {
	h := New()
	if state := h.Status(); state.Status != "stopped" || state.Mode != "rule" || state.ServiceMode {
		t.Fatalf("错误初始状态: %+v", state)
	}
	traffic, err := h.Snapshot(context.Background())
	if err != nil || len(traffic.UpHistory) != 60 || len(traffic.DownHistory) != 60 || traffic.TotalUp != 0 {
		t.Fatalf("错误首屏快照: %+v %v", traffic, err)
	}
	traffic.UpHistory[0] = 9
	traffic, _ = h.Snapshot(context.Background())
	if traffic.UpHistory[0] != 0 {
		t.Fatal("快照暴露内部切片")
	}
	if err := h.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := h.Start(context.Background(), spec.Default()); err == nil {
		t.Fatal("空配置启动成功")
	}
	if err := h.SetMode(context.Background(), "global"); err == nil {
		t.Fatal("停止时可切模式")
	}
	if err := h.SelectNode(context.Background(), "g", "n"); err == nil {
		t.Fatal("停止时可选择节点")
	}
	data, err := json.Marshal(h.Status())
	if err != nil || !json.Valid(data) {
		t.Fatal(err)
	}
}

func TestCanceledOperations(t *testing.T) {
	h := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := h.Start(ctx, spec.Default()); err == nil {
		t.Fatal("已取消请求仍执行")
	}
	if err := h.Stop(ctx); err == nil {
		t.Fatal("未报告请求取消")
	}
	if h.Status().Status != "stopped" {
		t.Fatal("已取消启动修改状态")
	}
}
