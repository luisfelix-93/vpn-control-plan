package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/luisfelix-93/vpn-control-plane/internal/domain"
)

type PeerUseCase struct {
	repo           domain.PeerRepository
	vpnManager     domain.VPNManager
	network        domain.Network
	serverPubkey   string
	serverEndpoint string
}

func NewPeerUseCase(
	repo  domain.PeerRepository,
	vpnManager domain.VPNManager,
	network domain.Network,
	serverPubkey string,
	serverEndpoint string,
) *PeerUseCase {
	return &PeerUseCase{
		repo: repo,
		vpnManager: vpnManager,
		network: network,
		serverPubkey: serverPubkey,
		serverEndpoint: serverEndpoint,
	}      
}

// RegisterNewPeer executa o fluxo completo para adicionar um novo dispositivo à rede
func (u *PeerUseCase) RegisterNewPeer(ctx context.Context, name string) (string, error) {
	// 1. Gera o par de chave (Privada e Pública) via adaptador de rede
	// Nota: a chave privada nunca será salva no banco, apenas entregue ao cliente agora.
	privKey, pubKey, err := u.vpnManager.GenerateKeyPair(ctx)
	if err != nil {
		return "", fmt.Errorf("falha ao gerar chaves: %w", err)
	}

	// 2. Cria um novo Peer
	id := uuid.New().String()
	peer, err := domain.NewPeer(id, name, pubKey)
	if err != nil {
		return "", fmt.Errorf("falha ao criar peer: %w", err)
	}

	// 3. Busca os IPs em uso e pede pro domínio calcular o próximo IP livre
	usedIPs, err := u.repo.GetUsedIPs(ctx)
	if err != nil {
		return "", fmt.Errorf("falha ao buscar IPs em uso: %w", err)
	}
	
	nextIP, err := u.network.FindNextAvailableIP(usedIPs)
	if err != nil {
		return "", fmt.Errorf("falha ao buscar próximo IP: %w", err)
	}

	// 4. Atribui o IP ao Peer
	if err := peer.AssignIP(nextIP); err != nil {
		return "", fmt.Errorf("falha ao atribuir IP: %w", err)
	}

	// 5. Aplica a regra na infraestrutura real (Linux)
	if err := u.vpnManager.AddPeer(ctx, pubKey, nextIP); err != nil {
		return "", fmt.Errorf("falha ao adicionar peer: %w", err)
	}

	// 6. Persiste o estado no banco de dados
	if err := u.repo.Save(ctx, peer); err != nil {
		return "", fmt.Errorf("falha ao salvar peer: %w", err)
	}

	// 7. Gear o arquivo de configuração para o cliente final
	clientConfig, err := u.vpnManager.GenerateClientConfig(ctx, peer, privKey, u.serverEndpoint, u.serverPubkey)
	if err != nil {
		return "", fmt.Errorf("falha ao gerar arquivo de configuração: %w", err)
	}

	return clientConfig, nil

}