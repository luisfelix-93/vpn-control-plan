package http

import (
	"encoding/json"
	"net/http"

	"github.com/luisfelix-93/vpn-control-plane/internal/usecase"
)

type PeerHandler struct {
	useCase *usecase.PeerUseCase
}

func NewPeerHandler(uc *usecase.PeerUseCase) *PeerHandler {
	return &PeerHandler{
		useCase: uc,
	}
}

// RegisterRequest agora exige saber em qual Cluster o dispositivo será criado
type RegisterRequest struct {
	ClusterID string `json:"clusterId"`
	Name      string `json:"name"`
}

func (h *PeerHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.ClusterID == "" {
		http.Error(w, "Os campos 'name' e 'clusterId' são obrigatórios", http.StatusBadRequest)
		return
	}

	// Chama a orquestração passando o ID da zona de rede
	clientConfig, err := h.useCase.RegisterNewPeer(r.Context(), req.ClusterID, req.Name)
	if err != nil {
		http.Error(w, "Erro ao registrar cliente", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(clientConfig))
}