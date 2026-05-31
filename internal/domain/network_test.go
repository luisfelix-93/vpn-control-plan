package domain

import (
	"net"
	"testing"
)

func TestNetwork_FindNextAvailableIP(t *testing.T) {
	// Table-driven tests: definimos os cenários em um slice de structs
	tests := []struct {
		name      string
		cidr      string
		usedIPs   []net.IP
		wantIP    string
		wantError error
	}{
		{
			name:      "Deve retornar o primeiro IP disponivel (10.8.0.2)",
			cidr:      "10.8.0.0/24",
			usedIPs:   []net.IP{},
			wantIP:    "10.8.0.2",
			wantError: nil,
		},
		{
			name:      "Deve pular IPs em uso e entregar o proximo livre",
			cidr:      "10.8.0.0/24",
			usedIPs:   []net.IP{net.ParseIP("10.8.0.2"), net.ParseIP("10.8.0.3")},
			wantIP:    "10.8.0.4",
			wantError: nil,
		},
		{
			name:      "Deve retornar erro se a rede estiver cheia (/30)",
			// Uma rede /30 tem 4 IPs (ex: .0 rede, .1 gateway, .2 host, .3 broadcast)
			// Como pulamos o .1, só sobra o .2. Se ele estiver em uso, a rede está cheia.
			cidr:      "192.168.1.0/30",
			usedIPs:   []net.IP{net.ParseIP("192.168.1.2")},
			wantIP:    "",
			wantError: ErrNetworkFull,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network := &Network{CIDR: tt.cidr}
			gotIP, err := network.FindNextAvailableIP(tt.usedIPs)

			// Valida o erro esperado
			if err != tt.wantError {
				t.Fatalf("esperava erro '%v', recebeu '%v'", tt.wantError, err)
			}

			// Valida o IP gerado
			if gotIP != nil && gotIP.String() != tt.wantIP {
				t.Errorf("esperava IP %v, recebeu %v", tt.wantIP, gotIP.String())
			}
		})
	}
}