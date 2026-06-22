package domain

import (
	"errors"
	"time"
)

var (
	ErrInvalidClusterData = errors.New("nome, cidr e interface são obrigatórios para um cluster")
)

const (
	ClusterStatusOnline = "online"
	ClusterStatusOffline = "offline"
	ClusterStatusUnknown = "unknown"
)

// Cluster representa uma zona de rede isolada

type Cluster struct {
	ID              string
	Name            string
	CIDR            string
	InterfaceName   string
	ServerPubKey    string
	ServerEndpoint  string
	Status          string
	LastHeartbeat   time.Time
	CreatedAt       time.Time
}

// NewCluster é a factory para garantir a validade da entidade
func NewCluster(id, name, cidr, interfaceName, pubKey, endpoint string) (*Cluster, error) {
	if name == "" || cidr == "" || interfaceName == "" {
		return nil, ErrInvalidClusterData
	}

	return &Cluster{
		ID:             id,
		Name:           name,
		CIDR:           cidr,
		InterfaceName:  interfaceName,
		ServerPubKey:   pubKey,
		ServerEndpoint: endpoint,
		Status:         ClusterStatusUnknown,
		CreatedAt:      time.Now(),
	}, nil
}