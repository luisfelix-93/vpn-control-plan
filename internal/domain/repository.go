package domain

import (
	"context"
	"net"
	"time"
)

type PeerRepository interface {
	InitSchema(ctx context.Context) error
	Save(ctx context.Context, peer *Peer) error
	FindByID(ctx context.Context, id string) (*Peer, error)
	GetAll(ctx context.Context) ([]*Peer, error)
	GetUsedIPs(ctx context.Context, clusterID string) ([]net.IP, error)
	CountByCluster(ctx context.Context, clusterID string) (int, error)
	UpdateHealthStatus(ctx context.Context, id string, status string, lastSeen time.Time) error
}
