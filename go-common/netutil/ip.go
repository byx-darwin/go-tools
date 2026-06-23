package netutil

import (
	"net"
)

// GetInternalIP 获取本机内网IP地址
// 如果有多个内网IP，返回第一个找到的内网IP
// 如果没有找到内网IP，返回空字符串和错误
func GetInternalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				if isInternalIP(ipnet.IP) {
					return ipnet.IP.String(), nil
				}
			}
		}
	}
	return "", nil
}

// isInternalIP 判断是否为内网IP
// 内网IP范围：
// A类: 10.0.0.0--10.255.255.255
// B类: 172.16.0.0--172.31.255.255
// C类: 192.168.0.0--192.168.255.255
func isInternalIP(ip net.IP) bool {
	if ip.IsLoopback() {
		return false
	}

	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 10 {
			return true
		}
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true
		}
		if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
		return false
	}

	if ip.IsPrivate() {
		return true
	}
	return false
}
