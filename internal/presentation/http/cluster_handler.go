package http

import (
	"encoding/json"
	"net/http"

	"github.com/luisfelix-93/vpn-control-plane/internal/infra/metrics"
	"github.com/luisfelix-93/vpn-control-plane/internal/usecase"
)

type ClusterHandler struct {
	useCase *usecase.ClusterUseCase
}

func NewClusterHandler(useCase *usecase.ClusterUseCase) *ClusterHandler {
	return &ClusterHandler{
		useCase: useCase,
	}
}

type CreateClusterRequest struct {
	Name           string `json:"name" validate:"required"`
	CIDR           string `json:"cidr" validate:"required"`
	InterfaceName  string `json:"interface_name" validate:"required"`
	ServerPubKey   string `json:"server_pub_key" validate:"required"`
	ServerEndpoint string `json:"server_endpoint" validate:"required"`
}

func (h *ClusterHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateClusterRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	cluster, err := h.useCase.CreateCluster(r.Context(), req.Name, req.CIDR, req.InterfaceName, req.ServerPubKey, req.ServerEndpoint)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Retorna os dados do cluster criado (incluindo o ID gerado)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(cluster)
}

func (h *ClusterHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	clusterID := r.PathValue("id")
	if clusterID == "" {
		metrics.IncClusterHeartbeatResult("unknown", "error")
		http.Error(w, "ID do cluster não informado", http.StatusBadRequest)
		return
	}

	if err := h.useCase.ProcessHeartbeat(r.Context(), clusterID); err != nil {
		metrics.IncClusterHeartbeatResult(clusterID, "error")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	metrics.IncClusterHeartbeatResult(clusterID, "success")

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ACK"))
}
