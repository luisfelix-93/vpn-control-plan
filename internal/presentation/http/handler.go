package http

import (
	"encoding/json"
	"net/http"

	"github.com/luisfelix-93/vpn-control-plane/internal/usecase"
)

type PeerHandler struct {
	useCase *usecase.PeerUseCase
}

func NewPeerHandler(useCase *usecase.PeerUseCase) *PeerHandler {
	return &PeerHandler{
		useCase: useCase,
	}
}

type RegisterRequest struct {
	Name string `json:"name"`
}

func (h *PeerHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Nome é obrigatório", http.StatusBadRequest)
		return 
	}

	clientConfig, err := h.useCase.RegisterNewPeer(r.Context(), req.Name)
	if err != nil {
		http.Error(w, "Erro ao registrar cliente", http.StatusInternalServerError)

		return 
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(clientConfig))
}