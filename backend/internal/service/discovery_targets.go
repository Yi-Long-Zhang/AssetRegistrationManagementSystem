package service

import (
	"fmt"
	"net"
	"strings"
)

func expandTargets(targets string, maxHosts int) ([]string, error) {
	if maxHosts <= 0 {
		maxHosts = 1024
	}
	var ips []string
	seen := map[string]bool{}
	add := func(ip string) {
		if seen[ip] {
			return
		}
		seen[ip] = true
		ips = append(ips, ip)
	}
	for _, t := range splitTargets(targets) {
		if strings.Contains(t, "/") {
			ip, ipnet, err := net.ParseCIDR(t)
			if err != nil {
				return nil, fmt.Errorf("invalid CIDR %s: %w", t, err)
			}
			for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); incIP(ip) {
				add(ip.String())
				if len(ips) >= maxHosts {
					return nil, fmt.Errorf("targets exceed max hosts (%d): %s", maxHosts, t)
				}
			}
		} else {
			ip := net.ParseIP(strings.TrimSpace(t))
			if ip == nil {
				return nil, fmt.Errorf("invalid target: %s", t)
			}
			add(ip.String())
		}
	}
	return ips, nil
}

// incIP 将 IPv4 地址字节 +1（用于 CIDR 遍历）。
func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
