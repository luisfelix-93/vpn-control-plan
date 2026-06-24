package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/luisfelix-93/vpn-control-plane/internal/domain"
)

var _ domain.ClusterRepository = (*ClusterRepositoryImpl)(nil)

type ClusterRepositoryImpl struct {
	db *sql.DB
}

func NewClusterRepository(db *sql.DB) *ClusterRepositoryImpl {
	return &ClusterRepositoryImpl{
		db: db,
	}
}

// InitClusterSchema cria a tabela principal de zonas de rede
func (r *ClusterRepositoryImpl) InitSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS clusters (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		cidr TEXT NOT NULL,
		interface_name TEXT NOT NULL,
		server_pub_key TEXT NOT NULL,
		server_endpoint TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'unknown',
		last_heartbeat DATETIME,
		created_at DATETIME NOT NULL
	);`
	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("falha ao inicializar schema do sqlite: %w", err)
	}

	queryLatencies := `
	CREATE TABLE IF NOT EXISTS cluster_latencies (
		source_id TEXT NOT NULL,
		target_id TEXT NOT NULL,
		latency_ms REAL NOT NULL,
		measured_at DATETIME NOT NULL,
		PRIMARY KEY (source_id, target_id),
		FOREIGN KEY(source_id) REFERENCES clusters(id) ON DELETE CASCADE,
		FOREIGN KEY(target_id) REFERENCES clusters(id) ON DELETE CASCADE
	);`

	_, err = r.db.ExecContext(ctx, queryLatencies)
	if err != nil {
		return fmt.Errorf("falha ao inicializar schema de latencias: %w", err)
	}

	return nil

}

// Save permite cadastrar ou atualizar um cluster
func (r *ClusterRepositoryImpl) Save(ctx context.Context, cluster *domain.Cluster) error {
	query := `
	INSERT INTO clusters (id, name, cidr, interface_name, server_pub_key, server_endpoint, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		cidr = excluded.cidr,
		interface_name = excluded.interface_name,
		server_pub_key = excluded.server_pub_key,
		server_endpoint = excluded.server_endpoint;
	`

	_, err := r.db.ExecContext(ctx, query,
		cluster.ID, cluster.Name, cluster.CIDR, cluster.InterfaceName,
		cluster.ServerPubKey, cluster.ServerEndpoint, cluster.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("falha ao salvar cluster: %w", err)
	}
	return nil
}

// FindByID busca a configuração da rede para o UseCase utilizar
func (r *ClusterRepositoryImpl) FindByID(ctx context.Context, id string) (*domain.Cluster, error) {
	query := `
	SELECT id, name, cidr, interface_name, server_pub_key, server_endpoint, status, last_heartbeat, created_at 
	FROM clusters WHERE id = ?`

	row := r.db.QueryRowContext(ctx, query, id)

	var c domain.Cluster
	var lastHeartbeat sql.NullString
	var createdAt string

	err := row.Scan(&c.ID, &c.Name, &c.CIDR, &c.InterfaceName, &c.ServerPubKey, &c.ServerEndpoint, &c.Status, &lastHeartbeat, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("cluster não encontrado")
		}
		return nil, fmt.Errorf("falha ao buscar cluster: %w", err)
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("falha ao fazer parse de created_at do cluster %s: %w", id, err)
	}
	c.CreatedAt = parsedCreatedAt

	if lastHeartbeat.Valid {
		parsedHeartbeat, err := time.Parse(time.RFC3339, lastHeartbeat.String)
		if err != nil {
			return nil, fmt.Errorf("falha ao fazer parse de last_heartbeat do cluster %s: %w", id, err)
		}
		c.LastHeartbeat = parsedHeartbeat
	}
	return &c, nil
}

func (r *ClusterRepositoryImpl) GetAll(ctx context.Context) ([]*domain.Cluster, error) {
	query := `
	SELECT id, name, cidr, interface_name, server_pub_key, server_endpoint, status, last_heartbeat, created_at 
	FROM clusters`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar clusters: %w", err)
	}
	defer rows.Close()

	var clusters []*domain.Cluster
	for rows.Next() {
		var c domain.Cluster
		var lastHeartbeat sql.NullString
		var createdAt string
		if err := rows.Scan(&c.ID, &c.Name, &c.CIDR, &c.InterfaceName, &c.ServerPubKey, &c.ServerEndpoint, &c.Status, &lastHeartbeat, &createdAt); err != nil {
			return nil, fmt.Errorf("falha ao escanear cluster: %w", err)
		}
		parsedCreatedAt, err := time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("falha ao fazer parse de created_at do cluster %s: %w", c.ID, err)
		}
		c.CreatedAt = parsedCreatedAt

		if lastHeartbeat.Valid {
			parsedHeartbeat, err := time.Parse(time.RFC3339, lastHeartbeat.String)
			if err != nil {
				return nil, fmt.Errorf("falha ao fazer parse de last_heartbeat do cluster %s: %w", c.ID, err)
			}
			c.LastHeartbeat = parsedHeartbeat
		}
		clusters = append(clusters, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erro ao iterar sobre os clusters: %w", err)
	}
	return clusters, nil
}

func (r *ClusterRepositoryImpl) RecordHeartbeat(ctx context.Context, id, status string, timestamp time.Time) error {
	query := `UPDATE clusters SET status = ?, last_heartbeat = ? WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, status, timestamp.Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("falha ao registrar heartbeat: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("falha ao obter número de linhas afetadas: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("cluster não encontrado")
	}
	return nil
}

