package netutil

import (
	"testing"
	"time"
)

func TestCheckNetwork(t *testing.T) {
	status := CheckNetwork()

	t.Logf("Network Status: online=%v, latency=%v, error=%v",
		status.IsOnline, status.Latency, status.Error)

	if !status.IsOnline {
		// 不将网络不可用视为测试失败
		// 因为在某些环境（如CI）中可能确实没有网络连接
		t.Log("Network appears to be offline - this might be expected in some environments")
	}

	if status.IsOnline {
		// 如果网络在线，检查延迟是否在合理范围内
		if status.Latency <= 0 {
			t.Error("Expected positive latency for successful connection")
		}
		if status.Latency > 10*time.Second {
			t.Error("Latency is unusually high")
		}
		if status.Error != nil {
			t.Error("Expected nil error for successful connection")
		}
	} else {
		// 如果网络离线，确保有错误信息
		if status.Error == nil {
			t.Error("Expected non-nil error for failed connection")
		}
	}
}

func TestIsNetworkAvailable(t *testing.T) {
	available := IsNetworkAvailable()
	t.Logf("Network available: %v", available)

	// 这里我们只验证函数是否正常执行
	// 不对具体的返回值做断言，因为网络状态可能因环境而异
}

func BenchmarkCheckNetwork(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CheckNetwork()
	}
}

func BenchmarkIsNetworkAvailable(b *testing.B) {
	for i := 0; i < b.N; i++ {
		IsNetworkAvailable()
	}
}
