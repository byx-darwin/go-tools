package netutil

import (
	"net"
	"testing"
	"time"
)

// ── GetInternalIP ──

func TestGetInternalIP_ReturnsValidOrEmpty(t *testing.T) {
	ip, err := GetInternalIP()
	if err != nil {
		t.Fatalf("GetInternalIP() unexpected error: %v", err)
	}
	if ip != "" {
		if net.ParseIP(ip) == nil {
			t.Errorf("GetInternalIP() returned invalid IP: %v", ip)
		}
	}
	// Empty is also acceptable in some environments (no internal IP)
	t.Logf("Internal IP: %q", ip)
}

// ── isInternalIP ──

func TestIsInternalIP_IPv4(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"A类: 10.0.0.0", "10.0.0.0", true},
		{"A类: 10.0.0.1", "10.0.0.1", true},
		{"A类: 10.255.255.255", "10.255.255.255", true},
		{"B类: 172.16.0.0", "172.16.0.0", true},
		{"B类: 172.16.0.1", "172.16.0.1", true},
		{"B类: 172.31.255.255", "172.31.255.255", true},
		{"C类: 192.168.0.0", "192.168.0.0", true},
		{"C类: 192.168.1.1", "192.168.1.1", true},
		{"C类: 192.168.255.255", "192.168.255.255", true},
		{"公网: 8.8.8.8", "8.8.8.8", false},
		{"公网: 1.1.1.1", "1.1.1.1", false},
		{"公网: 114.114.114.114", "114.114.114.114", false},
		{"公网: 223.5.5.5", "223.5.5.5", false},
		{"本地回环: 127.0.0.1", "127.0.0.1", false},
		{"本地回环: 127.255.255.255", "127.255.255.255", false},
		{"B类下限: 172.15.255.255", "172.15.255.255", false},
		{"B类上限: 172.32.0.0", "172.32.0.0", false},
		{"边界: 0.0.0.0", "0.0.0.0", false},
		{"边界: 255.255.255.255", "255.255.255.255", false},
		{"非内网: 9.9.9.9", "9.9.9.9", false},
		{"非内网: 11.0.0.1", "11.0.0.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse %s", tt.ip)
			}
			got := isInternalIP(ip)
			if got != tt.want {
				t.Errorf("isInternalIP(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestIsInternalIP_IPv6(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"loopback", "::1", false},            // IsLoopback → false
		{"link-local", "fe80::1", false},       // link-local unicast, not IsPrivate
		{"unique-local", "fc00::1", true},      // ULA, IsPrivate = true
		{"global-unicast", "2001:db8::1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse %s", tt.ip)
			}
			got := isInternalIP(ip)
			if got != tt.want {
				t.Errorf("isInternalIP(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestIsInternalIP_NilInput(t *testing.T) {
	// nil net.IP — should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("isInternalIP panicked on nil: %v", r)
		}
	}()
	_ = isInternalIP(nil)
}

// ── CheckNetwork ──

func TestCheckNetwork_ReturnsStatus(t *testing.T) {
	status := CheckNetwork()
	t.Logf("online=%v latency=%v error=%v", status.IsOnline, status.Latency, status.Error)

	if status.IsOnline {
		if status.Latency <= 0 {
			t.Error("positive latency expected when online")
		}
		if status.Latency > 10*time.Second {
			t.Error("latency unusually high")
		}
		if status.Error != nil {
			t.Error("error should be nil when online")
		}
	} else {
		if status.Error == nil {
			t.Error("error should be non-nil when offline")
		}
	}
}

func TestCheckNetwork_Consistency(t *testing.T) {
	s1 := CheckNetwork()
	s2 := CheckNetwork()
	// Two consecutive calls should not crash
	if s1.IsOnline != s2.IsOnline && !s1.IsOnline && s2.IsOnline {
		// network just came back — fine
		t.Log("network state flapped between calls")
	}
}

// ── IsNetworkAvailable ──

func TestIsNetworkAvailable_ReturnsBool(t *testing.T) {
	available := IsNetworkAvailable()
	t.Logf("NetworkAvailable = %v", available)
	// Just verify it doesn't panic and returns a valid bool (no assertion on value)
}

// ── Benchmarks ──

func BenchmarkGetInternalIP(b *testing.B) {
	for range b.N {
		_, _ = GetInternalIP()
	}
}

func BenchmarkIsInternalIP(b *testing.B) {
	ip := net.ParseIP("192.168.1.1")
	b.ResetTimer()
	for range b.N {
		isInternalIP(ip)
	}
}

func BenchmarkCheckNetwork(b *testing.B) {
	for range b.N {
		CheckNetwork()
	}
}