func (r *ClusterRepositoryImpl) RecordLatency(ctx context.Context, latency *domain.ClusterLatency) error {

	query := `
	INSERT INTO cluster_latencies (source_id, target_id, latency_ms, measured_at)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(source_id, target_id) DO UPDATE SET
		latency_ms = excluded.latency_ms,
		measured_at = excluded.measured_at;
	`
	_, err := r.db.ExecContext(ctx, query, latency.SourceClusterID, latency.TargetClusterID, latency.LatencyMS, latency.MeasuredAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("falha ao registrar latência: %w", err)
	}
	return nil
}

func (r *ClusterRepositoryImpl) GetLatencyFrom(ctx context.Context, sourceID string) ([]*domain.ClusterLatency, error) {
	query := `SELECT source_id, target_id, latency_ms, measured_at FROM cluster_latencies WHERE source_id = ?`
	rows, err := r.db.QueryContext(ctx, query, sourceID)
	if err != nil {
		return nil, fmt.Errorf("falha ao consultar latências: %w", err)
	}
	defer rows.Close()

	var latencies []*domain.ClusterLatency
	for rows.Next() {
		var l domain.ClusterLatency
		var measuredAt string
		if err := rows.Scan(&l.SourceClusterID, &l.TargetClusterID, &l.LatencyMS, &measuredAt); err != nil {
			return nil, fmt.Errorf("falha ao escanear latência: %w", err)
		}
		parsedMeasuredAt, err := time.Parse(time.RFC3339, measuredAt)
		if err != nil {
			return nil, fmt.Errorf("falha ao fazer parse de measured_at na latência %s->%s: %w", l.SourceClusterID, l.TargetClusterID, err)
		}
		l.MeasuredAt = parsedMeasuredAt
		latencies = append(latencies, &l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erro ao iterar sobre as latências: %w", err)
	}
	return latencies, nil
}

// GetAllLatencies executa um full scan super leve na tabela de relacionamento
func (r *ClusterRepositoryImpl) GetAllLatencies(ctx context.Context) ([]*domain.ClusterLatency, error) {
	query := `SELECT source_id, target_id, latency_ms, measured_at FROM cluster_latencies`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar matriz global de latencias: %w", err)
	}
	defer rows.Close()

	var latencies []*domain.ClusterLatency
	for rows.Next() {
		var l domain.ClusterLatency
		var measuredAt string

		if err := rows.Scan(&l.SourceClusterID, &l.TargetClusterID, &l.LatencyMS, &measuredAt); err != nil {
			return nil, err
		}

		parsedMeasuredAt, err := time.Parse(time.RFC3339, measuredAt)
		if err != nil {
			return nil, fmt.Errorf("falha ao fazer parse de measured_at na latência %s->%s: %w", l.SourceClusterID, l.TargetClusterID, err)
		}
		l.MeasuredAt = parsedMeasuredAt
		latencies = append(latencies, &l)
	}

	return latencies, nil
}
