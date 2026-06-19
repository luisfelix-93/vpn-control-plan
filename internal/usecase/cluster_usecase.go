package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/luisfelix-93/vpn-control-plane/internal/domain"
)

type ClusterUseCase struct {
	repo domain.ClusterRepository
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