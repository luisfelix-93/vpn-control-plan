package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/luisfelix-93/vpn-control-plane/internal/domain"
)

type PeerUseCase struct {
	peerRepo domain.PeerRepository
	clusterRepo domain.ClusterRepository
	vpnManager domain.VPNManager
}


func NewPeerUseCase(
	peerRepo    domain.PeerRepository,
	clusterRepo domain.ClusterRepository,
	vpnManager  domain.VPNManager,
) *PeerUseCase {
	return &PeerUseCase{
		peerRepo:    peerRepo,
		clusterRepo: clusterRepo,
		vpnManager:  vpnManager,
	}
}

func (uc *PeerUseCase) RegisterNewPeer(ctx context.Context, clusterID, peerName string) (string, error) {
	// 1. Valida se o Cluster (Zona de Rede) realmente existe
	cluster, err := uc.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return "", fmt.Errorf("cluster inválido ou não encontrado: %w", err)
	}

	// 2. Gerar par de chaves para o Peer
	privKey, pubKey, err := uc.vpnManager.GenerateKeyPair(ctx)
	if err != nil {
		return "", fmt.Errorf("falha ao gerar chaves: %w", err)
	}

	peer, err := domain.NewPeer(peerID, peerName, pubKey, cluster.ID)
		return "", fmt.Errorf("dados inválidos para o peer: %w", err)
	}

	// 4. IPAM Dinâmico: Busca IPs em uso apenas neste cluster específico
	usedIPs, err := uc.peerRepo.GetUsedIPs(ctx, cluster.ID)
	if err != nil {
		return "", fmt.Errorf("falha ao obter IPs usados: %w", err)
	}

	// Instancia a rede dinamicamente usando o CIDR do cluster (ex: 10.9.0.0/24)
	network := &domain.Network{CIDR: cluster.CIDR}
	nextIP, err := network.FindNextAvailableIP(usedIPs)
	if err != nil {
		return "", fmt.Errorf("falha ao encontrar próximo IP disponível: %w", err)
	}

	if err := peer.AssignIP(nextIP); err != nil {
		return "", fmt.Errorf("falha ao alocar IP: %w", err)
	}

	// 5. Aplica a regra no Linux apontando para a interface correta (ex: wg0-cloud)
	if err := uc.vpnManager.AddPeer(ctx, cluster.InterfaceName, peer.PublicKey, peer.AllocatedIP); err != nil {
		return "", fmt.Errorf("falha ao adicionar peer na interface: %w", err)
	}

	// 6. Persiste o estado no banco de dados
	if err := uc.peerRepo.Save(ctx, peer); err != nil {
		// Rollback direcionado para a interface correta
		_ = uc.vpnManager.RemovePeer(context.Background(), cluster.InterfaceName, peer.PublicKey)
		return "", fmt.Errorf("falha ao salvar peer: %w", err)
	}

	// 7. Retorna o arquivo de configuração (.conf) usando os dados do Cluster
	clientConfig, err := uc.vpnManager.GenerateClientConfig(ctx, peer, privKey, cluster.ServerPubKey, cluster.ServerEndpoint)
	if err != nil {
		return "", fmt.Errorf("falha ao gerar configuração do cliente: %w", err)
	}
	return clientConfig, nil
}
