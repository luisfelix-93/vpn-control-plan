package metrics

import (
	"github.com/luisfelix-93/vpn-control-plane/internal/domain"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Definicção das métricas (Gauges são ideais para valores que sobe e descem)
var (
	TotalClusters = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "vpn_clusters_total",
		Help: "Número total de clusters registrados",
	})
	TotalPeers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "vpn_peers_total",
		Help: "Número total de peers registrados",
	})
)

// CollectorService encapsula a lógica de varredura de métricas

type CollectorService struct {
	clusterRepo domain.ClusterRepository
	peerRepo    domain.PeerRepository
}

func NewCollectorService(cRepo domain.ClusterRepository, pRepo domain.PeerRepository) *CollectorService {
	return &CollectorService{
		clusterRepo: cRepo,
		peerRepo:    pRepo,
	}
}




