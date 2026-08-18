package service

import (
	"encoding/xml"
	"fmt"
	"path/filepath"
	"sort"
)

type ScanResult struct {
	IP        string   `json:"ip"`
	MAC       string   `json:"mac"`
	Hostname  string   `json:"hostname"`
	Status    string   `json:"status"` // up / down
	OpenPorts []string `json:"openPorts"`
	Services  []string `json:"services"`
	OS        string   `json:"os"`
}

// nmap XML 解析结构（-oX - 输出）
type nmapRun struct {
	Hosts []nmapHost `xml:"host"`
}

type nmapHost struct {
	Status    nmapStatus    `xml:"status"`
	Addresses []nmapAddress `xml:"address"`
	Hostnames struct {
		Names []nmapHostname `xml:"hostname"`
	} `xml:"hostnames"`
	Ports struct {
		Ports []nmapPort `xml:"port"`
	} `xml:"ports"`
	OS struct {
		Matches []nmapOSMatch `xml:"osmatch"`
	} `xml:"os"`
}

type nmapStatus struct {
	State string `xml:"state,attr"`
}

type nmapAddress struct {
	Addr     string `xml:"addr,attr"`
	AddrType string `xml:"addrtype,attr"`
}

type nmapHostname struct {
	Name string `xml:"name,attr"`
}

type nmapPort struct {
	Protocol string        `xml:"protocol,attr"`
	PortID   string        `xml:"portid,attr"`
	State    nmapPortState `xml:"state"`
	Service  nmapService   `xml:"service"`
}

type nmapPortState struct {
	State string `xml:"state,attr"`
}

type nmapService struct {
	Name    string `xml:"name,attr"`
	Product string `xml:"product,attr"`
	Version string `xml:"version,attr"`
}

type nmapOSMatch struct {
	Name string `xml:"name,attr"`
}

// ParseNmapXML 将 nmap -oX 输出解析为有序的 ScanResult 列表（按 IP 排序）。
func ParseNmapXML(data []byte) ([]ScanResult, error) {
	var run nmapRun
	if err := xml.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("parse nmap xml: %w", err)
	}
	results := make([]ScanResult, 0, len(run.Hosts))
	for _, h := range run.Hosts {
		res := ScanResult{Status: h.Status.State}
		for _, a := range h.Addresses {
			switch a.AddrType {
			case "ipv4":
				if res.IP == "" {
					res.IP = a.Addr
				}
			case "mac":
				if res.MAC == "" {
					res.MAC = a.Addr
				}
			}
		}
		if res.IP == "" && len(h.Addresses) > 0 {
			res.IP = h.Addresses[0].Addr
		}
		if len(h.Hostnames.Names) > 0 {
			res.Hostname = h.Hostnames.Names[0].Name
		}
		if len(h.OS.Matches) > 0 {
			res.OS = h.OS.Matches[0].Name
		}
		var ports, services []string
		for _, p := range h.Ports.Ports {
			if p.State.State != "open" {
				continue
			}
			ports = append(ports, p.PortID+"/"+p.Protocol)
			s := p.PortID + "/" + p.Protocol + ": " + p.Service.Name
			if p.Service.Product != "" {
				s += " " + p.Service.Product
			}
			if p.Service.Version != "" {
				s += " " + p.Service.Version
			}
			services = append(services, s)
		}
		sort.Strings(ports)
		sort.Strings(services)
		res.OpenPorts = ports
		res.Services = services
		results = append(results, res)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].IP < results[j].IP })
	return results, nil
}

func absPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// BuildNmapArgs 依据规则与全局配置组装 nmap 参数（切片传参，杜绝 shell 拼接注入）。
