package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/luisfelix-93/vpn-control-plane/internal/domain"
	"github.com/luisfelix-93/vpn-control-plane/internal/domain/routing"
)

type PeerUseCase struct {
	peerRepo    domain.PeerRepository
	clusterRepo domain.ClusterRepository
	vpnManager  domain.VPNManager
	routingStrategy routing.Strategy
}

func NewPeerUseCase(
	peerRepo domain.PeerRepository,
	clusterRepo domain.ClusterRepository,
	vpnManager domain.VPNManager,
	strategy routing.Strategy,
) *PeerUseCase {
	return &PeerUseCase{
		peerRepo:    peerRepo,
		clusterRepo: clusterRepo,
		vpnManager:  vpnManager,
		routingStrategy: strategy,
	}
}

func (uc *PeerUseCase) RegisterNewPeer(ctx context.Context, clusterID, peerName string) (string, error) {
	var targetCluster *domain.Cluster
	var err error

	// 1. Logica de Balaceamento de Clusters

	if clusterID != "" {
		// Roteamento estático: O cliente exigiu um nó específico
		targetCluster, err = uc.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return "", fmt.Errorf("cluster inválido ou não encontrado: %w", err)
	}
	} else {
		// Roteamento dinâmico: O Control Plane decide o melhor nó
		clusters, err := uc.clusterRepo.GetAll(ctx)
		if err != nil {
			return "", fmt.Errorf("falha ao obter clusters disponíveis: %w", err)
		}
		targetCluster, err = uc.routingStrategy.SelectBestCluster(clusters, nil)
		if err != nil {
			return "", fmt.Errorf("falha ao selecionar o melhor cluster: %w", err)
		}
	}
	
	// 2. Gerar par de chaves para o Peer
	privKey, pubKey, err := uc.vpnManager.GenerateKeyPair(ctx)
	if err != nil {
		return "", fmt.Errorf("falha ao gerar chaves: %w", err)
	}

	peerID := uuid.NewString()
	peer, err := domain.NewPeer(peerID, peerName, pubKey, targetCluster.ID)
	if err != nil {
		return "", fmt.Errorf("dados inválidos para o peer: %w", err)
	}

	// 4. IPAM Dinâmico: Busca IPs em uso apenas neste cluster específico
	usedIPs, err := uc.peerRepo.GetUsedIPs(ctx, targetCluster.ID)
	if err != nil {
		return "", fmt.Errorf("falha ao obter IPs usados: %w", err)
	}

	// Instancia a rede dinamicamente usando o CIDR do cluster (ex: 10.9.0.0/24)
	network := &domain.Network{CIDR: targetCluster.CIDR}
	nextIP, err := network.FindNextAvailableIP(usedIPs)
	if err != nil {
		return "", fmt.Errorf("falha ao encontrar próximo IP disponível: %w", err)
	}

	if err := peer.AssignIP(nextIP); err != nil {
		return "", fmt.Errorf("falha ao alocar IP: %w", err)
	}

	// 5. Aplica a regra no Linux apontando para a interface correta (ex: wg0-cloud)
	if err := uc.vpnManager.AddPeer(ctx, targetCluster.InterfaceName, peer.PublicKey, peer.AllocatedIP); err != nil {
		return "", fmt.Errorf("falha ao adicionar peer na interface: %w", err)
	}

	// 6. Persiste o estado no banco de dados
	if err := uc.peerRepo.Save(ctx, peer); err != nil {
		// Rollback direcionado para a interface correta
		_ = uc.vpnManager.RemovePeer(context.Background(), targetCluster.InterfaceName, peer.PublicKey)
		return "", fmt.Errorf("falha ao salvar peer: %w", err)
	}

	// 7. Retorna o arquivo de configuração (.conf) usando os dados do Cluster
	clientConfig, err := uc.vpnManager.GenerateClientConfig(ctx, peer, privKey, targetCluster.ServerPubKey, targetCluster.ServerEndpoint, targetCluster.CIDR)
	if err != nil {
		return "", fmt.Errorf("falha ao gerar configuração do cliente: %w", err)
	}
	return clientConfig, nil
}
