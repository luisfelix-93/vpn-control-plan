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
	TotalPeers = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "total_peers",
			Help: "Total de peers por cluster e por status",
		},
		[]string{"cluster_id", "status"},
	)
	PeerStatusTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vpn_peers_status_total",
		Help: "Total de peers por cluster e status de saúde",
	}, []string{"cluster_id", "status"})
	PeerLastSeenUnix = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vpn_peer_last_seen_unix",
		Help: "Timestamp Unix do último sinal de vida por peer",
	}, []string{"cluster_id", "peer_id"})
	HealthCheckCycleDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "vpn_healthcheck_cycle_duration_seconds",
		Help:    "Duração de cada ciclo do health checker de peers",
		Buckets: prometheus.DefBuckets,
	})
	ClusterLastHeartbeatUnix = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vpn_cluster_last_heartbeat_unix",
		Help: "Timestamp Unix do último heartbeat recebido por cluster",
	}, []string{"cluster_id"})
	ClusterStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vpn_cluster_status",
		Help: "Status atual do cluster por label (1=ativo, 0=inativo)",
	}, []string{"cluster_id", "status"})
	ClusterHeartbeatsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "vpn_cluster_heartbeats_total",
		Help: "Quantidade total de heartbeats recebidos por cluster e resultado",
	}, []string{"cluster_id", "result"})
	ClusterHeartbeatAgeSeconds = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vpn_cluster_heartbeat_age_seconds",
		Help: "Idade, em segundos, do último heartbeat conhecido por cluster",
	}, []string{"cluster_id"})
	ClusterLatencyMS = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vpn_cluster_latency_ms",
		Help: "Latência em milissegundos reportada entre dois nós da rede",
	}, []string{"source_id", "target_id"})
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
		log.Printf("Erro ao coletar métricas de clusters: %v\n", err)
		return
	}

	TotalClusters.Set(float64(len(clusters)))

	ClusterLastHeartbeatUnix.Reset()
	ClusterStatus.Reset()
	ClusterHeartbeatAgeSeconds.Reset()

	now := time.Now()
	for _, c := range clusters {
		setClusterHeartbeatState(c.ID, c.Status, c.LastHeartbeat, now)
	}

	peers, err := s.peerRepo.GetAll(ctx)
	if err != nil {
		log.Printf("Erro ao coletar métricas de peers: %v\n", err)
		return
	}

	SyncPeerHealthMetrics(peers)

	log.Println("Métricas de negócio sincronizadas com o banco de dados.")
}

func ObserveHealthCheckCycleDuration(duration time.Duration) {
	HealthCheckCycleDuration.Observe(duration.Seconds())
}

func SyncPeerHealthMetrics(peers []*domain.Peer) {
	PeerStatusTotal.Reset()
	TotalPeers.Reset()
	PeerLastSeenUnix.Reset()

	statusByCluster := make(map[string]map[string]float64)

	for _, p := range peers {
		if p == nil {
			continue
		}

		clusterID := p.ClusterID
		if clusterID == "" {
			clusterID = "unknown"
		}

		status := normalizePeerStatus(p.Status)
		if _, ok := statusByCluster[clusterID]; !ok {
			statusByCluster[clusterID] = map[string]float64{
				domain.StatusOnline:  0,
				domain.StatusOffline: 0,
				domain.StatusUnknown: 0,
			}
		}

		statusByCluster[clusterID][status]++

		if !p.LastSeen.IsZero() {
			PeerLastSeenUnix.WithLabelValues(clusterID, p.ID).Set(float64(p.LastSeen.Unix()))
		}
	}

	for clusterID, counters := range statusByCluster {
		for _, status := range []string{domain.StatusOnline, domain.StatusOffline, domain.StatusUnknown} {
			value := counters[status]
			PeerStatusTotal.WithLabelValues(clusterID, status).Set(value)
			TotalPeers.WithLabelValues(clusterID, status).Set(value)
		}
	}
}

func IncClusterHeartbeatResult(clusterID, result string) {
	if clusterID == "" {
		clusterID = "unknown"
	}
	if result == "" {
		result = "success"
	}
	ClusterHeartbeatsTotal.WithLabelValues(clusterID, result).Inc()
}

func SetClusterHeartbeatState(clusterID, status string, lastHeartbeat time.Time) {
	setClusterHeartbeatState(clusterID, status, lastHeartbeat, time.Now())
}

func setClusterHeartbeatState(clusterID, status string, lastHeartbeat, now time.Time) {
	if clusterID == "" {
		clusterID = "unknown"
	}

	normalizedStatus := normalizeClusterStatus(status)

	for _, st := range []string{domain.ClusterStatusOnline, domain.ClusterStatusOffline, domain.ClusterStatusUnknown} {
		value := 0.0
		if st == normalizedStatus {
			value = 1
		}
		ClusterStatus.WithLabelValues(clusterID, st).Set(value)
	}

	if !lastHeartbeat.IsZero() {
		ClusterLastHeartbeatUnix.WithLabelValues(clusterID).Set(float64(lastHeartbeat.Unix()))
		age := now.Sub(lastHeartbeat).Seconds()
		if age < 0 {
			age = 0
		}
		ClusterHeartbeatAgeSeconds.WithLabelValues(clusterID).Set(age)
	}
}

func normalizePeerStatus(status string) string {
	switch status {
	case domain.StatusOnline, domain.StatusOffline, domain.StatusUnknown:
		return status
	default:
		return domain.StatusUnknown
	}
}

func normalizeClusterStatus(status string) string {
	switch status {
	case domain.ClusterStatusOnline, domain.ClusterStatusOffline, domain.ClusterStatusUnknown:
		return status
	default:
		return domain.ClusterStatusUnknown
	}
}

func (s *CollectorService) collectLatency(ctx context.Context) {
	// 1. Atualiza métricas de Clusters e Peers
	clusters, err := s.clusterRepo.GetAll(ctx)
	if err == nil {
		TotalClusters.Set(float64(len(clusters)))
		for _, c := range clusters {
			count, _ := s.peerRepo.CountByCluster(ctx, c.ID)
			TotalPeers.WithLabelValues(c.ID, domain.StatusUnknown).Set(float64(count))
		}
	} else {
		log.Printf("Erro ao coletar métricas de clusters: %v\n", err)
	}

	// 2. Atualiza a matriz global de latência
	latencies, err := s.clusterRepo.GetAllLatencies(ctx)
	if err == nil {
		for _, l := range latencies {
			ClusterLatencyMS.WithLabelValues(l.SourceClusterID, l.TargetClusterID).Set(l.LatencyMS)
		}
	} else {
		log.Printf("Erro ao coletar métricas de latência: %v\n", err)
	}
}
