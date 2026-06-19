package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/luisfelix-93/vpn-control-plane/internal/domain"
)

var _ domain.ClusterRepository = (*ClusterRepositoryImpl) (nil)

type ClusterRepositoryImpl struct {
	db *sql.DB
}

func NewClusterRepository(db *sql.DB) *ClusterRepositoryImpl {
	return &ClusterRepositoryImpl{
		db: db,
	}
}

//InitClusterSchema cria a tabela principal de zonas de rede
func (r *ClusterRepositoryImpl) InitSchema (ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS clusters (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		cidr TEXT NOT NULL,
		interface_name TEXT NOT NULL,
		server_pub_key TEXT NOT NULL,
		server_endpoint TEXT NOT NULL,
		created_at DATETIME NOT NULL
	);
	`
	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("falha ao inicializar schema de clusters: %w", err)
	}
	return nil
}

// Save permite cadastrar ou atualizar um cluster
func (r * ClusterRepositoryImpl) Save(ctx context.Context, cluster *domain.Cluster) error {
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
	cluster.ServerPubKey, cluster.ServerEndpoint, cluster.CreatedAt)
	if err != nil {
		return fmt.Errorf("falha ao salvar cluster: %w", err)
	}
	return nil
}
// FindByID busca a configuração da rede para o UseCase utilizar
func (r *ClusterRepositoryImpl) FindByID(ctx context.Context, id string) (*domain.Cluster, error) {
	query := `
	SELECT id, name, cidr, interface_name, server_pub_key, server_endpoint, created_at 
	FROM clusters WHERE id = ?`
	
	row := r.db.QueryRowContext(ctx, query, id)

	var c domain.Cluster
	var createdAt string

	err := row.Scan(&c.ID, &c.Name, &c.CIDR, &c.InterfaceName, &c.ServerPubKey, &c.ServerEndpoint, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("cluster não encontrado")
		}
		return nil, fmt.Errorf("falha ao buscar cluster: %w", err)
	}

	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &c, nil
}

func (r *ClusterRepositoryImpl) GetAll(ctx context.Context) ([]*domain.Cluster, error) {
	query := `
	SELECT id, name, cidr, interface_name, server_pub_key, server_endpoint, created_at 
	FROM clusters`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar clusters: %w", err)
	}
	defer rows.Close()

	var clusters []*domain.Cluster
	for rows.Next() {
		var c domain.Cluster
		var createdAt string
		if err := rows.Scan(&c.ID, &c.Name, &c.CIDR, &c.InterfaceName, &c.ServerPubKey, &c.ServerEndpoint, &createdAt); err != nil {
			return nil, fmt.Errorf("falha ao escanear cluster: %w", err)
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		clusters = append(clusters, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erro ao iterar sobre os clusters: %w", err)
	}
	return clusters, nil
}