package routing

import (
	"errors"

	"github.com/luisfelix-93/vpn-control-plane/internal/domain"
)

var ErrNoClustersAvailable = errors.New("nenhum cluster disponível para roteamento")

// Strategy define o contrato para os algoritmos de balanceamento
//
type Strategy interface {
	SelectBestCluster(clusters []*domain.Cluster, activePeersCount map[string]int) (*domain.Cluster, error)
}

type LeastConnectionStrategy struct {}

func NewLeastConnectionStrategy() *LeastConnectionStrategy {
	return &LeastConnectionStrategy{}
}

func (s *LeastConnectionStrategy) SelectBestCluster(clusters []*domain.Cluster, activePeersCount map[string]int) (*domain.Cluster, error) {
	if len(clusters) == 0 {
		return nil, ErrNoClustersAvailable
	}

	var bestCluster *domain.Cluster
	minPeers := int(^uint(0) >> 1)

	for _, c := range clusters {
		if c.Status == domain.ClusterStatusOffline {
			continue
		}

		count := activePeersCount[c.ID]
		if count < minPeers {
			minPeers = count
			bestCluster = c
		}
	}

	if bestCluster == nil {
		return nil, ErrNoClustersAvailable
	}

	return bestCluster, nil
}