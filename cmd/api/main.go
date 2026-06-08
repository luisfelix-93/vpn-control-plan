package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/luisfelix-93/vpn-control-plane/internal/domain"
	"github.com/luisfelix-93/vpn-control-plane/internal/infra/sqlite"
	"github.com/luisfelix-93/vpn-control-plane/internal/infra/wireguard"
	presentation "github.com/luisfelix-93/vpn-control-plane/internal/presentation/http"
	"github.com/luisfelix-93/vpn-control-plane/internal/usecase"
)

func main() {
	// 1. Configurações base (Idealmente viriam de variáveis de ambiente .env)
	dbPath := "./vpn.db"
	vpnInterface := "wg0"
	serverPubKey := os.Getenv("SERVER_PUB_KEY") // Chave pública do seu servidor
	serverEndpoint := "vpn.meudominio.com:51820" // Seu DDNS ou IP Público
	vpnNetwork := &domain.Network{CIDR: "10.8.0.0/24"}

	if serverPubKey == "" {
		log.Println("Aviso: SERVER_PUB_KEY não definida. O arquivo de configuração gerado estará incompleto.")
	}

	// 2. Inicializa o Banco de Dados (Infra)
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Falha ao abrir o banco: %v", err)
	}
	defer db.Close()

	repo := sqlite.NewPeerRepository(db)
	if err := repo.InitSchema(context.Background()); err != nil {
		log.Fatalf("Falha ao criar tabelas: %v", err)
	}

	// 3. Inicializa o Adaptador de Rede (Infra)
	vpnAdapter := wireguard.NewCLIAdapter(vpnInterface)

	// 4. Inicializa os Casos de Uso (Application Layer)
	peerUseCase := usecase.NewPeerUseCase(repo, vpnAdapter, vpnNetwork, serverPubKey, serverEndpoint)

	// 5. Inicializa os Controladores HTTP (Presentation Layer)
	peerHandler := presentation.NewPeerHandler(peerUseCase)

	// 6. Configura as Rotas (Usando o roteador nativo do Go 1.22+)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /peers", peerHandler.Register)

	// 7. Sobe o Servidor
	port := ":8080"
	log.Printf("Control Plane da VPN rodando na porta %s...", port)
	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("Servidor caiu: %v", err)
	}
}