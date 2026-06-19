package domain

import (
	"context"
	"net"
)

type VPNManager interface {
	AddPeer(ctx context.Context, interfaceName, publicKey string, allowedIP net.IP) error
	RemovePeer(ctx context.Context, interfaceName, publicKey string) error
	GenerateClientConfig(ctx context.Context, peer *Peer, clientPrivateKey, serverPublicKey, serverEndpoint, allowedIPs string) (string, error)
	GenerateKeyPair(ctx context.Context) (privateKey, publicKey string, err error)
}
