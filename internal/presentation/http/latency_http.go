package http

import (
	"encoding/json"
	"net/http"

	"github.com/luisfelix-93/vpn-control-plane/internal/usecase"
)

type LatencyHandler struct {
	usecase *usecase.ClusterUseCase
}

func NewLatencyHandler(usecase *usecase.ClusterUseCase) *LatencyHandler {
	return &LatencyHandler{
		usecase: usecase,
	}
}

func (h *LatencyHandler) Report(w http.ResponseWriter, r *http.Request) {
	sourceClusterID := r.PathValue("id")
	if sourceClusterID == "" {
		http.Error(w, "ID do cluster de origem não informado", http.StatusBadRequest)
		return
	}

	var reports []usecase.ReportLatencyPayload
	if err := json.NewDecoder(r.Body).Decode(&reports); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	if err := h.usecase.ProcessLatencyReport(r.Context(), sourceClusterID, reports); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Relatório de latência processado com sucesso"))
}