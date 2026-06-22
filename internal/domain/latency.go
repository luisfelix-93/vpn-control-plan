package domain

import (
	"errors"
	"time"
)

var (
	ErrInvalidLatencyData = errors.New("dados de latência inválidos: source e target cluster IDs são obrigatórios")
)
type ClusterLatency struct {
	SourceClusterID string
	TargetClusterID string
	LatencyMS       float64
	MeasuredAt      time.Time
}

func NewClusterLatency(sourceID, targetID string, latencyMS float64) (*ClusterLatency, error) {
	if sourceID == "" || targetID == "" {
		return nil, ErrInvalidLatencyData
	}
	return &ClusterLatency{
		SourceClusterID: sourceID,
		TargetClusterID: targetID,
		LatencyMS:       latencyMS,
		MeasuredAt:      time.Now(),
	}, nil
}