package domain

import (
	"errors"
	"net"
	"time"
)

var (
	ErrInvalidPeerData = errors.New("nome e chave pública são obrugatórios")
	ErrPeerRevoked = errors.New("Não é possível operar em um peer revogado")
)

const (
	StatusOnline = "online"
	StatusOffline = "offline"
	StatusUnknown = "unknown"
)

type Peer struct {
	ID         		string
	Name       		string
	ClusterID  		string
	PublicKey  		string
	AllocatedIP     net.IP
	IsRevoked       bool
	Status          string
	LastSeen        time.Time
	CreatedAt       time.Time
}

func NewPeer(id, name, publicKey, clusterID string) (*Peer, error) {
	if name == "" || publicKey == "" {
		return nil, ErrInvalidPeerData
	}

	return &Peer{
		ID:         id,
		Name:       name,
		PublicKey:  publicKey,
		ClusterID: clusterID,
		// AllocatedIP: nil,
		IsRevoked:  false,
		Status:   StatusUnknown,
		CreatedAt:  time.Now(),
	}, nil
}


func (p *Peer) AssignIP(ip net.IP) error {
    if p.IsRevoked {
        return ErrPeerRevoked
    }
    p.AllocatedIP = ip
    return nil
}

func (p *Peer) Revoke() {
	p.IsRevoked = true
}
