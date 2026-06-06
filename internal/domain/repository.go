package domain

import (
	"context"
	"net"
)

type PeerRepository interface {
	InitSchema(ctx context.Context) error
	Save(ctx context.Context, peer *Peer) error
	FindByID(ctx context.Context, id string) (*Peer, error)
	GetUsedIPs(ctx context.Context) ([]net.IP, error)
}
