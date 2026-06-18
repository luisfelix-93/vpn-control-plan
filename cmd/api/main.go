package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	"github.com/luisfelix-93/vpn-control-plane/internal/domain"
	"github.com/luisfelix-93/vpn-control-plane/internal/infra/sqlite"
	"github.com/luisfelix-93/vpn-control-plane/internal/infra/wireguard"
	presentation "github.com/luisfelix-93/vpn-control-plane/internal/presentation/http"
	"github.com/luisfelix-93/vpn-control-plane/internal/usecase"
)

func main() {
	dbPath := "./vpn.db"

	// 1. Inicializa o Banco de Dados
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Falha ao abrir o banco: %v", err)
	}
	defer db.Close()

	// 2. Repositórios e Schemas
	clusterRepo := sqlite.NewClusterRepository(db)
	if err := clusterRepo.InitSchema(context.Background()); err != nil {
		log.Fatalf("Falha no schema de clusters: %v", err)
	}

	peerRepo := sqlite.NewPeerRepository(db)
	if err := peerRepo.InitSchema(context.Background()); err != nil {
		log.Fatalf("Falha no schema de peers: %v", err)
	}

	// Função utilitária para garantir que temos Redes para testar
	seedClusters(clusterRepo)

	// 3. Adaptador de Rede (Agora Stateless)
	vpnAdapter := wireguard.NewCLIAdapter()

	// 4. Orquestração (Injetando as novas dependências)
	peerUseCase := usecase.NewPeerUseCase(peerRepo, clusterRepo, vpnAdapter)

	// 5. API
	peerHandler := presentation.NewPeerHandler(peerUseCase)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /peers", peerHandler.Register)

	port := ":8080"
	log.Printf("VPN Control Plane (Multi-Cluster) rodando na porta %s...", port)
	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("Servidor caiu: %v", err)
	}
}

// seedClusters insere dados iniciais para você não precisar inserir na mão agora
func seedClusters(repo domain.ClusterRepository) {
	ctx := context.Background()
	
	// Cluster 1: O seu Homelab Local
	c1, _ := domain.NewCluster("cluster-homelab", "Homelab Local", "10.8.0.0/24", "wg0", "PUB_KEY_LAB", "192.168.1.50:51820")
	_ = repo.Save(ctx, c1)

	// Cluster 2: A sua VPS na nuvem atuando como Exit Node (Torrents/Privacidade)
	c2, _ := domain.NewCluster("cluster-cloud", "Exit Node Cloud", "10.9.0.0/24", "wg-cloud", "PUB_KEY_CLOUD", "189.20.30.40:51820")
	_ = repo.Save(ctx, c2)
}