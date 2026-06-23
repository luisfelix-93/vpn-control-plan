package health

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/luisfelix-93/vpn-control-plane/internal/domain"
	"github.com/luisfelix-93/vpn-control-plane/internal/infra/metrics"
	"github.com/luisfelix-93/vpn-control-plane/internal/infra/network"
)

type CheckerService struct {
	peerRepo domain.PeerRepository
	pinger   *network.Pinger
}

func NewCheckerService(repo domain.PeerRepository, pinger *network.Pinger) *CheckerService {
	return &CheckerService{
		peerRepo: repo,
		pinger:   pinger,
	}
}

func (s *CheckerService) Start(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Encerrando Health Checker...")
			return
		case <-ticker.C:
			s.checkPeers(ctx)
		}
	}
}

func (s *CheckerService) checkPeers(ctx context.Context) {
	start := time.Now()
	defer func() {
		metrics.ObserveHealthCheckCycleDuration(time.Since(start))
	}()

	peers, err := s.peerRepo.GetAll(ctx)
	if err != nil {
		log.Println("Erro ao obter peers:", err)
		return
	}

	var wg sync.WaitGroup

	for _, p := range peers {
		// Ignoramos peers revogados, não há por que pingá-los
		if p.IsRevoked || p.AllocatedIP == nil {
			continue
		}

		wg.Add(1)

		// Dispara a checagem em uma goroutine separada
		go func(peer *domain.Peer) {
			defer wg.Done()

			isAlive := s.pinger.Ping(ctx, peer.AllocatedIP.String())

			status := domain.StatusOffline
			lastSeen := peer.LastSeen // Mantém o LastSeen atual caso o peer esteja offline

			if isAlive {
				status = domain.StatusOnline
				lastSeen = time.Now()
			}

			// Só atualiza o banco se houver mudança de status ou se ficou online (para atualizar o LastSeen)
			if peer.Status != status || isAlive {
				err := s.peerRepo.UpdateHealthStatus(ctx, peer.ID, status, lastSeen)
				if err != nil {
					log.Printf("Erro ao atualizar status do peer %s: %v", peer.Name, err)
				}
			}
		}(p) // Passamos o ponteiro como argumento para evitar problemas de closure no loop
	}

	// Aguarda todos os pings finalizarem antes de encerrar este ciclo
	wg.Wait()

	updatedPeers, err := s.peerRepo.GetAll(ctx)
	if err != nil {
		log.Println("Erro ao obter peers para sincronizar métricas:", err)
		return
	}

	metrics.SyncPeerHealthMetrics(updatedPeers)
}
