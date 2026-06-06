package domain

import (
	"context"
	"net"
)

type VPNManager interface {
	AddPeer(ctx context.Context, publicKey string, allowedIP net.IP) error

	RemovePeer(ctx context.Context, publicKey string) error

	GenerateClientConfig(ctx context.Context, peer *Peer, clientPrivateKey, serverPrivateKey, serverEndpoint string) (string, error)

	GenerateKeyPair(ctx context.Context) (privateKey, publicKey string, err error)
}
