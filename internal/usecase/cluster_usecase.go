package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/luisfelix-93/vpn-control-plane/internal/domain"
)

type ClusterUseCase struct {
	repo domain.ClusterRepository
}

type ReportLatencyPayload struct {
	TargetClusterID string  `json:"targetId"`
	LatencyMS       float64 `json:"latencyMs"`
}


func NewClusterUseCase(repo domain.ClusterRepository) *ClusterUseCase {
	return &ClusterUseCase{
		repo: repo,
	}
}

func (uc *ClusterUseCase) CreateCluster(ctx context.Context, name, cidr, interfaceName, pubKey, endpoint string) (*domain.Cluster, error) {
	id := uuid.New().String()
	
	cluster, err := domain.NewCluster(id, name, cidr, interfaceName, pubKey, endpoint)
	if err != nil {
		return nil, fmt.Errorf("dados de cluster inválidos: %w", err)
	}

	if err := uc.repo.Save(ctx, cluster); err != nil {
		return nil, fmt.Errorf("falha ao persistir o cluster: %w", err)
	}

	return cluster, nil
}

func (uc *ClusterUseCase) ProcessHeartbeat(ctx context.Context, id string) error {
	now := time.Now()
	err := uc.repo.RecordHeartbeat(ctx, id, domain.ClusterStatusOnline, now)
	if err != nil {
		return fmt.Errorf("falha ao processar heartbeat: %w", err)
	}
	return nil
}


func (uc *ClusterUseCase) ProcessLatencyReport(ctx context.Context, sourceID string, reports []ReportLatencyPayload) error {
	// Valida se o cluster de origem existe (para evitar inserções de nós fantasmas)
	_, err := uc.repo.FindByID(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("falha ao buscar laterncias do cluster %s: %w", sourceID, err)
	}

	for _, report := range reports {
		
		if sourceID == report.TargetClusterID {
			continue // Ignora latências para si mesmo
		}

		latency, err := domain.NewClusterLatency(sourceID, report.TargetClusterID, report.LatencyMS)
		if err != nil {
			continue // Ignora dados de latência inválidos
		}
		if err := uc.repo.RecordLatency(ctx, latency); err != nil {
			return fmt.Errorf("falha ao registrar latência para target %s: %w", report.TargetClusterID, err)
		}
	}
	return nil
}