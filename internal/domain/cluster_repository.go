package domain

import "context"

type ClusterRepository interface {
	FindByID(ctx context.Context, id string) (*Cluster, error)
}