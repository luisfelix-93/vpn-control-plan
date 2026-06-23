package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/luisfelix-93/vpn-control-plane/internal/domain"
	_ "github.com/mattn/go-sqlite3"
)

var _ domain.PeerRepository = (*PeerRepository)(nil)

type PeerRepository struct {
	db *sql.DB
}

func NewPeerRepository(db *sql.DB) *PeerRepository {
	return &PeerRepository{
		db: db,
	}
}

// InitSchema é um utilitário prático para criarmos a tabela assim que a aplicação subir
// dispensando ferramentas complexas de migração para um projeto desse escopo

func (r *PeerRepository) InitSchema(ctx context.Context) error {
	r.db.ExecContext(ctx, "PRAGMA foreign_keys = ON;")
	query := `
	CREATE TABLE IF NOT EXISTS peers (
		id TEXT PRIMARY KEY,
		cluster_id TEXT NOT NULL,
		name TEXT NOT NULL,
		public_key TEXT NOT NULL UNIQUE,
		allocated_ip TEXT NOT NULL UNIQUE,
		is_revoked INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'unknown',
		last_seen DATETIME,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (cluster_id) REFERENCES clusters(id) ON DELETE CASCADE
	);`

	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("falha ao inicializar schema do sqlite: %w", err)
	}

	return nil
}

//Save insere um novo peer ou atualiza um existente usando UPSERT (ON CONFLICT)

func (r *PeerRepository) Save(ctx context.Context, peer *domain.Peer) error {
	query := `
	INSERT INTO peers (id, cluster_id, name, public_key, allocated_ip, is_revoked, status, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		cluster_id = excluded.cluster_id,
		name = excluded.name,
		public_key = excluded.public_key,
		allocated_ip = excluded.allocated_ip,
		is_revoked = excluded.is_revoked,
		status = excluded.status;
	`

	isRevokedInt := 0
	if peer.IsRevoked {
		isRevokedInt = 1
	}

	var ipStr string
	if peer.AllocatedIP != nil {
		ipStr = peer.AllocatedIP.String()
	}

	status := peer.Status
	if status == "" {
		status = domain.StatusUnknown
	}

	_, err := r.db.ExecContext(ctx, query, peer.ID, peer.ClusterID, peer.Name, peer.PublicKey, ipStr, isRevokedInt, status, peer.CreatedAt)
	if err != nil {
		return fmt.Errorf("falha ao salvar peer: %w", err)
	}

	return nil
}

func (r *PeerRepository) FindByID(ctx context.Context, id string) (*domain.Peer, error) {
	query := `SELECT id, cluster_id, name, public_key, allocated_ip, is_revoked, status, last_seen, created_at FROM peers WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)

	var peer domain.Peer
	var ipStr string
	var isRevokedInt int
	var lastSeen sql.NullString
	var createdAt string

	err := row.Scan(&peer.ID, &peer.ClusterID, &peer.Name, &peer.PublicKey, &ipStr, &isRevokedInt, &peer.Status, &lastSeen, &createdAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("peer não encontrado")
		}
		return nil, fmt.Errorf("falha ao buscar peer: %w", err)
	}

	// Hidratando a entidade com os tipos corretos
	peer.AllocatedIP = net.ParseIP(ipStr)
	peer.IsRevoked = isRevokedInt == 1
	peer.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if lastSeen.Valid {
		peer.LastSeen, _ = time.Parse(time.RFC3339, lastSeen.String)
	}

	return &peer, nil
}

// GetUsedIPs atende à necessidade crítica do domínio para o IPAM

func (r *PeerRepository) GetUsedIPs(ctx context.Context, clusterID string) ([]net.IP, error) {
	query := `SELECT allocated_ip FROM peers WHERE cluster_id = ? AND allocated_ip != ''`
	rows, err := r.db.QueryContext(ctx, query, clusterID)
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar IPs: %w", err)
	}
	defer rows.Close()

	var ips []net.IP
	for rows.Next() {
		var ipStr string
		if err := rows.Scan(&ipStr); err != nil {
			return nil, err
		}
		if parsedIP := net.ParseIP(ipStr); parsedIP != nil {
			ips = append(ips, parsedIP)
		}
	}
	return ips, nil
}

func (r *PeerRepository) CountByCluster(ctx context.Context, clusterID string) (int, error) {
	query := `SELECT COUNT(*) FROM peers WHERE cluster_id = ?`
	row := r.db.QueryRowContext(ctx, query, clusterID)

	var count int
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("falha ao contar peers: %w", err)
	}
	return count, nil
}

func (r *PeerRepository) UpdateHealthStatus(ctx context.Context, peerID, status string, lastSeen time.Time) error {
	query := `UPDATE peers SET status = ?, last_seen = ? WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, status, lastSeen, peerID)
	if err != nil {
		return fmt.Errorf("falha ao atualizar status de saúde do peer: %w", err)
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("falha ao verificar atualização do status de saúde do peer: %w", err)
	}

	if affectedRows == 0 {
		return fmt.Errorf("peer não encontrado")
	}

	return nil
}

func (r *PeerRepository) GetAll(ctx context.Context) ([]*domain.Peer, error) {
	query := `SELECT id, cluster_id, name, public_key, allocated_ip, is_revoked, status, last_seen, created_at FROM peers`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar peers: %w", err)
	}
	defer rows.Close()

	var peers []*domain.Peer
	for rows.Next() {
		var peer domain.Peer
		var ipStr string
		var isRevokedInt int
		var lastSeen sql.NullString
		var createdAt string
		if err := rows.Scan(&peer.ID, &peer.ClusterID, &peer.Name, &peer.PublicKey, &ipStr, &isRevokedInt, &peer.Status, &lastSeen, &createdAt); err != nil {
			return nil, fmt.Errorf("falha ao escanear peer: %w", err)
		}
		peer.AllocatedIP = net.ParseIP(ipStr)
		peer.IsRevoked = isRevokedInt == 1
		peer.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if lastSeen.Valid {
			peer.LastSeen, _ = time.Parse(time.RFC3339, lastSeen.String)
		}
		peers = append(peers, &peer)
	}
	return peers, nil
}
