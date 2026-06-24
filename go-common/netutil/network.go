package netutil

import (
	"context"
	"net"
	"time"
)

// NetworkStatus 表示网络状态检查的结果
type NetworkStatus struct {
	IsOnline bool          // 是否在线
	Latency  time.Duration // 延迟时间
	Error    error         // 如果发生错误，这里会包含错误信息
}

// CheckNetwork 检查网络是否可用
// 通过尝试连接多个可靠的公共DNS服务器来检测网络状态
// 返回 NetworkStatus 包含连接状态、延迟时间和可能的错误
func CheckNetwork() NetworkStatus {
	hosts := []string{
		"8.8.8.8:53",         // Google DNS
		"114.114.114.114:53", // 114 DNS
		"223.5.5.5:53",       // AliDNS
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for _, host := range hosts {
		start := time.Now()

		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", host)
		if err == nil {
			_ = conn.Close()
			return NetworkStatus{
				IsOnline: true,
				Latency:  time.Since(start),
			}
		}
	}

	return NetworkStatus{
		IsOnline: false,
		Error:    context.DeadlineExceeded,
	}
}

// IsNetworkAvailable 快速检查网络是否可用
func IsNetworkAvailable() bool {
	return CheckNetwork().IsOnline
}
