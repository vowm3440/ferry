package host

import (
	"testing"
	"time"
)

func TestTrafficKeepsRateBaselineAcrossFrequentSnapshots(t *testing.T) {
	h := New()
	h.sampledAt = time.Now()
	h.updateTraffic(connectionSnapshot{UploadTotal: 50, DownloadTotal: 100})
	h.sampledAt = time.Now().Add(-2 * time.Second)
	h.updateTraffic(connectionSnapshot{UploadTotal: 100, DownloadTotal: 200, Memory: 4096})
	if h.traffic.Up < 49 || h.traffic.Up > 50 || h.traffic.Down < 99 || h.traffic.Down > 100 {
		t.Fatalf("首次快照丢失了速率计数: %+v", h.traffic)
	}
	if h.traffic.UpHistory[59] != h.traffic.Up || h.traffic.UpHistory[58] != 0 {
		t.Fatal("历史应将遗漏秒补零")
	}
	if h.state.MemoryBytes != 4096 {
		t.Fatal("未更新内存统计")
	}
}

func TestTrafficRejectsOutOfOrderSnapshots(t *testing.T) {
	h := New()
	h.sampledAt = time.Now()
	h.updateTraffic(connectionSnapshot{UploadTotal: 100, DownloadTotal: 200})
	h.updateTraffic(connectionSnapshot{UploadTotal: 50, DownloadTotal: 100})
	if h.traffic.TotalUp != 100 || h.traffic.TotalDown != 200 {
		t.Fatal("乱序响应导致累计流量倒退")
	}
}

func TestLongTrafficGapRetainsOnlyLastMinute(t *testing.T) {
	h := New()
	h.sampledAt = time.Now().Add(-2 * time.Minute)
	for i := range h.traffic.UpHistory {
		h.traffic.UpHistory[i] = 99
	}
	h.updateTraffic(connectionSnapshot{UploadTotal: 240})
	for _, value := range h.traffic.UpHistory[:59] {
		if value != 0 {
			t.Fatal("保留了60秒之前的历史")
		}
	}
	if h.traffic.UpHistory[59] < 1 {
		t.Fatal("缺少最后采样值")
	}
}
