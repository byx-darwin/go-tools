package netutil

import (
	"net"
	"testing"
)

func TestGetInternalIP(t *testing.T) {
	ip, err := GetInternalIP()
	if err != nil {
		t.Errorf("GetInternalIP() error = %v", err)
		return
	}
	if ip == "" {
		t.Log("No internal IP found, this might be normal in some environments")
		return
	}

	// 验证返回的IP是否为有效的IP地址
	if net.ParseIP(ip) == nil {
		t.Errorf("GetInternalIP() returned invalid IP: %v", ip)
	}
	t.Logf("Internal IP: %v", ip)
}

func TestIsInternalIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"A类内网", "10.0.0.1", true},
		{"B类内网", "172.16.0.1", true},
		{"C类内网", "192.168.1.1", true},
		{"公网IP", "8.8.8.8", false},
		{"本地回环", "127.0.0.1", false},
		{"公网IP2", "114.114.114.114", false},
		{"B类边界1", "172.15.255.255", false},
		{"B类边界2", "172.32.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if got := isInternalIP(ip); got != tt.expected {
				t.Errorf("isInternalIP() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func BenchmarkGetInternalIP(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = GetInternalIP()
	}
}

func BenchmarkIsInternalIP(b *testing.B) {
	ip := net.ParseIP("192.168.1.1")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		isInternalIP(ip)
	}
}
