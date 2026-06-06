package wireguard

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net"
	"os/exec"
	"strings"

	"github.com/luisfelix-93/vpn-control-plane/internal/domain"
)

var _ domain.VPNManager = (*CLIAdapter)(nil)

type CLIAdapter struct {
	interfaceName string
}

func NewCLIAdapter(interfaceName string) *CLIAdapter {
	return &CLIAdapter{
		interfaceName: interfaceName,
	}
}

// AddPeer executa wg set wg0 peer <PUBKEY> allowed-ipsd <IP>/32

func (a *CLIAdapter) AddPeer(ctx context.Context, publicKey string, allowedIp net.IP) error {
	ipCIDR := fmt.Sprintf("%s/32", allowedIp.String())

	cmd := exec.CommandContext(ctx, "wg", "set", a.interfaceName, "peer", publicKey, "allowed-ips", ipCIDR)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("falha ao adcionar peer no host: %w, stderr: %s", err, strings.TrimSpace(stderr.String()))
	}

	return nil
}

// RemovePeer executa wg set wg0 peer <PUBKEY> remove
func (a *CLIAdapter) RemovePeer(ctx context.Context, publicKey string) error {
	cmd := exec.CommandContext(ctx, "wg", "set", a.interfaceName, "peer", publicKey, "remove")
	
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("falha ao remover peer do host: %w, stderr: %s", err, strings.TrimSpace(stderr.String()))
	}

	return nil
}

// GenerateKeyPair é um utilitário que o UseCase vai chamar antes de salvar o Peer

func (a *CLIAdapter) GenerateKeyPair(ctx context.Context) (privateKey, publicKey string, err error) {
	genCmd := exec.CommandContext(ctx, "wg", "genkey")
	privOut, err := genCmd.Output()

	if err != nil {
		return "", "", fmt.Errorf("falha ao gerar private key: %w", err)
	}

	privKey := strings.TrimSpace(string(privOut))

	pubCmd := exec.CommandContext(ctx, "wg", "pubkey")
	pubCmd.Stdin = strings.NewReader(privKey)
	pubOut, err := pubCmd.Output()

	if err != nil {
		return "", "", fmt.Errorf("falha ao gerar public key: %w", err)
	}

	pubKey := strings.TrimSpace(string(pubOut))

	return privKey, pubKey, nil
}

// GenerateClientConfig renderiza o payload que será entregue ao usuário (ex: via QR Code)
func (a *CLIAdapter) GenerateClientConfig(ctx context.Context, peer *domain.Peer, clientPrivateKey, serverPrivateKey, serverEndpoint string) (string, error) {
	const confTmpl = `[Interface]
PrivateKey = {{.ClientPrivateKey}}
Address = {{.ClientIP}}/32
DNS = 10.8.0.1 # Idealmente o IP do seu CoreDNS ou Pi-hole interno

[Peer]
PublicKey = {{.ServerPublicKey}}
Endpoint = {{.ServerEndpoint}}
# AllowedIPs define o roteamento. Se quiser split-tunnel (apenas acesso ao lab), coloque a sub-rede do lab.
# Se quiser Full Tunnel (todo tráfego de internet via sua casa), use 0.0.0.0/0
AllowedIPs = 10.8.0.0/24, 192.168.1.0/24
PersistentKeepalive = 25
`
	t, err := template.New("wgconf").Parse(confTmpl)
	if err != nil {
		return "", fmt.Errorf("falha ao criar template: %w", err)
	}

	data := struct {
		ClientPrivateKey string
		ClientIP         string
		ServerPublicKey  string
		ServerEndpoint   string
	}{
		ClientPrivateKey: clientPrivateKey,
		ClientIP:         peer.AllocatedIP.String(),
		ServerPublicKey:  serverPrivateKey,
		ServerEndpoint:   serverEndpoint,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("falha ao renderizar template: %w", err)
	}

	return buf.String(), nil

}