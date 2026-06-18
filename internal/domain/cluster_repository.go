package domain

import "context"

type ClusterRepository interface {
	Save(ctx context.Context, cluster *Cluster) error
	FindByID(ctx context.Context, id string) (*Cluster, error)
}