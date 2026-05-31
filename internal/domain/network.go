package domain

import (
	"errors"
	"net"
)

var (
	ErrNetworkFull = errors.New("Não há mais IPs disponíveis nesta sub-rede")
)

type Network struct {
	CIDR string
}

func (n *Network) FindNextAvailableIP(usedIPs []net.IP) (net.IP, error) {
	ip, ipNet, err := net.ParseCIDR(n.CIDR)
	if err != nil {
		return nil, err
	}

	usedMap := make(map[string]bool)
	for _, u := range usedIPs {
		usedMap[u.String()] = true
	}

	for ip := ip.Mask(ipNet.Mask); ipNet.Contains(ip); incrementIP(ip) {
		if ip[3] == 0 || ip[3] == 255 || ip[3] == 1 {
			continue
		}

		if !usedMap[ip.String()] {
			// Retorna uma cópia segura do IP
			available := make(net.IP, len(ip))
			copy(available, ip)
			return available, nil
		}

	}

	return nil, ErrNetworkFull
}

func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}