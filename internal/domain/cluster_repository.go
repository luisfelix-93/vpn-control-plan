package domain

import (
	"context"
	"time"
)

type ClusterRepository interface {
	Save(ctx context.Context, cluster *Cluster) error
	FindByID(ctx context.Context, id string) (*Cluster, error)
	GetAll(ctx context.Context) ([]*Cluster, error)
	RecordHeartbeat(ctx context.Context, id, status string, timestamp time.Time) error

	RecordLatency(ctx context.Context, latency *ClusterLatency) error
	GetLatencyFrom(ctx context.Context, sourceID string) ([]*ClusterLatency, error)
}