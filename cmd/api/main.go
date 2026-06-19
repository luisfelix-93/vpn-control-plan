package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/luisfelix-93/vpn-control-plane/internal/infra/metrics"
	"github.com/luisfelix-93/vpn-control-plane/internal/infra/sqlite"
	"github.com/luisfelix-93/vpn-control-plane/internal/infra/wireguard"
	presentation "github.com/luisfelix-93/vpn-control-plane/internal/presentation/http"
	"github.com/luisfelix-93/vpn-control-plane/internal/usecase"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	dbPath := "./vpn.db"

	// 1. Inicializa o Banco de Dados
	db, err := sql.Open("sqlite3", "file:"+dbPath+"?_foreign_keys=on")
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

	// 3. Adaptador de Rede (Agora Stateless)
	vpnAdapter := wireguard.NewCLIAdapter()

	// 4. Orquestração (Injetando as novas dependências)
	peerUseCase := usecase.NewPeerUseCase(peerRepo, clusterRepo, vpnAdapter)
	clusterUseCase := usecase.NewClusterUseCase(clusterRepo)
	// --- SETUP DAS MÉTRICAS EM BACKGROUND ---
	// Iniciamos a rotina que atualiza o estado interno do Prometheus a cada 15 segundos
	metricsCollector := metrics.NewCollectorService(clusterRepo, peerRepo)
	go metricsCollector.Start(context.Background(), 15*time.Second)
	// ----------------------------------------
	// 5. API
	peerHandler := presentation.NewPeerHandler(peerUseCase)
	clusterHandler := presentation.NewClusterHandler(clusterUseCase)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /clusters", clusterHandler.Create)
	mux.HandleFunc("POST /peers", peerHandler.Register)
	mux.Handle("GET /metrics", promhttp.Handler())

	port := ":8080"
	log.Printf("VPN Control Plane (Multi-Cluster) rodando na porta %s...", port)
	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("Servidor caiu: %v", err)
	}
}
