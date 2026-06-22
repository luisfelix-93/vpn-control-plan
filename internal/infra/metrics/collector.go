package metrics

import (
	"context"
	"log"
	"time"

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
	TotalPeers = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "total_peers",
			Help: "Total de peers por cluster e por status",
		},
		[]string{"cluster_id", "status"},
	)
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

func (s *CollectorService) Start(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Encerrando coletor de métricas")
			return
		case <-ticker.C:
			s.collectMetrics(ctx)
		}
	}
}

func (s *CollectorService) collectMetrics(ctx context.Context) {
	clusters, err := s.clusterRepo.GetAll(ctx)
	if err != nil {
		TotalClusters.Set(float64(len(clusters)))

		for _, c := range clusters {
			ips, _ := s.peerRepo.GetUsedIPs(ctx, c.ID)
			TotalPeers.WithLabelValues(c.ID).Set(float64(len(ips)))
		}
	}

	log.Println("Métricas de negócio sincronizadas com o banco de dados.")
}
